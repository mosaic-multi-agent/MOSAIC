package session_test

// Start repair edge paths: the start checks that run on the repaired artifact,
// a repair consultation that is stopped, and a dispatch log that cannot be used.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// repairIncidentWithRunID repairs the incident but also records another,
// well-formed run identity, which no longer matches the run folder.
func repairIncidentWithRunID(otherRunID string) docEdit {
	return func(t *testing.T, doc string) string {
		t.Helper()
		return replaceOnce(t, repairIncident(t, doc), "run_id: "+testRunID, "run_id: "+otherRunID)
	}
}

func TestSession_AcceptedStartRepairWhoseRunIdentityNoLongerMatchesTheFolder_IsRefusedAsAnyResumeIs(t *testing.T) {
	const otherRunID = "20260101T000000Z-abcd"
	rig := newRepairRig(t, []repairStep{
		{edit: repairIncidentWithRunID(otherRunID), reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("must not be reached")},
	}, withDamagedRun(t, startOpts{}))

	got := rig.start(t)

	requireRefused(t, got, nil)
	var idErr *domain.RunIdentityError
	if !errors.As(got.Cause, &idErr) || idErr.Problem != domain.RunIdentityFolderMismatch {
		t.Fatalf("want the run identity refusal (folder mismatch) as the cause, got %T (%v)", got.Cause, got.Cause)
	}
	if rig.cons.calls() != 1 {
		t.Errorf("want the one repair consultation and nothing after the refusal, got %d", rig.cons.calls())
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want the returned dispatch not carried out on a refused run, got %v", invokedInstances(rig.f))
	}
}

func TestSession_AcceptedStartRepairThatChangesWorkflowOrMode_IsRefusedNamingBothValues(t *testing.T) {
	tests := []struct {
		name      string
		old, repl string
		wantBoth  [2]string
	}{
		{"workflow", "workflow: linear\n", "workflow: other-flow\n", [2]string{"linear", "other-flow"}},
		{"mode", "runner_mode: orchestrated", "runner_mode: auto", [2]string{"orchestrated", "auto"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			change := func(t *testing.T, doc string) string {
				return replaceOnce(t, repairIncident(t, doc), tc.old, tc.repl)
			}
			rig := newRepairRig(t, []repairStep{
				{edit: change, reply: dispatchReply("agent-a", 0)},
				{reply: stopReply("must not be reached")},
			}, withDamagedRun(t, startOpts{}))

			got := rig.start(t)

			requireRefused(t, got, nil)
			for _, v := range tc.wantBoth {
				if !strings.Contains(got.Message, v) {
					t.Errorf("want the refusal to name %q, got %q", v, got.Message)
				}
			}
			if rig.cons.calls() != 1 || len(rig.f.Invocations()) != 0 {
				t.Errorf("want one consultation and no dispatch, got %d and %v", rig.cons.calls(), invokedInstances(rig.f))
			}
			if readText(t, rig.run.artifactPath) != rig.cons.leftAfter(0) {
				t.Error("want the repaired artifact left as the orchestrator wrote it")
			}
		})
	}
}

func TestSession_StoppedStartRepairConsultation_EndsTheRunWithoutRetryOrManualFallback(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{force: true},
		{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
	}, func(r *repairRig) {
		withDamagedRun(t, startOpts{})(r)
		r.cfg.ManualResolution = true
	})

	got := rig.start(t)

	if got.Status == domain.RunCompleted || got.Status == domain.RunStoppedByConsultant {
		t.Fatalf("want the stopped repair to end the run, got %q (%s)", got.Status, got.Message)
	}
	if unrepaired := unrepairedOf(t, got); unrepaired.Reason != domain.UnrepairedStopped {
		t.Errorf("want reason stopped, got %q", unrepaired.Reason)
	}
	if rig.cons.calls() != 1 || rig.manual.CallCount != 0 {
		t.Errorf("want no retry and no manual fallback, got %d consultations and %d manual", rig.cons.calls(), rig.manual.CallCount)
	}
	if n := len(rig.user.asked); n != 0 {
		t.Errorf("want no what-next question while the artifact is damaged, got %d questions", n)
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch, got %v", invokedInstances(rig.f))
	}
	requireNoWriteOnDamaged(t, rig)
}

func TestSession_ResumeWithDamageAndAnUnusableDispatchLog_NamesNoDispatchLogPath(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: repairIncident, reply: stopReply("done")},
	}, func(r *repairRig) {
		withDamagedRun(t, startOpts{noDispatchLog: true})(r)
		writeFile(t, dispatchLogPathOf(r), "this is not a dispatch log\n{broken\n")
	})

	rig.start(t)

	report := requireDamageReport(t, rig.cons.request(0))
	if report.DispatchLog != "" {
		t.Errorf("want no dispatch-log path when the log cannot be used, got %q", report.DispatchLog)
	}
	if strings.TrimSpace(report.LastGoodCopy) == "" {
		t.Error("want the last-good copy still named, it is usable")
	}
}
