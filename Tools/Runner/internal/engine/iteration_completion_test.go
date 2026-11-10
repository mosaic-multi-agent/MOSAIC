package engine_test

// Iteration completion as derived from the Execution Log when a block is
// entered again. A SUCCESS entry completes an iteration only when its row's
// On Success ends the scope of the last group the iteration's approach lists
// (the iteration itself for an ungrouped block): COMPLETE on any row of that
// scope, or NEXT on its last row. NEXT on a row that is not the last, and a
// target, end nothing. An entry whose recorded value cannot be read completes
// nothing, and iteration IDs in the log match the schedule ignoring case.
//
// Every test hands engine.Next a log literal: the planning row ran, then part
// of iteration 1 ran, then the planning row ran again and returned SUCCESS.
// The answer is the step dispatched when the block is entered again.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// ---- Fixtures ----

// icUngroupedCompleteRows has a COMPLETE on the first block row, so the second
// block row is never reached.
//
//	0 planner  1 worker (COMPLETE)  2 unreached  3 reporter
func icUngroupedCompleteRows() []scopeRow {
	return []scopeRow{bsPlanner(), bsBuild("worker", "COMPLETE"), bsBuild("unreached", ""), bsReporter()}
}

// icGroupedCompleteRows has a COMPLETE on the first row of the last group a
// TDD iteration lists.
//
//	0 planner  1 t1  2 t2  3 i1 (COMPLETE)  4 unreached  5 reporter
func icGroupedCompleteRows() []scopeRow {
	return []scopeRow{
		bsPlanner(), bsTest("t1", ""), bsTest("t2", ""), bsImpl("i1", "COMPLETE"), bsImpl("i-unreached", ""), bsReporter(),
	}
}

// icReentered is the step dispatched when the block is entered again after
// the planning row ran a second time. prefix is the played start of an
// iteration whose first step is the planning row.
func icReentered(t *testing.T, aw domain.AdmittedWorkflow, mode domain.ExecutionMode,
	schedules domain.BlockSchedules, prefix []domain.DispatchStep) domain.DispatchStep {
	t.Helper()
	logged := append(append([]domain.DispatchStep{}, prefix...), prefix[0])
	return itNextAfter(t, aw, mode, schedules, itLoggedSteps(logged))
}

// icReenteredAgain is icReentered for an iteration that has not completed: it
// runs again, and its first step already finished, so the engine names that
// step in a finished-work deviation instead of dispatching it.
func icReenteredAgain(t *testing.T, aw domain.AdmittedWorkflow, mode domain.ExecutionMode,
	schedules domain.BlockSchedules, prefix []domain.DispatchStep) domain.DispatchStep {
	t.Helper()
	logged := append(append([]domain.DispatchStep{}, prefix...), prefix[0])
	return itFinishedStepAfter(t, aw, mode, schedules, itLoggedSteps(logged))
}

// icWithStage returns a copy of steps in which every step from index from on
// records the given stage value.
func icWithStage(steps []domain.DispatchStep, from int, value string) []domain.DispatchStep {
	out := append([]domain.DispatchStep{}, steps...)
	for i := from; i < len(out); i++ {
		out[i].Stage = value
	}
	return out
}

// requireStep asserts the row and recorded stage value of a dispatched step.
func requireStep(t *testing.T, step domain.DispatchStep, row int, stage string) {
	t.Helper()
	if step.RowIndex != row || step.Stage != stage {
		t.Errorf("want row %d stage %q, got row %d stage %q", row, stage, step.RowIndex, step.Stage)
	}
}

// requireStepOfIteration asserts that the step belongs to the iteration whose
// recorded stage value is one of stages and that its row is one of rows: the
// iteration runs again, from its start or from where it stopped.
func requireStepOfIteration(t *testing.T, step domain.DispatchStep, rows []int, stages ...string) {
	t.Helper()
	rowOK := false
	for _, r := range rows {
		rowOK = rowOK || step.RowIndex == r
	}
	stageOK := false
	for _, s := range stages {
		stageOK = stageOK || step.Stage == s
	}
	if !rowOK || !stageOK {
		t.Errorf("want one of rows %v with stage in %q (the iteration runs again), got row %d stage %q",
			rows, stages, step.RowIndex, step.Stage)
	}
}

// ---- COMPLETE on a row that is not the last of its scope completes ----

func TestIterationCompletion_UngroupedCompleteOnFirstRow_IterationIsComplete(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icUngroupedCompleteRows(), stageBlockTables(), mode)
		prefix := itPlayed(t, aw, mode, itIteration("1"))[:2]

		t.Run("next iteration runs", func(t *testing.T) {
			step := icReentered(t, aw, mode, scheduleOf("Stage", itIteration("1"), itIteration("2")), prefix)

			requireStep(t, step, 1, "2")
		})
		t.Run("block exits when it was the last", func(t *testing.T) {
			step := icReentered(t, aw, mode, scheduleOf("Stage", itIteration("1")), prefix)

			requireStep(t, step, 3, "")
		})
	})
}

func TestIterationCompletion_GroupedCompleteOnFirstRowOfLastGroup_IterationIsComplete(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), mode)
		prefix := itPlayed(t, aw, mode, domain.ScheduleIteration{ID: "1", Approach: "TDD"})[:4]

		t.Run("next iteration runs", func(t *testing.T) {
			step := icReentered(t, aw, mode, scheduleOf("Stage",
				domain.ScheduleIteration{ID: "1", Approach: "TDD"},
				domain.ScheduleIteration{ID: "2", Approach: "TDD"}), prefix)

			requireStep(t, step, 1, "2.Test")
		})
		t.Run("block exits when it was the last", func(t *testing.T) {
			step := icReentered(t, aw, mode, scheduleOf("Stage",
				domain.ScheduleIteration{ID: "1", Approach: "TDD"}), prefix)

			requireStep(t, step, 5, "")
		})
	})
}

// ---- NEXT mid-scope and a target complete nothing ----

func TestIterationCompletion_NextOnRowThatIsNotLastOfScope_IterationRunsAgain(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		prefix := itPlayed(t, aw, mode, itIteration("1"))[:2]

		step := icReenteredAgain(t, aw, mode, scheduleOf("Stage", itIteration("1"), itIteration("2")), prefix)

		requireStepOfIteration(t, step, []int{1, 2}, "1")
	})
}

func TestIterationCompletion_TargetOnRow_IterationRunsAgain(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("first", "last"), bsBuild("skipped", ""), bsBuild("last", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		prefix := itPlayed(t, aw, mode, itIteration("1"))[:2]

		step := icReenteredAgain(t, aw, mode, scheduleOf("Stage", itIteration("1"), itIteration("2")), prefix)

		requireStepOfIteration(t, step, []int{1, 3}, "1")
	})
}

// ---- Entries that cannot be matched to an iteration ----

// A completing entry whose recorded value is missing completes nothing; that
// is not an error and the iteration is not skipped.
func TestIterationCompletion_UngroupedEntryWithMissingValue_IterationRunsAgain(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icUngroupedCompleteRows(), stageBlockTables(), mode)
		prefix := icWithStage(itPlayed(t, aw, mode, itIteration("1"))[:2], 1, "")

		step := icReentered(t, aw, mode, scheduleOf("Stage", itIteration("1"), itIteration("2")), prefix)

		requireStepOfIteration(t, step, []int{1}, "1")
	})
}

// A grouped value must name both group and iteration; the last group's
// completing entry with only the group name completes nothing.
func TestIterationCompletion_GroupedEntryWithMalformedValue_IterationRunsAgain(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), mode)
		played := itPlayed(t, aw, mode, domain.ScheduleIteration{ID: "1", Approach: "TDD"})[:4]
		prefix := icWithStage(played, 3, "Implementation")

		step := icReenteredAgain(t, aw, mode, scheduleOf("Stage",
			domain.ScheduleIteration{ID: "1", Approach: "TDD"},
			domain.ScheduleIteration{ID: "2", Approach: "TDD"}), prefix)

		requireStepOfIteration(t, step, []int{1, 3}, "1.Test", "1.Implementation")
	})
}

// A log entry recorded as "a" completes the schedule's iteration "A".
func TestIterationCompletion_LogIterationIdIsMatchedIgnoringCase(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icUngroupedCompleteRows(), stageBlockTables(), mode)
		prefix := icWithStage(itPlayed(t, aw, mode, itIteration("A"))[:2], 1, "a")

		step := icReentered(t, aw, mode, scheduleOf("Stage", itIteration("A"), itIteration("B")), prefix)

		requireStep(t, step, 1, "B")
	})
}
