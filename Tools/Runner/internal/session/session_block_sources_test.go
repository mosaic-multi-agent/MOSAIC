package session_test

// Tests that the session reads a block's schedule from the Source and Section
// the workflow declares for it, whatever the artifact is named, and runs
// every iteration of that schedule.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Start_BlockScheduleFromAuditPlan_DispatchesEveryIteration
// verifies that a block whose declared Source is AuditPlan.md, written by the
// planner row (no Plan.md exists anywhere), runs one iteration per schedule
// row and then continues at the row after the block.
func TestSession_Start_BlockScheduleFromAuditPlan_DispatchesEveryIteration(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B", "C"))))
	rig.queueOKTimes("auditor", 3)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B", "C"})
	if n := rig.invocationsOf("merger"); n != 1 {
		t.Errorf("want the merger dispatched once after the block exits, got %d", n)
	}
}

// TestSession_Start_BlockScheduleFromNonPlanArtifact_DispatchesEveryIteration
// verifies that a Source whose name does not end in Plan.md, with the
// schedule under its own section after another table, is read from that
// section.
func TestSession_Start_BlockScheduleFromNonPlanArtifact_DispatchesEveryIteration(t *testing.T) {
	rig := newBlockRig(t, "kb-progress-block-orch.md", nil, "bootstrapper", "writer", "indexer")
	rig.queueOK("bootstrapper", runFileWrite("KBProgress.md", scheduleDoc("Generation", "Generation", ids("intro", "design"))))
	rig.queueOKTimes("writer", 2)
	rig.queueOK("indexer")

	got, err := rig.ses.Start(context.Background(), rig.config("kb-progress-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the writer ran at",
		rig.dispatchedIterations("writer", "docs/", ".md"), []string{"intro", "design"})
	if n := rig.invocationsOf("indexer"); n != 1 {
		t.Errorf("want the indexer dispatched once after the block exits, got %d", n)
	}
}

// TestSession_Start_SeededPlanWithFirstRowABlockRow_DispatchesEveryIteration
// verifies that a seeded Plan.md is read at run start when the first row of
// the workflow is a block row, and that every iteration runs both of the
// block's rows. This is regression coverage of the seeded Plan.md shape (the
// run-root Plan.md is also the fixed path of today's code); the discriminating
// seeded case is the one with a Source of another name, in
// session_block_seeded_source_test.go.
func TestSession_Start_SeededPlanWithFirstRowABlockRow_DispatchesEveryIteration(t *testing.T) {
	rig := newBlockRig(t, "seeded-plan-first-block-orch.md", nil, "implementer", "reviewer", "test-runner")
	rig.writeRunFile(t, "Plan.md", scheduleDoc("Stage", "Stages", ids("a", "b")))
	rig.queueOKTimes("implementer", 2)
	rig.queueOKTimes("reviewer", 2)
	rig.queueOK("test-runner")

	got, err := rig.ses.Start(context.Background(), rig.config("seeded-plan-first-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the implementer ran at",
		rig.dispatchedIterations("implementer", "Stage-", "/PlanProgress.md"), []string{"a", "b"})
	requireSequence(t, "iterations the reviewer ran at",
		rig.dispatchedIterations("reviewer", "Stage-", "/review.md"), []string{"a", "b"})
	if n := rig.invocationsOf("test-runner"); n != 1 {
		t.Errorf("want the test-runner dispatched once after the block exits, got %d", n)
	}
}

// TestSession_Start_ProducedSourceNeverWritten_RedispatchesProducerThenEndsNamingBlockAndSource
// verifies that when the row producing a block's declared Source keeps
// succeeding without writing it and no routing consultant is wired, the
// producer is dispatched three times in total (the first attempt and two
// re-dispatches), no iteration is dispatched, and the run ends unresolved with
// a message naming the block and the Source. Only the presence of both names
// is asserted, not the wording.
func TestSession_Start_ProducedSourceNeverWritten_RedispatchesProducerThenEndsNamingBlockAndSource(t *testing.T) {
	rig := newNoConsultantRig(t, "audit-plan-block-finding-orch.md", "planner", "auditor")
	rig.queueOKTimes("planner", 3)

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block-finding", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunDeviationUnresolved)
	requireNames(t, "outcome message", got.Message, "Finding", "AuditPlan.md")
	if n := rig.invocationsOf("planner"); n != 3 {
		t.Errorf("want 3 planner attempts (initial plus 2 re-dispatches), got %d", n)
	}
	if n := rig.invocationsOf("auditor"); n != 0 {
		t.Errorf("want no iteration dispatched without a schedule, got %d dispatches of auditor", n)
	}
}
