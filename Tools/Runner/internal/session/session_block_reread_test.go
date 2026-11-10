package session_test

// Tests of when a block's schedule is read again: after a row whose Output
// produces the block's declared Source (a producer outside the block or a
// writer inside it), and not after an output that merely contains Stage-*.
// Also: sibling blocks keep separate schedules, and per-iteration HITL comes
// from the schedule of the row's own block.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Start_ProducerRewritesDeclaredSource_ReadsScheduleBeforeBlockEntered
// verifies that a schedule read at run start is replaced by a re-read after
// the planner row, whose Output is the declared Source, so the block runs the
// planner's iterations and not the stale ones.
func TestSession_Start_ProducerRewritesDeclaredSource_ReadsScheduleBeforeBlockEntered(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", scheduleDoc("Stage", "Stages", ids("old")))
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B", "C"))))
	rig.queueOKTimes("auditor", 3)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B", "C"})
}

// TestSession_Start_InBlockWriterOfProducedSource_RereadsAfterWritingRow
// verifies that a row of the block whose Output is the block's Source causes
// a re-read after that row: an iteration it appends to the schedule is run.
func TestSession_Start_InBlockWriterOfProducedSource_RereadsAfterWritingRow(t *testing.T) {
	rig := newBlockRig(t, "writer-produced-orch.md", nil, "planner", "worker", "closer")
	rig.queueOK("planner", runFileWrite("Plan.md", scheduleDoc("Stage", "Stages", ids("A"))))
	// The first dispatch of the worker appends iteration B to the schedule.
	rig.queueOK("worker", runFileWrite("Plan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOK("worker")
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-produced", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "B"})
}

// TestSession_Start_InBlockWriterOfSeededSource_RereadsAfterWritingRow
// verifies the same re-read for a seeded schedule that one of its own block's
// rows writes.
func TestSession_Start_InBlockWriterOfSeededSource_RereadsAfterWritingRow(t *testing.T) {
	rig := newBlockRig(t, "writer-seeded-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Plan.md", scheduleDoc("Stage", "Stages", ids("A")))
	rig.queueOK("worker", runFileWrite("Plan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOK("worker")
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), rig.config("writer-seeded", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A", "B"})
}

// TestSession_Start_OutputWithStageStarProducingNoSource_DoesNotRereadSchedule
// verifies that an output containing Stage-* that matches no block Source
// triggers no re-read: the schedule the run started with stays in use even
// though the file changed on disk during the planner row.
func TestSession_Start_OutputWithStageStarProducingNoSource_DoesNotRereadSchedule(t *testing.T) {
	rig := newBlockRig(t, "stagestar-no-source-orch.md", nil, "planner", "worker")
	rig.writeRunFile(t, "Plan.md", scheduleDoc("Stage", "Stages", ids("A")))
	rig.queueOK("planner", runFileWrite("Plan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOKTimes("worker", 2)

	got, err := rig.ses.Start(context.Background(), rig.config("stagestar-no-source", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "out-", ".md"), []string{"A"})
}

// TestSession_Start_SiblingBlocksWithDifferentSources_EachRunsItsOwnSchedule
// verifies that two blocks, each with its own Source and a producer of its
// own, run the iterations of their own schedule and not each other's.
func TestSession_Start_SiblingBlocksWithDifferentSources_EachRunsItsOwnSchedule(t *testing.T) {
	rig := newBlockRig(t, "sibling-blocks-orch.md", nil, "planner", "alpha-worker", "bridge", "beta-worker", "finisher")
	rig.queueOK("planner", runFileWrite("AlphaPlan.md", scheduleDoc("Alpha", "Stages", ids("x1", "x2"))))
	rig.queueOKTimes("alpha-worker", 2)
	rig.queueOK("bridge", runFileWrite("BetaPlan.md", scheduleDoc("Beta", "Stages", ids("y1", "y2", "y3"))))
	rig.queueOKTimes("beta-worker", 3)
	rig.queueOK("finisher")

	got, err := rig.ses.Start(context.Background(), rig.config("sibling-blocks", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the alpha-worker ran at",
		rig.dispatchedIterations("alpha-worker", "alpha-", ".md"), []string{"x1", "x2"})
	requireSequence(t, "iterations the beta-worker ran at",
		rig.dispatchedIterations("beta-worker", "beta-", ".md"), []string{"y1", "y2", "y3"})
}

// TestSession_Start_PerIterationHITL_ComesFromTheScheduleOfTheRowsOwnBlock
// verifies that the HITL flag an iteration carries is the one in the schedule
// of the block the dispatched row belongs to. Both schedules hold the IDs s1
// and s2 with opposite flags, so a row that read the other block's schedule,
// or one shared schedule, would send the wrong flags.
func TestSession_Start_PerIterationHITL_ComesFromTheScheduleOfTheRowsOwnBlock(t *testing.T) {
	approved := &fixedApprovalReader{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "sibling-blocks-orch.md", approved, "planner", "alpha-worker", "bridge", "beta-worker", "finisher")
	alpha := []scheduleRow{{id: "s1", hitl: false}, {id: "s2", hitl: true}}
	beta := []scheduleRow{{id: "s1", hitl: true}, {id: "s2", hitl: false}}
	rig.queueOK("planner", runFileWrite("AlphaPlan.md", scheduleDoc("Alpha", "Stages", alpha)))
	rig.queueOKTimes("alpha-worker", 2)
	rig.queueOK("bridge", runFileWrite("BetaPlan.md", scheduleDoc("Beta", "Stages", beta)))
	rig.queueOKTimes("beta-worker", 2)
	rig.queueOK("finisher")

	got, err := rig.ses.Start(context.Background(), rig.config("sibling-blocks", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireFlags(t, "alpha-worker", rig.hitlOf("alpha-worker"), []bool{false, true})
	requireFlags(t, "beta-worker", rig.hitlOf("beta-worker"), []bool{true, false})
}

// TestSession_Start_DefectiveRereadAfterWritingRow_RedispatchesWriterAndIgnoresPreviousSchedule
// verifies that a schedule that is defective when read again after the row
// that wrote it is never used, not even the previous valid schedule: the
// writing row is dispatched again and the block runs the iterations of the
// schedule it then writes (C, D), not those of the previous one (A, B).
func TestSession_Start_DefectiveRereadAfterWritingRow_RedispatchesWriterAndIgnoresPreviousSchedule(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B")))
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("X", "x", "Y"))))
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("C", "D"))))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := rig.invocationsOf("planner"); n != 2 {
		t.Errorf("want the planner dispatched again after the defective schedule, got %d dispatches", n)
	}
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"C", "D"})
}

// TestSession_Start_ProducerOutputWithWildcard_ReadsScheduleOfMatchedSource
// verifies that a producer whose Output uses * within a path segment to match
// the declared Source (Audit*.md for AuditPlan.md) causes the re-read: the
// block runs the schedule the planner wrote, not the stale one on disk.
func TestSession_Start_ProducerOutputWithWildcard_ReadsScheduleOfMatchedSource(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-star-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", scheduleDoc("Stage", "Stages", ids("old")))
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block-star", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
}

// TestSession_Start_RowOfOneBlockWritesOtherBlocksSource_RunningScheduleKept
// verifies that a write to one block's Source by a row of another block that
// is running does not change the running block's schedule: the beta-worker
// also outputs AlphaPlan.md with different iterations, yet Beta keeps running
// the iterations of BetaPlan.md.
func TestSession_Start_RowOfOneBlockWritesOtherBlocksSource_RunningScheduleKept(t *testing.T) {
	rig := newBlockRig(t, "sibling-blocks-cross-writer-orch.md", nil, "planner", "alpha-worker", "bridge", "beta-worker", "finisher")
	rig.queueOK("planner", runFileWrite("AlphaPlan.md", scheduleDoc("Alpha", "Stages", ids("x1"))))
	rig.queueOK("alpha-worker")
	rig.queueOK("bridge", runFileWrite("BetaPlan.md", scheduleDoc("Beta", "Stages", ids("y1", "y2"))))
	rig.queueOK("beta-worker", runFileWrite("AlphaPlan.md", scheduleDoc("Alpha", "Stages", ids("z1", "z2", "z3"))))
	rig.queueOK("beta-worker")
	rig.queueOK("finisher")

	got, err := rig.ses.Start(context.Background(), rig.config("sibling-blocks-cross-writer", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the beta-worker ran at",
		rig.dispatchedIterations("beta-worker", "beta-", ".md"), []string{"y1", "y2"})
	requireSequence(t, "iterations the alpha-worker ran at",
		rig.dispatchedIterations("alpha-worker", "alpha-", ".md"), []string{"x1"})
}

// requireFlags asserts the human_in_the_loop values of an agent's dispatches.
func requireFlags(t *testing.T, agent string, got, want []bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: want %d dispatches with flags %v, got %d with %v", agent, len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s dispatch %d: want human_in_the_loop=%v, got %v", agent, i+1, want[i], got[i])
		}
	}
}
