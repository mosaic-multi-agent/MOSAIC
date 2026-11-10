package session_test

// A row of a block that writes the block's own Source while the block runs is
// the producer of that schedule: when the schedule it wrote is defective, that
// same row is re-dispatched at the iteration it ran at, whether the block's
// Source is produced before the block or seeded. A completed row the admitted
// model does not list as a writer of the block triggers nothing.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_InBlockWriterWritesDefectiveSchedule_RedispatchesThatRowAtItsIteration
// verifies, for a produced block, that the worker that wrote a defective
// schedule at iteration A is dispatched again at A (not the planner outside the
// block, and no other row of the block), with the fix request; a corrected
// schedule then lets the block continue to the iteration it added.
func TestSession_InBlockWriterWritesDefectiveSchedule_RedispatchesThatRowAtItsIteration(t *testing.T) {
	rig := newBlockRig(t, "writer-produced-orch.md", nil, "planner", "worker", "closer")
	rig.queueProducer("planner", "planned", runFileWrite("Plan.md", okSchedule("A")))
	rig.queueProducer("worker", "worked A", runFileWrite("Plan.md", dupSchedule()))
	rig.queueProducer("worker", "worked A again", runFileWrite("Plan.md", okSchedule("A", "B")))
	rig.queueOK("worker")
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-produced", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "dispatch order", rig.dispatchOrder(),
		[]string{"planner", "worker", "worker", "worker", "closer"})
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "A", "B"})
	requireFixRequest(t, rig.invocationsFor("worker"), 1, "Plan.md", duplicateIDPartsFor("Plan.md")...)
}

// TestSession_InBlockWriterStillDefectiveAfterLimit_ConsultsAfterThreeAttempts
// verifies that the count for an in-block writer follows the same basis: the
// worker's attempts at iteration A are bounded at three, then the consultation
// follows with the defect, and nothing else of the block ran.
func TestSession_InBlockWriterStillDefectiveAfterLimit_ConsultsAfterThreeAttempts(t *testing.T) {
	rig := newBlockRig(t, "writer-produced-orch.md", nil, "planner", "worker", "closer")
	rig.queueProducer("planner", "planned", runFileWrite("Plan.md", okSchedule("A")))
	for i := 0; i < 3; i++ {
		rig.queueProducer("worker", sdProducerMessage, runFileWrite("Plan.md", dupSchedule()))
	}
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-produced", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "A", "A"})
	if n := rig.invocationsOf("planner"); n != 1 {
		t.Errorf("want the planner outside the block dispatched once, got %d", n)
	}
	if rig.consult.CallCount != 1 {
		t.Fatalf("want exactly 1 consultation, got %d", rig.consult.CallCount)
	}
	requireScheduleDefectConsult(t, rig.consult.Requests[0], duplicateIDPartsFor("Plan.md")...)
}

// TestSession_SeededBlockRowWritesDefectiveSchedule_RedispatchesThatRowAtItsIteration
// verifies the same for a block whose Source is a seed: the worker that wrote
// the defective schedule is dispatched again at the same iteration (the seeded
// refusal applies to the read at run start only), and the corrected schedule
// lets the block continue.
func TestSession_SeededBlockRowWritesDefectiveSchedule_RedispatchesThatRowAtItsIteration(t *testing.T) {
	rig := newBlockRig(t, "writer-seeded-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Plan.md", okSchedule("A"))
	rig.queueProducer("worker", "worked A", runFileWrite("Plan.md", dupSchedule()))
	rig.queueProducer("worker", "worked A again", runFileWrite("Plan.md", okSchedule("A", "B")))
	rig.queueOK("worker")
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-seeded", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "dispatch order", rig.dispatchOrder(), []string{"worker", "worker", "worker", "closer"})
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "A", "B"})
	requireFixRequest(t, rig.invocationsFor("worker"), 1, "Plan.md", duplicateIDPartsFor("Plan.md")...)
}

// TestSession_SeededBlockRowStillDefectiveAfterLimit_ConsultsAfterThreeAttempts
// verifies that a seeded block's in-block writer has the same limit and
// consultation: three attempts at iteration A, then the consultation with the
// defect, and nothing else of the block ran.
func TestSession_SeededBlockRowStillDefectiveAfterLimit_ConsultsAfterThreeAttempts(t *testing.T) {
	rig := newBlockRig(t, "writer-seeded-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Plan.md", okSchedule("A"))
	for i := 0; i < 3; i++ {
		rig.queueProducer("worker", sdProducerMessage, runFileWrite("Plan.md", dupSchedule()))
	}
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-seeded", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "A", "A"})
	if rig.consult.CallCount != 1 {
		t.Fatalf("want exactly 1 consultation, got %d", rig.consult.CallCount)
	}
	requireScheduleDefectConsult(t, rig.consult.Requests[0], duplicateIDPartsFor("Plan.md")...)
}

// TestSession_InBlockWriterDefectAtSecondIteration_StartsAFreshCount
// verifies in a live run that the attempts are counted per iteration value:
// the worker is defective once at A (fixed by one re-dispatch) and once at B
// (fixed by one re-dispatch); four worker attempts in all complete the run
// without a consultation.
func TestSession_InBlockWriterDefectAtSecondIteration_StartsAFreshCount(t *testing.T) {
	rig := newBlockRig(t, "writer-produced-orch.md", nil, "planner", "worker", "closer")
	rig.queueProducer("planner", "planned", runFileWrite("Plan.md", okSchedule("A")))
	rig.queueProducer("worker", "worked A", runFileWrite("Plan.md", dupSchedule()))
	rig.queueProducer("worker", "worked A again", runFileWrite("Plan.md", okSchedule("A", "B")))
	rig.queueProducer("worker", "worked B", runFileWrite("Plan.md", dupSchedule()))
	rig.queueProducer("worker", "worked B again", runFileWrite("Plan.md", okSchedule("A", "B")))
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-produced", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "A", "B", "B"})
	if rig.consult.CallCount != 0 {
		t.Errorf("want no consultation, got %d", rig.consult.CallCount)
	}
}

// TestSession_RowNotListedAsWriter_DefectiveFileItWroteTriggersNoRedispatch
// verifies that a completed row whose declared Output is not the block's Source
// triggers no re-read and no re-dispatch, even when the subagent wrote a
// defective file at that path: the auditor's Output is out-{Stage}.md, so
// the schedule the planner wrote keeps driving the block.
func TestSession_RowNotListedAsWriter_DefectiveFileItWroteTriggersNoRedispatch(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueProducer("planner", "planned", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueProducer("auditor", "audited A", runFileWrite(sdSourceFile, dupSchedule()))
	rig.queueOK("auditor")
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "dispatch order", rig.dispatchOrder(),
		[]string{"planner", "auditor", "auditor", "merger"})
}

// duplicateIDPartsFor is duplicateIDParts for a Source with the given name.
func duplicateIDPartsFor(source string) []string {
	return []string{"V2", sdBlockName, source, sdSection, sdDuplicateToken}
}
