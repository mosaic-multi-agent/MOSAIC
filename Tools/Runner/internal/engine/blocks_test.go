package engine_test

// Block entry and exit over the per-block admitted model: several sibling
// blocks, blocks on any phase, empty schedules, and resume positions inside
// blocks that are not named EXECUTION.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// siblingRows: planner produces both sources, rows 2-3 are the ungrouped block
// Stage on EXECUTION, rows 4-5 the ungrouped block Audit on REVIEW, row 6 is
// the reporter.
func siblingRows() []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
		{phase: "REVIEW.[Audit]", agent: "audit-fixer"},
		{phase: "REPORT", agent: "reporter"},
	}
}

func siblingTables() string {
	return scopeBlocks(stageBlockRow, "| Audit | Audit.md | Items |")
}

func siblingSchedules(stageIDs, auditIDs []domain.IterationID) domain.BlockSchedules {
	return joinSchedules(
		scheduleOf("Stage", idsOf("", stageIDs...)...),
		scheduleOf("Audit", idsOf("", auditIDs...)...),
	)
}

func TestBlocks_SiblingBlocks_AreEnteredAndExitedInTurn(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, siblingRows(), siblingTables(), mode)
		schedules := siblingSchedules([]domain.IterationID{"1", "2"}, []domain.IterationID{"x1", "x2"})

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2, 3, 4, 3, 4, 5})
		requireStages(t, trace, []string{"", "1", "1", "2", "2", "x1", "x1", "x2", "x2", ""})
	})
}

func TestBlocks_EmptySchedule_ExitsBlockAtOnceAndContinuesAfterIt(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "BUILD.[Stage]", agent: "checker"},
		{phase: "REPORT", agent: "reporter"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage")

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 3})
	})
}

func TestBlocks_EmptySchedule_BlockAtEndOfTable_CompletesRun(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "BUILD.[Stage]", agent: "checker"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage"), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0})
	})
}

func TestBlocks_NonEmptyBlockAtEndOfTable_ExitCompletesRun(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "BUILD.[Stage]", agent: "checker"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("", "1", "2")...), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2})
		requireStages(t, trace, []string{"", "1", "1", "2", "2"})
	})
}

func TestBlocks_EmptyScheduleOfFirstSibling_ExitsIntoTheSecondSibling(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, siblingRows(), siblingTables(), mode)
		schedules := siblingSchedules(nil, []domain.IterationID{"x1"})

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 3, 4, 5})
		requireStages(t, trace, []string{"", "x1", "x1", ""})
	})
}

func TestBlocks_EmptyScheduleOfSecondSibling_ExitsToRowAfterIt(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, siblingRows(), siblingTables(), mode)
		schedules := siblingSchedules([]domain.IterationID{"1"}, nil)

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 5})
	})
}

func TestBlocks_TwoBlocksOnPhasesOtherThanExecution_RunInTurn(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "VERIFY.[Audit]", agent: "verifier"},
		{phase: "REPORT", agent: "reporter"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, siblingTables(), mode)
		schedules := siblingSchedules([]domain.IterationID{"1", "2"}, []domain.IterationID{"x1"})

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 1, 2, 3})
		requireStages(t, trace, []string{"", "1", "2", "x1", ""})
	})
}

// A block row records the bare phase name of the row, whatever it is.
func TestBlocks_NonExecutionBlock_RecordsBarePhaseOfRow(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "REPORT", agent: "reporter"},
	}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("", "1")...), mode: domain.ExecutionModeAutoReview}

	trace := run.play(t)

	if len(trace.steps) < 2 {
		t.Fatalf("want a dispatch of the block row, got rows %v", trace.rows())
	}
	if got := trace.steps[1].Phase; got != "BUILD" {
		t.Errorf("recorded phase of the block row: want BUILD, got %q", got)
	}
}

// ---- Resume inside a block that is not named EXECUTION ----

func buildBlockRows() []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "BUILD.[Stage]", agent: "checker"},
		{phase: "REPORT", agent: "reporter"},
	}
}

func TestBlocks_ResumeInterruptedEntryWithoutRecordedRow_StaysInTheBlockIteration(t *testing.T) {
	aw := mustAdmitScope(t, buildBlockRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("", "1", "2", "3")...)
	log := (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("builder", "2", domain.StatusSUCCESS, 2).lastInPhase("BUILD").
		workflowStep("checker", "2", domain.StatusSUCCESS, 0).lastInPhase("BUILD")

	info, err := resumeLogIDs(aw, schedules, log, true)

	requireResumeBlock(t, info, err, 2, true, "Stage", "2")
	requireResumeGroup(t, info, 0)
}

func TestBlocks_ResumeAdvancedEntryWithoutRecordedRow_ContinuesInTheBlockIteration(t *testing.T) {
	aw := mustAdmitScope(t, buildBlockRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("", "1", "2", "3")...)
	log := (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("builder", "2", domain.StatusSUCCESS, 0).lastInPhase("BUILD")

	info, err := resumeLogIDs(aw, schedules, log, false)

	requireResumeBlock(t, info, err, 2, false, "Stage", "2")
}

func TestBlocks_NextAfterEntryWithoutRecordedRow_DispatchesNextBlockRowOfSameIteration(t *testing.T) {
	aw := mustAdmitScope(t, buildBlockRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("", "1", "2", "3")...)
	log := (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("builder", "2", domain.StatusSUCCESS, 0).lastInPhase("BUILD")

	step := requireNextStepIDs(t, aw, schedules, scopeAgents(aw), log)

	if step.RowIndex != 2 || step.Stage != "2" {
		t.Errorf("want row 2 of iteration 2, got row %d stage %q", step.RowIndex, step.Stage)
	}
}

// The position after the last row of a block is the first row of the next
// sibling block, and its iteration is the first iteration of that block ("x1");
// only a fresh start carries no iteration.
func TestBlocks_ResumeAfterLastRowOfBlock_PositionsAtFirstRowOfNextSiblingBlock(t *testing.T) {
	aw := mustAdmitScope(t, siblingRows(), siblingTables(), domain.ExecutionModeAutoReview)
	schedules := siblingSchedules([]domain.IterationID{"1"}, []domain.IterationID{"x1", "x2"})
	log := (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("impl-a", "1", domain.StatusSUCCESS, 2).
		workflowStep("impl-b", "1", domain.StatusSUCCESS, 3)

	info, err := resumeLogIDs(aw, schedules, log, false)

	requireResumeBlock(t, info, err, 3, false, "Audit", "x1")
	requireResumeGroup(t, info, 0)
}

func TestBlocks_ResumeAfterLastRowOfLastBlock_PositionsAtRowAfterIt(t *testing.T) {
	aw := mustAdmitScope(t, siblingRows(), siblingTables(), domain.ExecutionModeAutoReview)
	schedules := siblingSchedules([]domain.IterationID{"1"}, []domain.IterationID{"x1"})
	log := (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("auditor", "x1", domain.StatusSUCCESS, 4).lastInPhase("REVIEW").
		workflowStep("audit-fixer", "x1", domain.StatusSUCCESS, 5).lastInPhase("REVIEW")

	info, err := resumeLogIDs(aw, schedules, log, false)

	requireResumeBlock(t, info, err, 5, false, "", "")
	requireResumeGroup(t, info, -1)
}

// ---- A row named EXECUTION that belongs to no block ----

// Membership of a block is decided by the block model, never by the phase
// name: a plain row on the phase EXECUTION is resolved like any other plain
// row, from a log entry that records no row.
func TestBlocks_PlainRowOnPhaseNamedExecution_IsNotTreatedAsBlockRow(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION", agent: "runner-a"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "REPORT", agent: "reporter"},
	}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("", "1")...)
	log := (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("runner-a", "", domain.StatusSUCCESS, 0)

	step := requireNextStepIDs(t, aw, schedules, scopeAgents(aw), log)

	if step.RowIndex != 2 || step.Stage != "1" {
		t.Errorf("want the first row of block iteration 1 after the plain row, got row %d stage %q", step.RowIndex, step.Stage)
	}
	info, err := resumeLogIDs(aw, schedules, log, false)
	requireResumeBlock(t, info, err, 2, false, "Stage", "1")
}

// requireResumeBlock asserts a successful resume at the given zero-based row,
// block and iteration.
func requireResumeBlock(
	t *testing.T, info domain.ResumeInfo, err error,
	wantRow int, wantRerun bool, wantBlock domain.BlockName, wantIteration domain.IterationID,
) {
	t.Helper()
	if err != nil {
		t.Fatalf("ResumePointFromSchedules: unexpected error: %v", err)
	}
	if info.RowIndex != wantRow || info.RerunLast != wantRerun {
		t.Errorf("position: want row %d rerun %v, got row %d rerun %v", wantRow, wantRerun, info.RowIndex, info.RerunLast)
	}
	if info.Block != wantBlock || info.Iteration != wantIteration {
		t.Errorf("block/iteration: want %q/%q, got %q/%q", wantBlock, wantIteration, info.Block, info.Iteration)
	}
}
