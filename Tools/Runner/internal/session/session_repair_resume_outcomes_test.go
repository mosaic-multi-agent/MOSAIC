package session_test

// How a repair consultation made at run start ends: accepted, rejected twice,
// failed, a consultant stop, no consultant. The damaged artifact is never
// written by the Runner before an accepted repair.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// restoreIncident is a repair of the damaged incident whatever the artifact
// holds now: it starts again from the damaged text.
func restoreIncident(t *testing.T, _ string) string {
	t.Helper()
	return repairIncident(t, incidentDamagedText(t))
}

func TestSession_ResumeWithDamageAndNoLastGoodCopy_StillConsultsAndNamesNoLastGoodPath(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("done")},
	}, withDamagedRun(t, startOpts{noLastGood: true}))

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	report := requireDamageReport(t, rig.cons.request(0))
	if report.LastGoodCopy != "" {
		t.Errorf("want no last-good path in the report, got %q", report.LastGoodCopy)
	}
	if want := []string{"agent-a#84"}; !slices.Equal(invokedInstances(rig.f), want) {
		t.Errorf("want the repair accepted without a last-good copy and its dispatch carried out, got %v", invokedInstances(rig.f))
	}
}

func TestSession_ResumeWithDamageAndNoDispatchLog_NamesNoDispatchLogPath(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: repairIncident, reply: stopReply("done")},
	}, withDamagedRun(t, startOpts{noDispatchLog: true}))

	rig.start(t)

	if report := requireDamageReport(t, rig.cons.request(0)); report.DispatchLog != "" {
		t.Errorf("want no dispatch-log path when the run has no log, got %q", report.DispatchLog)
	}
}

func TestSession_StartRepairRejectedOnce_IsAskedAgainWithAFreshReportAndTheSecondReplyIsCarriedOut(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: dropRow77, reply: dispatchReply("agent-b", 1)},
		{edit: restoreIncident, reply: dispatchReply("agent-a", 0)},
		{reply: stopReply("done")},
	}, withDamagedRun(t, startOpts{}))

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	first, second := requireDamageReport(t, rig.cons.request(0)), requireDamageReport(t, rig.cons.request(1))
	if slices.Equal(problemStrings(first.Problems), problemStrings(second.Problems)) {
		t.Errorf("want a fresh report naming what the rejected repair left wrong, got the same problems %v", problemStrings(first.Problems))
	}
	requireStartTriggersNull(t, rig.cons.request(1))
	if want := []string{"agent-a#84"}; !slices.Equal(invokedInstances(rig.f), want) {
		t.Errorf("want only the accepted repair's dispatch carried out, got %v", invokedInstances(rig.f))
	}
}

func TestSession_StartRepairRejectedTwice_StopsWithTheReportAndPathsAndNoRunnerWrite(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: dropRow77, reply: dispatchReply("agent-b", 1)},
		{reply: dispatchReply("agent-b", 1)},
		{reply: stopReply("must not be reached")},
	}, withDamagedRun(t, startOpts{}))

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedRejected || unrepaired.Consultations != 2 {
		t.Errorf("want reason rejected after 2 consultations, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	if len(unrepaired.Problems) == 0 {
		t.Error("want the problems of the latest verdict in the error")
	}
	if rig.cons.calls() != 2 {
		t.Errorf("want at most 2 repair consultations, got %d", rig.cons.calls())
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch from a rejected repair, got %v", invokedInstances(rig.f))
	}
	requireEvidenceKept(t, unrepaired)
	for _, path := range []string{unrepaired.DamagedCopy, unrepaired.LastGoodCopy, rig.run.artifactPath} {
		if !strings.Contains(got.Message, path) {
			t.Errorf("want the outcome message to name %q, got %q", path, got.Message)
		}
	}
	if readText(t, rig.run.artifactPath) != rig.cons.leftAfter(1) {
		t.Error("want the artifact exactly as the orchestrator last left it")
	}
}

func TestSession_FailedStartRepairConsultation_StopsAtOnceWithoutRetryOrManualFallback(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{err: transportFailure()},
		{edit: repairIncident, reply: dispatchReply("agent-a", 0)},
	}, func(r *repairRig) {
		withDamagedRun(t, startOpts{})(r)
		r.cfg.ManualResolution = true
	})

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedConsultationFailed || unrepaired.Consultations != 1 {
		t.Errorf("want reason consultation-failed after 1 consultation, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	var cause *domain.ConsultationError
	if !errors.As(got.Cause, &cause) {
		t.Errorf("want the consultation error kept as the cause, got %v", got.Cause)
	}
	if rig.cons.calls() != 1 || rig.manual.CallCount != 0 {
		t.Errorf("want no retry and no manual fallback, got %d consultations and %d manual", rig.cons.calls(), rig.manual.CallCount)
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch, got %v", invokedInstances(rig.f))
	}
	requireNoWriteOnDamaged(t, rig)
	requireEvidenceKept(t, unrepaired)
}

func TestSession_AcceptedStartRepairReturningStop_IsAConsultantStop(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: repairIncident, reply: stopReply("workflow is done")},
	}, withDamagedRun(t, startOpts{}))

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if got.StopReason != "workflow is done" {
		t.Errorf("want the repair reply's stop reason, got %q", got.StopReason)
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch after the stop, got %v", invokedInstances(rig.f))
	}
}

func TestSession_StartRepairWithoutARoutingConsultant_FailsWithoutAConsultation(t *testing.T) {
	rig := newRepairRig(t, nil, func(r *repairRig) {
		withDamagedRun(t, startOpts{})(r)
		r.deps.Routing = nil
	})

	got := rig.start(t)

	if got.Status != domain.RunFailed {
		t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedNoConsultant || unrepaired.Consultations != 0 {
		t.Errorf("want reason no-consultant after 0 consultations, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch, got %v", invokedInstances(rig.f))
	}
	requireNoWriteOnDamaged(t, rig)
}

func TestSession_AcceptedStartRepair_IsReportedToTheObserverWithItsAttempt(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		{edit: dropRow77, reply: dispatchReply("agent-b", 1)},
		{edit: restoreIncident, reply: stopReply("done")},
	}, withDamagedRun(t, startOpts{}))

	rig.start(t)

	var attempts []int
	for _, a := range rig.trace.started() {
		attempts = append(attempts, a.RepairAttempt)
	}
	if want := []int{1, 2}; !slices.Equal(attempts, want) {
		t.Errorf("want repair attempts %v announced, got %v", want, attempts)
	}
	judged := rig.trace.judged()
	if len(judged) != 2 || judged[0].Accepted || !judged[1].Accepted || judged[0].Attempt != 1 || judged[1].Attempt != 2 {
		t.Errorf("want attempt 1 not accepted and attempt 2 accepted, got %+v", judged)
	}
}
