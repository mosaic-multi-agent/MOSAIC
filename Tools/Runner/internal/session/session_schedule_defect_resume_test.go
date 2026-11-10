package session_test

// The re-dispatch count is derived from the Execution Log alone, so it
// survives a resume: a run that stopped right after a producer's defective
// SUCCESS resumes by re-dispatching it while re-dispatches remain, and
// consults once they are used up. Infrastructure entries do not reset the
// count, a block row's entry does, and a PARTIALLY_DONE entry is not an
// attempt.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// startResumed resumes the audit-plan-block run, with its Source holding a
// defective schedule, from the given log.
func startResumed(t *testing.T, log []domain.ExecutionLogEntry, planners int) (*blockRig, domain.RunOutcome) {
	t.Helper()
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, sdSourceFile, dupSchedule())
	rig.resumeWithLog(sdWorkflow, domain.ExecutionModeAuto, log)
	for i := 0; i < planners; i++ {
		rig.queueProducer("planner", sdProducerMessage, runFileWrite(sdSourceFile, dupSchedule()))
	}
	rig.consult.queueStop("giving up")
	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(sdWorkflow, domain.ExecutionModeAuto))
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	return rig, got
}

// TestSession_Resume_AfterOneDefectiveAttempt_RedispatchesProducerAtTheRecordedValue
// verifies that a resume after one producer attempt re-dispatches the
// producer, with the fix request, and that two more defective attempts reach
// the limit: the log holds the first attempt, so only two re-dispatches remain.
func TestSession_Resume_AfterOneDefectiveAttempt_RedispatchesProducerAtTheRecordedValue(t *testing.T) {
	log := []domain.ExecutionLogEntry{plannerSuccess(1, sdProducerMessage)}

	rig, got := startResumed(t, log, 3)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	planners := rig.invocationsFor("planner")
	if len(planners) != 2 {
		t.Fatalf("want 2 re-dispatches after the logged attempt, got %d dispatches", len(planners))
	}
	requireFixRequest(t, planners, 0, sdSourceFile, duplicateIDParts()...)
	if rig.consult.CallCount != 1 {
		t.Errorf("want 1 consultation after the limit, got %d", rig.consult.CallCount)
	}
}

// TestSession_Resume_AfterTwoAttempts_RedispatchesOnceMoreThenConsults
// verifies that the count survives the resume: with two attempts in the log
// only one re-dispatch remains before the consultation.
func TestSession_Resume_AfterTwoAttempts_RedispatchesOnceMoreThenConsults(t *testing.T) {
	log := []domain.ExecutionLogEntry{plannerSuccess(1, "first"), plannerSuccess(2, "second")}

	rig, got := startResumed(t, log, 3)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 1 {
		t.Errorf("want exactly 1 re-dispatch after 2 logged attempts, got %d", n)
	}
	if rig.consult.CallCount != 1 {
		t.Errorf("want 1 consultation after the limit, got %d", rig.consult.CallCount)
	}
}

// TestSession_Resume_AfterThreeAttempts_ConsultsWithoutRedispatch verifies that
// a log that already holds three attempts leads straight to the consultation,
// carrying the logged summary as last_status_message.
func TestSession_Resume_AfterThreeAttempts_ConsultsWithoutRedispatch(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		plannerSuccess(1, "first"), plannerSuccess(2, "second"), plannerSuccess(3, sdProducerMessage),
	}

	rig, got := startResumed(t, log, 0)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 0 {
		t.Errorf("want no further re-dispatch once the limit is used up, got %d", n)
	}
	if rig.consult.CallCount != 1 {
		t.Fatalf("want 1 consultation, got %d", rig.consult.CallCount)
	}
	requireScheduleDefectConsult(t, rig.consult.Requests[0], duplicateIDParts()...)
}

// TestSession_Resume_InfrastructureEntryBetweenAttempts_DoesNotResetTheCount
// verifies that an infrastructure agent's entry between two attempts is
// skipped: the two attempts leave exactly one re-dispatch.
func TestSession_Resume_InfrastructureEntryBetweenAttempts_DoesNotResetTheCount(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		plannerSuccess(1, "first"),
		logStep(2, "checkpoint", "PLANNING", "", 0, domain.StatusSUCCESS, "checkpoint"),
		plannerSuccess(3, sdProducerMessage),
	}

	rig, got := startResumed(t, log, 3)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 1 {
		t.Errorf("want exactly 1 re-dispatch after 2 attempts around an infrastructure entry, got %d", n)
	}
}

// TestSession_Resume_BlockRowEntryBetweenAttempts_StartsAFreshCount verifies
// that an entry of a block row ends the trailing run: a later defect of the
// same producer starts a fresh count and gets both re-dispatches.
func TestSession_Resume_BlockRowEntryBetweenAttempts_StartsAFreshCount(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		plannerSuccess(1, "first"),
		logStep(2, "auditor", "EXECUTION", "A", 2, domain.StatusSUCCESS, "audited A"),
		plannerSuccess(3, sdProducerMessage),
	}

	rig, got := startResumed(t, log, 3)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 2 {
		t.Errorf("want 2 re-dispatches in a fresh count, got %d", n)
	}
}

// TestSession_Resume_PartiallyDoneEntryOfProducer_IsNotAnAttempt verifies that
// a PARTIALLY_DONE entry of the producer is not counted: two SUCCESS attempts
// around it leave exactly one re-dispatch.
func TestSession_Resume_PartiallyDoneEntryOfProducer_IsNotAnAttempt(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		plannerSuccess(1, "first"),
		logStep(2, "planner", "PLANNING", "", 1, domain.StatusPARTIALLY_DONE, "half"),
		plannerSuccess(3, sdProducerMessage),
	}

	rig, got := startResumed(t, log, 3)

	requireRunStatus(t, got, nil, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 1 {
		t.Errorf("want exactly 1 re-dispatch (PARTIALLY_DONE is not an attempt), got %d", n)
	}
}
