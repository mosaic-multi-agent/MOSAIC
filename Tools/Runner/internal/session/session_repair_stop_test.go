package session_test

// A repair consultation that fails, or is stopped, ends the run at once: no
// retry, no manual fallback, no write to the damaged artifact, every copy kept.
// A stop that ends the consultation whose read found the damage is also final.

import (
	"errors"
	"os"
	"testing"

	"mosaic-run/internal/domain"
)

// requireEvidenceKept asserts the damaged and last-good copies named by the
// unrepaired error exist on disk.
func requireEvidenceKept(t *testing.T, u *domain.UnrepairedDamageError) {
	t.Helper()
	for name, path := range map[string]string{"damaged copy": u.DamagedCopy, "last-good copy": u.LastGoodCopy} {
		if path == "" {
			t.Errorf("want the %s named in the unrepaired error", name)
			continue
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("want the %s kept on disk: %v", name, err)
		}
	}
}

func TestSession_FailedRepairConsultation_StopsAtOnceWithoutRetryOrManualFallback(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{err: transportFailure()},
		{edit: fixLog, reply: dispatchReply("agent-b", 1)},
	}, func(r *repairRig) { r.cfg.ManualResolution = true })

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
	if rig.cons.calls() != 3 {
		t.Errorf("want no retry of the repair consultation, got %d consultations", rig.cons.calls())
	}
	if rig.manual.CallCount != 0 {
		t.Errorf("want no manual-resolution fallback for a failed repair consultation, got %d manual consultations", rig.manual.CallCount)
	}
	if readText(t, rig.run.artifactPath) != rig.cons.leftAfter(1) {
		t.Error("want the artifact untouched: still as the damaging consultation left it")
	}
	requireEvidenceKept(t, unrepaired)
}

func TestSession_DamagingConsultationFailing_WithManualResolutionOn_IsRepairedNotHandedToTheManualResolver(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, err: transportFailure()},
		{edit: fixLog, reply: stopReply("repaired")},
	}, func(r *repairRig) { r.cfg.ManualResolution = true })

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if got.StopReason != "repaired" {
		t.Errorf("want the repair consultation's stop, got %q", got.StopReason)
	}
	if rig.manual.CallCount != 0 {
		t.Errorf("want the manual resolver left out of a damaged-artifact failure, got %d manual consultations", rig.manual.CallCount)
	}
	requireDamageReport(t, rig.cons.request(2))
}

func TestSession_StopConfirmedDuringTheRepairConsultation_StopsAtOnceAndLeavesTheArtifact(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, arm: true, reply: dispatchReply("agent-b", 1)},
		{reply: stopReply("must not be reached")},
	}, nil)

	got := rig.start(t)

	if got.Status != domain.RunStopped {
		t.Fatalf("want RunStopped, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedStopped || unrepaired.Consultations != 1 {
		t.Errorf("want reason stopped after 1 consultation, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	if n := countInvocationsFor(rig.f.Invocations(), "agent-b"); n != 0 {
		t.Errorf("want no dispatch after the stop, got %d agent-b invocations", n)
	}
	if readText(t, rig.run.artifactPath) != rig.cons.leftAfter(2) {
		t.Error("want the artifact as the orchestrator left it, with no write by the Runner")
	}
	requireEvidenceKept(t, unrepaired)
}

func TestSession_ForceStopDuringTheRepairConsultation_StopsAtOnceWithoutAskingTheUser(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, reply: dispatchReply("agent-b", 1)},
		{force: true},
		{reply: stopReply("must not be reached")},
	}, nil)

	got := rig.start(t)

	if got.Status != domain.RunStopped {
		t.Fatalf("want RunStopped, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedStopped {
		t.Errorf("want reason stopped, got %q", unrepaired.Reason)
	}
	if rig.cons.calls() != 3 {
		t.Errorf("want no consultation after the force stop, got %d", rig.cons.calls())
	}
	if n := len(rig.user.asked); n != 0 {
		t.Errorf("want no what-next question after a force-stopped repair, got %d questions", n)
	}
	if readText(t, rig.run.artifactPath) != rig.cons.leftAfter(1) {
		t.Error("want the artifact exactly as the damaging consultation left it")
	}
	requireEvidenceKept(t, unrepaired)
}

func TestSession_StopConfirmedDuringTheDamagingConsultation_StopsWithoutARepairConsultation(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, arm: true, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("must not be reached")},
	}, nil)

	got := rig.start(t)

	if got.Status != domain.RunStopped {
		t.Fatalf("want RunStopped, got %q (%s)", got.Status, got.Message)
	}
	unrepaired := unrepairedOf(t, got)
	if unrepaired.Reason != domain.UnrepairedStopped || unrepaired.Consultations != 0 {
		t.Errorf("want reason stopped after 0 consultations, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	if rig.cons.calls() != 2 {
		t.Errorf("want no repair consultation after a stop, got %d consultations", rig.cons.calls())
	}
	if unrepaired.DamagedCopy == "" {
		t.Error("want the damaged copy saved before the stop is decided")
	}
}

func TestSession_ForceStopDuringTheDamagingConsultation_StopsWithoutARepairConsultation(t *testing.T) {
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, force: true},
		{edit: fixLog, reply: stopReply("must not be reached")},
	}, nil)

	got := rig.start(t)

	if got.Status != domain.RunStopped {
		t.Fatalf("want RunStopped, got %q (%s)", got.Status, got.Message)
	}
	if unrepaired := unrepairedOf(t, got); unrepaired.Reason != domain.UnrepairedStopped || unrepaired.Consultations != 0 {
		t.Errorf("want reason stopped after 0 consultations, got %q after %d", unrepaired.Reason, unrepaired.Consultations)
	}
	if rig.cons.calls() != 2 {
		t.Errorf("want no repair consultation after the force stop, got %d consultations", rig.cons.calls())
	}
	if n := len(rig.user.asked); n != 0 {
		t.Errorf("want no what-next question while the artifact is damaged, got %d questions", n)
	}
}

func TestSession_PendingManualDispatchRequest_DoesNotRedirectTheRepairConsultation(t *testing.T) {
	flag := &requestFlag{}
	rig := newRepairRig(t, []repairStep{
		firstStep(),
		{edit: damageLog, during: flag.request, reply: dispatchReply("agent-b", 1)},
		{edit: fixLog, reply: stopReply("repaired")},
	}, func(r *repairRig) { r.deps.TakeManualDispatchRequest = flag.take })

	got := rig.start(t)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	requireDamageReport(t, rig.cons.request(2))
	if rig.manual.CallCount != 0 {
		t.Errorf("want the repair consultation to go to the orchestrator, the manual resolver was asked %d times", rig.manual.CallCount)
	}
	if !flag.isPending() {
		t.Error("want the pending manual-dispatch request not consumed by the repair consultation")
	}
}
