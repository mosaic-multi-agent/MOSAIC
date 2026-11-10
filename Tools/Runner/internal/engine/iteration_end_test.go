package engine_test

// Iteration end (the answer behind ITERATION_END) on WorkflowFormat 2.16 tables.
// A step that returned SUCCESS ends its iteration when its On Success ends the
// scope of the last group the iteration's approach lists (the iteration itself
// for an ungrouped block): COMPLETE on any row of that scope, NEXT on its last
// row. It is the routing that decides, not the row's position in the table.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ieFirstGroupCompleteRows ends its Test group with COMPLETE on the first
// row, so the second Test row is never reached.
//
//	0 planner  1 t1 (COMPLETE)  2 t2 (unreached)  3 i1  4 i2  5 reporter
func ieFirstGroupCompleteRows() []scopeRow {
	return []scopeRow{
		bsPlanner(), bsTest("t1", "COMPLETE"), bsTest("t2", ""), bsImpl("i1", ""), bsImpl("i2", ""), bsReporter(),
	}
}

// Regression coverage: in the 4.0 shape the last row of the last listed group
// is also where routing ends the scope, so these answers equal the positional
// ones. The discriminating iteration-end tests are the COMPLETE-early ones.
func TestIterationEnd_Regression_EachApproach_EndsAfterReviewRowOfItsLastGroup(t *testing.T) {
	cases := []struct {
		approach domain.Approach
		want     []string
	}{
		{"TDD", []string{"4:1.Implementation"}},
		{"Implementation-First", []string{"2:1.Test"}},
		{"Implementation-Only", []string{"4:1.Implementation"}},
		{"Tests-Only", []string{"2:1.Test"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.approach), func(t *testing.T) {
			forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
				aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), mode)
				schedules := scheduleOf("Stage", idsOf(tc.approach, "1")...)
				steps := bndPlay(t, aw, mode, schedules)

				got := bndIterationEnds(aw, schedules, mode, steps)

				requireEnds(t, "iteration end", got, tc.want)
			})
		})
	}
}

func TestIterationEnd_Regression_IterationsWithDifferentApproaches_EachEndsAtItsOwnLastGroup(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), mode)
		schedules := scheduleOf("Stage",
			domain.ScheduleIteration{ID: "1", Approach: "TDD"},
			domain.ScheduleIteration{ID: "2", Approach: "Tests-Only"},
			domain.ScheduleIteration{ID: "3", Approach: "Implementation-First"})
		steps := bndPlay(t, aw, mode, schedules)

		got := bndIterationEnds(aw, schedules, mode, steps)

		requireEnds(t, "iteration end", got, []string{"4:1.Implementation", "2:2.Test", "2:3.Test"})
	})
}

// A COMPLETE on a row that is not the last row of the last group ends the
// iteration there; the rows below it are never run.
func TestIterationEnd_GroupEndedEarlyByComplete_EndsAtThatRow(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), mode)
		schedules := scheduleOf("Stage", idsOf("TDD", "1", "2")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndIterationEnds(aw, schedules, mode, steps)

		requireEnds(t, "iteration end", got, []string{"3:1.Implementation", "3:2.Implementation"})
	})
}

// COMPLETE on a row of a group that is not the last one listed ends that
// group only. The same row ends the iteration when the approach lists no
// group after its group. Tests-Only and Implementation-First discriminate
// from the table position (the COMPLETE row is not the last row of its group);
// TDD and Implementation-Only agree with it and are kept as support.
func TestIterationEnd_CompleteInFirstGroup_EndsIterationOnlyWhenNoGroupFollows(t *testing.T) {
	cases := []struct {
		approach domain.Approach
		want     []string
	}{
		{"TDD", []string{"4:1.Implementation"}},
		{"Implementation-First", []string{"1:1.Test"}},
		{"Implementation-Only", []string{"4:1.Implementation"}},
		{"Tests-Only", []string{"1:1.Test"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.approach), func(t *testing.T) {
			forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
				aw := mustAdmitScope(t, ieFirstGroupCompleteRows(), bsApproachTables(), mode)
				schedules := scheduleOf("Stage", idsOf(tc.approach, "1")...)
				steps := bndPlay(t, aw, mode, schedules)

				got := bndIterationEnds(aw, schedules, mode, steps)

				requireEnds(t, "iteration end", got, tc.want)
			})
		})
	}
}

func TestIterationEnd_Regression_UngroupedBlock_EndsAtLastRowOfEachIteration(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndIterationEnds(aw, schedules, mode, steps)

		requireEnds(t, "iteration end", got, []string{"2:1", "2:2"})
	})
}

func TestIterationEnd_UngroupedBlockCompleteOnFirstRow_EndsAtThatRow(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icUngroupedCompleteRows(), stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndIterationEnds(aw, schedules, mode, steps)

		requireEnds(t, "iteration end", got, []string{"1:1", "1:2"})
	})
}

// A target that jumps over rows keeps the iteration running; the row it lands
// on, the last of the scope, ends it.
func TestIterationEnd_Regression_TargetInsideIteration_EndsAtRowReachedLast(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("first", "last"), bsBuild("skipped", ""), bsBuild("last", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndIterationEnds(aw, schedules, mode, steps)

		requireEnds(t, "iteration end", got, []string{"3:1"})
	})
}

// Regression coverage: a row outside every block never ends an iteration; the
// positional answer is the same.
func TestIterationEnd_Regression_RowOutsideEveryBlock_IsNeverIterationEnd(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icUngroupedCompleteRows(), stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1")...)
		steps := bndStepsOf(scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t))
		last := len(steps) - 1

		got := engine.IsLastRowOfIteration(bndInputAt(aw, schedules, mode, steps, last))

		if got {
			t.Errorf("row %d is outside every block and must not end an iteration", steps[last].row)
		}
	})
}

// Regression coverage: a block with no entry in the schedules cannot say which
// iteration ends; the positional answer is false as well, because the approach
// cannot be resolved.
func TestIterationEnd_Regression_BlockWithoutSchedule_IsNeverIterationEnd(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), mode)
		steps := bndPlay(t, aw, mode, scheduleOf("Stage", idsOf("TDD", "1")...))

		got := bndIterationEnds(aw, domain.BlockSchedules{}, mode, steps)

		requireEnds(t, "iteration end without a schedule", got, []string{})
	})
}
