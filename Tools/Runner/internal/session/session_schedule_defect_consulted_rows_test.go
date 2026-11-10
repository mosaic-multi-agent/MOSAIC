package session_test

// Orchestrated mode: which row the Runner re-dispatches for a defective
// schedule comes from the admitted model (the rows that list the block's
// Source as an Output), not from the artifact overrides of a consulted
// dispatch; and a row of the block that wrote the schedule is re-dispatched by
// the Runner at the iteration the consulted reply ran it at.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Consulted_ProducerDispatchedWithOutputOverrideOmittingSource_IsStillRedispatched
// verifies that a consulted planner dispatch whose OutputArtifacts override
// names a different file than the Source is still the producing row: the
// planner wrote a defective Source, so the Runner dispatches the planner again
// with the fix request, without consulting for it.
func TestSession_Consulted_ProducerDispatchedWithOutputOverrideOmittingSource_IsStillRedispatched(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(1)
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	override := []string{dispatchPath("planner-notes.md")}
	rig.consult.queueDispatchWithOutputs("planner", "write the audit plan", 0, &override)
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "dispatch order", rig.dispatchOrder(), []string{"planner", "planner"})
	if rig.consult.CallCount != 2 {
		t.Errorf("want 2 consultations (first dispatch, after the corrected schedule), got %d", rig.consult.CallCount)
	}
	requireFixRequest(t, rig.invocationsFor("planner"), 1, sdSourceFile, duplicateIDParts()...)
}

// TestSession_Consulted_BlockRowDispatchedWithOutputOverrideNamingSource_TriggersNoRedispatch
// verifies the converse: the auditor is not listed by the model as a writer of
// the Source, so a consulted dispatch that overrides its outputs to name the
// Source does not make it the producer; the defective file it wrote triggers
// no re-dispatch and no consultation of kind schedule-defect.
func TestSession_Consulted_BlockRowDispatchedWithOutputOverrideNamingSource_TriggersNoRedispatch(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueProducer("planner", "planned", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueProducer("auditor", "audited A", runFileWrite(sdSourceFile, dupSchedule()))
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	override := []string{dispatchPath(sdSourceFile)}
	rig.consult.queueDispatchWithOutputs("auditor", "audit item A", 1, &override)
	rig.consult.stageLastDispatch("A")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "dispatch order", rig.dispatchOrder(), []string{"planner", "auditor"})
	if rig.consult.CallCount != 3 {
		t.Errorf("want 3 consultations (planner, auditor, stop), got %d", rig.consult.CallCount)
	}
	for i, req := range rig.consult.Requests {
		if req.Deviation != nil && req.Deviation.Kind == domain.DeviationScheduleDefect {
			t.Errorf("want no consultation of kind %q, consultation %d is one", domain.DeviationScheduleDefect, i+1)
		}
	}
}

// TestSession_Consulted_InBlockWriterWritesDefectiveSchedule_RunnerRedispatchesItAtItsIteration
// verifies that the worker, dispatched by a consulted reply at iteration A,
// that wrote a defective schedule is dispatched again by the Runner at A with
// the fix request and without a consultation in between; the consulted
// dispatch of the worker at the iteration the corrected schedule added then
// proceeds.
func TestSession_Consulted_InBlockWriterWritesDefectiveSchedule_RunnerRedispatchesItAtItsIteration(t *testing.T) {
	rig := newBlockRig(t, "writer-produced-orch.md", nil, "planner", "worker", "closer")
	rig.queueProducer("planner", "planned", runFileWrite("Plan.md", okSchedule("A")))
	rig.queueProducer("worker", "worked A", runFileWrite("Plan.md", dupSchedule()))
	rig.queueProducer("worker", "worked A again", runFileWrite("Plan.md", okSchedule("A", "B")))
	rig.queueOK("worker")
	rig.consult.queueDispatch("planner", "write the plan", 0)
	rig.consult.queueStagedDispatch("worker", "work item A", 1, "A")
	rig.consult.queueStagedDispatch("worker", "work item B", 1, "B")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-produced", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "dispatch order", rig.dispatchOrder(), []string{"planner", "worker", "worker", "worker"})
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "A", "B"})
	if rig.consult.CallCount != 4 {
		t.Errorf("want 4 consultations (planner, worker A, after the corrected schedule, worker B), got %d",
			rig.consult.CallCount)
	}
	requireFixRequest(t, rig.invocationsFor("worker"), 1, "Plan.md", duplicateIDPartsFor("Plan.md")...)
}
