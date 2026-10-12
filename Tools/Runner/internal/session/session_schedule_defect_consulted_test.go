package session_test

// Orchestrated mode: a defective produced schedule is a defect of the
// producing subagent, so the Runner re-dispatches the producing row itself,
// without consulting the orchestrator for the re-dispatch. After two
// re-dispatches the consultation follows, and a consulted reply that
// dispatches a block row while the schedule is defective is rejected.

import (
	"context"
	"errors"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Consulted_ProducedScheduleDefective_RunnerRedispatchesWithoutConsulting
// verifies that, after a consulted planner dispatch wrote a defective
// schedule, the Runner dispatches the planner again with the fix request and
// consults only once the corrected schedule is in place: the orchestrator's
// next dispatch, a block row, then proceeds.
func TestSession_Consulted_ProducedScheduleDefective_RunnerRedispatchesWithoutConsulting(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(1)
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueOK("auditor")
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStagedDispatch("auditor", "audit item A", 1, "A")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "dispatch order", rig.dispatchOrder(), []string{"planner", "planner", "auditor"})
	if rig.consult.CallCount != 3 {
		t.Errorf("want 3 consultations (first dispatch, after the corrected schedule, after the auditor), got %d",
			rig.consult.CallCount)
	}
	requireFixRequest(t, rig.invocationsFor("planner"), 1, sdSourceFile, duplicateIDParts()...)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A"})
}

// TestSession_Consulted_ProducedScheduleStillDefectiveAfterLimit_ConsultsWithDefect
// verifies that three defective attempts lead to a consultation carrying the
// defect in last_error_reason, and that the Runner made the two re-dispatches
// without consulting.
func TestSession_Consulted_ProducedScheduleStillDefectiveAfterLimit_ConsultsWithDefect(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(3)
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 3 {
		t.Errorf("want 3 planner attempts (consulted dispatch plus 2 re-dispatches), got %d", n)
	}
	if rig.consult.CallCount != 2 {
		t.Fatalf("want 2 consultations (first dispatch, the exhausted limit), got %d", rig.consult.CallCount)
	}
	requireScheduleDefectConsult(t, rig.consult.Requests[1], duplicateIDParts()...)
}

// TestSession_Consulted_ReplyDispatchingBlockRowWhileScheduleDefective_IsRejected
// verifies that after the limit a consulted reply that dispatches a block row
// is rejected as an invalid dispatch target and that row never runs, although a
// valid schedule was in force before the planner overwrote it.
func TestSession_Consulted_ReplyDispatchingBlockRowWhileScheduleDefective_IsRejected(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
	rig.queueDefectivePlanner(3)
	rig.queueOK("auditor")
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStagedDispatch("auditor", "audit item A", 1, "A")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("auditor"); n != 0 {
		t.Errorf("want no block row dispatched against the defective schedule, got %d", n)
	}
	var consultErr *domain.ConsultationError
	if !errors.As(got.Cause, &consultErr) || consultErr.Failure != domain.ConsultFailMalformedJSON {
		t.Fatalf("want Cause to be a ConsultationError of class %q, got %T: %v",
			domain.ConsultFailMalformedJSON, got.Cause, got.Cause)
	}
	var targetErr *domain.DispatchTargetError
	if !errors.As(got.Cause, &targetErr) {
		t.Errorf("want Cause to wrap a DispatchTargetError, got %v", got.Cause)
	}
}

// TestSession_Consulted_ReplyDispatchingTheProducerWhileScheduleDefective_IsAccepted
// verifies that the consultant can send the producer again after the limit: the
// reply is accepted and the corrected schedule lets a later block-row dispatch
// run.
func TestSession_Consulted_ReplyDispatchingTheProducerWhileScheduleDefective_IsAccepted(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(3)
	rig.queueProducer("planner", "fixed by hand", runFileWrite(sdSourceFile, okSchedule("A")))
	rig.queueOK("auditor")
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueDispatch("planner", "fix the audit plan", 0)
	rig.consult.queueStagedDispatch("auditor", "audit item A", 1, "A")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A"})
}
