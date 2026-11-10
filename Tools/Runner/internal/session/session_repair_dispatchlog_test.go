package session_test

// The dispatch-log path reaches the repair consultation and the verification
// only when the dispatch log's reader reports it as this run's own log. An
// empty path, a startup fallback file or another run's log gives neither a path,
// and the repair is then verified on its other checks.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
)

const (
	logVersionLine = `{"type":"version","tool_version":"1.12.0"}`
	// A step response far above every Execution Log row: judged against it, any
	// repair is rejected.
	contradictingResponse = `{"type":"response","agent_instance_id":"agent-a#9","response":{"agent_instance_id":"agent-a#9","status_code":"SUCCESS","status_message":"m"}}`
	// The response of the first recorded step: consistent with the artifact.
	consistentResponse = `{"type":"response","agent_instance_id":"agent-a#1","response":{"agent_instance_id":"agent-a#1","status_code":"SUCCESS","status_message":"m"}}`
)

func writeLogFile(t *testing.T, path string, lines ...string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	return path
}

// repairSteps is the damaging consultation followed by two repairs that fix
// the log and dispatch agent-b.
func repairSteps(final repairStep) []repairStep {
	return []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: dispatchReply("agent-b", 1)},
		final,
	}
}

func TestSession_OwnRunDispatchLog_ReachesTheRepairConsultationAndIsAcceptedWhenConsistent(t *testing.T) {
	rig := newRepairRig(t, repairSteps(repairStep{reply: stopReply("finished")}), func(r *repairRig) {
		own := dispatchlog.RunLogPath(filepath.Dir(r.run.runFolder), testRunID)
		writeLogFile(t, own, logVersionLine, consistentResponse)
		r.deps.DispatchLog = pathLogger{path: own}
	})

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	want := dispatchlog.RunLogPath(filepath.Dir(rig.run.runFolder), testRunID)
	if report := requireDamageReport(t, rig.cons.request(2)); report.DispatchLog != want {
		t.Errorf("want the run's own dispatch log %q in the report, got %q", want, report.DispatchLog)
	}
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want the repair accepted against a consistent log (agent-b dispatched once), got %d", n)
	}
}

func TestSession_OwnRunDispatchLog_IsUsedToVerifyTheRepair(t *testing.T) {
	rig := newRepairRig(t, repairSteps(repairStep{reply: dispatchReply("agent-b", 1)}), func(r *repairRig) {
		own := dispatchlog.RunLogPath(filepath.Dir(r.run.runFolder), testRunID)
		writeLogFile(t, own, logVersionLine, consistentResponse, contradictingResponse)
		r.deps.DispatchLog = pathLogger{path: own}
	})

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed: the log's response above every row rejects both repairs, got %q (%s)", got.Status, got.Message)
	}
	if unrepaired := unrepairedOf(t, got); unrepaired.Reason != domain.UnrepairedRejected {
		t.Errorf("want reason rejected, got %q", unrepaired.Reason)
	}
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 0 {
		t.Errorf("want no dispatch from a rejected repair, got %d", n)
	}
}

func TestSession_DispatchLogThatIsNotTheRunsOwn_GivesNeitherTheReportNorTheVerifierAPath(t *testing.T) {
	cases := map[string]func(workspace string) string{
		"no log file": func(string) string { return "" },
		"startup fallback file": func(ws string) string {
			return writeLogFile(t, filepath.Join(ws, "RunnerLogs", "startup-20260928T103906Z-dispatch.log"),
				logVersionLine, contradictingResponse)
		},
		"another run's log": func(ws string) string {
			return writeLogFile(t, filepath.Join(ws, "RunnerLogs", "20260101T000000Z-aaaa", "20260101T000000Z-aaaa-dispatch.log"),
				logVersionLine, contradictingResponse)
		},
	}
	for name, makePath := range cases {
		t.Run(name, func(t *testing.T) {
			rig := newRepairRig(t, repairSteps(repairStep{reply: stopReply("finished")}), func(r *repairRig) {
				r.deps.DispatchLog = pathLogger{path: makePath(filepath.Dir(r.run.runFolder))}
			})

			got := rig.start(t)

			requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
			if report := requireDamageReport(t, rig.cons.request(2)); report.DispatchLog != "" {
				t.Errorf("want no dispatch-log path in the report, got %q", report.DispatchLog)
			}
			if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 1 {
				t.Errorf("want the repair verified on its other checks and accepted (agent-b dispatched once), got %d", n)
			}
		})
	}
}
