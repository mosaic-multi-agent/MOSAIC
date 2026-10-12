package session_test

// Orchestrated-mode counterparts of the schedule re-read tests: rows the
// routing consultant selected. The re-read after such a row follows the same
// trigger as in the other modes: an output that produces the block's declared
// Source, including a row of the block that writes it, and not an output that
// merely contains Stage-*.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Start_ConsultedOutputWithStageStarProducingNoSource_DoesNotRereadSchedule
// verifies that, in orchestrated mode, a consulted row whose output contains
// Stage-* but matches no block Source triggers no re-read. The planner rewrites
// Plan.md so the HITL flag of iteration A flips from FALSE to TRUE with the
// same IDs; the worker dispatched at A still carries the flag of the schedule
// held at start. A literal Stage-* trigger would read the new flag.
func TestSession_Start_ConsultedOutputWithStageStarProducingNoSource_DoesNotRereadSchedule(t *testing.T) {
	approved := &fixedApprovalReader{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "stagestar-no-source-orch.md", approved, "planner", "worker")
	rig.writeRunFile(t, "Plan.md", scheduleDoc("Stage", "Stages", ids("A")))
	flipped := []scheduleRow{{id: "A", hitl: true}}
	rig.queueOK("planner", runFileWrite("Plan.md", scheduleDoc("Stage", "Stages", flipped)))
	rig.queueOK("worker")
	rig.consult.queueDispatch("planner", "write the notes", 0)
	rig.consult.queueStagedDispatch("worker", "work item A", 1, "A")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("stagestar-no-source", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireFlags(t, "worker", rig.hitlOf("worker"), []bool{false})
}

// TestSession_Start_ConsultedInBlockWriterOfSeededSource_RereadsAfterWritingRow
// verifies that, in orchestrated mode, a row of the block whose Output is the
// block's seeded Source causes a re-read after that row: iteration B, which the
// worker appended at A, is selectable only if the schedule was read again.
func TestSession_Start_ConsultedInBlockWriterOfSeededSource_RereadsAfterWritingRow(t *testing.T) {
	rig := newBlockRig(t, "writer-seeded-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Plan.md", scheduleDoc("Stage", "Stages", ids("A")))
	rig.queueOK("worker", runFileWrite("Plan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOK("worker")
	rig.consult.queueStagedDispatch("worker", "work item A", 0, "A")
	rig.consult.queueStagedDispatch("worker", "work item B", 0, "B")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-seeded", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "B"})
}
