package engine_test

// On Success routing inside a block on WorkflowFormat 2.16 tables in Modes 2
// and 3. A grouped block has one scope per group within the iteration; an
// ungrouped block has the iteration as its one scope. After a block row
// succeeds, its typed entry routes within that scope: a target dispatches that
// row (above or below), NEXT dispatches the row below when it is in the same
// scope and otherwise ends the scope, COMPLETE ends the scope. Ending a group's
// scope moves the iteration to the next group its approach lists; after the
// last listed group the iteration has completed.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

// ---- Fixtures ----

const (
	bsGroupTest = "EXECUTION.[Stage].Test"
	bsGroupImpl = "EXECUTION.[Stage].Implementation"
)

// bsApproachTables declares block Stage with groups Test and Implementation
// and the four approaches of the brownfield workflow.
func bsApproachTables() string {
	return scopeBlocks(stageBlockRow) + scopeGroups(
		"| Stage | TDD | Test, Implementation |",
		"| Stage | Implementation-First | Implementation, Test |",
		"| Stage | Implementation-Only | Implementation |",
		"| Stage | Tests-Only | Test |",
	)
}

func bsPlanner() scopeRow {
	return scopeRow{phase: "PLANNING", agent: "planner", output: "Plan.md"}
}

func bsReporter() scopeRow { return scopeRow{phase: "REPORT", agent: "reporter"} }

func bsBuild(agent, onSuccess string) scopeRow {
	return scopeRow{phase: "BUILD.[Stage]", agent: agent, onSuccess: onSuccess}
}

func bsTest(agent, onSuccess string) scopeRow {
	return scopeRow{phase: bsGroupTest, agent: agent, onSuccess: onSuccess}
}

func bsImpl(agent, onSuccess string) scopeRow {
	return scopeRow{phase: bsGroupImpl, agent: agent, onSuccess: onSuccess}
}

// bsTddShapeRows is the shape of the 4.0 brownfield-tdd workflow around its
// block: each group ends with a review row whose On Success is COMPLETE and
// whose On Findings names the group's producing row.
//
//	0 contracts-review  1 test-writer  2 tests-review  3 implementer  4 impl-review  5 test-runner
func bsTddShapeRows() []scopeRow {
	return []scopeRow{
		{phase: "DESIGN", agent: "contracts-review", output: "Plan.md", onSuccess: "[Stage]"},
		bsTest("test-writer", "tests-review"),
		{phase: bsGroupTest, agent: "tests-review", onSuccess: "COMPLETE", onFindings: "test-writer"},
		bsImpl("implementer", "impl-review"),
		{phase: bsGroupImpl, agent: "impl-review", onSuccess: "COMPLETE", onFindings: "implementer"},
		{phase: "REVIEW", agent: "test-runner", onFindings: "contracts-review"},
	}
}

// bsTddShapeStage is the recorded stage value of table row r (zero-based) of
// bsTddShapeRows in the iteration with the given ID.
func bsTddShapeStage(r int, id string) string {
	switch r {
	case 1, 2:
		return id + ".Test"
	case 3, 4:
		return id + ".Implementation"
	}
	return ""
}

// bsExpectation returns the rows and recorded stages a run of bsTddShapeRows
// dispatches when the block's iterations (given by ID, each with the rows its
// approach selects) run in order between the first and the last row.
func bsExpectation(ids []string, rowsPer [][]int) ([]int, []string) {
	rows, stages := []int{0}, []string{""}
	for i, id := range ids {
		for _, r := range rowsPer[i] {
			rows = append(rows, r)
			stages = append(stages, bsTddShapeStage(r, id))
		}
	}
	return append(rows, 5), append(stages, "")
}

// bsApproachRows is the rows each approach selects, in run order.
var bsApproachRows = map[domain.Approach][]int{
	"TDD":                  {1, 2, 3, 4},
	"Implementation-First": {3, 4, 1, 2},
	"Implementation-Only":  {3, 4},
	"Tests-Only":           {1, 2},
}

// ---- Ungrouped block: the iteration is the scope ----

func TestBlockScope_Ungrouped_Next_DispatchesRowBelowAndLastRowEndsIteration(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("", "1", "2")...), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2, 3})
		requireStages(t, trace, []string{"", "1", "1", "2", "2", ""})
	})
}

func TestBlockScope_Ungrouped_Complete_EndsIterationAndSkipsRowsBelow(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", "COMPLETE"), bsBuild("unreached", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("", "1", "2")...), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 1, 3})
		requireStages(t, trace, []string{"", "1", "2", ""})
	})
}

func TestBlockScope_Ungrouped_TargetBelow_SkipsRowsBetweenWithinIteration(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("first", "last"), bsBuild("skipped", ""), bsBuild("last", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("", "1", "2")...), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 3, 1, 3, 4})
		requireStages(t, trace, []string{"", "1", "1", "2", "2", ""})
	})
}

func TestBlockScope_Ungrouped_TargetAbove_DispatchesNamedRowThenEndsIterationOnComplete(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(),
		bsBuild("first", "third"),
		bsBuild("second", "COMPLETE"),
		bsBuild("third", "second"),
		bsReporter(),
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("", "1", "2")...), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 3, 2, 1, 3, 2, 4})
		requireStages(t, trace, []string{"", "1", "1", "1", "2", "2", "2", ""})
	})
}

// ---- Grouped block: each group is a scope ----

func TestBlockScope_Grouped_Next_AtGroupEnd_StartsNextGroupOfIterationsApproach(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(), bsTest("t1", ""), bsTest("t2", ""), bsImpl("i1", ""), bsImpl("i2", ""), bsReporter(),
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, bsApproachTables(), mode)
		schedules := scheduleOf("Stage",
			domain.ScheduleIteration{ID: "1", Approach: "TDD"},
			domain.ScheduleIteration{ID: "2", Approach: "Implementation-First"})

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 3, 4, 3, 4, 1, 2, 5})
		requireStages(t, trace, []string{
			"", "1.Test", "1.Test", "1.Implementation", "1.Implementation",
			"2.Implementation", "2.Implementation", "2.Test", "2.Test", "",
		})
	})
}

func TestBlockScope_Grouped_Complete_MidGroupEndsThatGroupOnly(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(), bsTest("t1", "COMPLETE"), bsTest("t-unreached", ""), bsImpl("i1", ""), bsImpl("i2", ""), bsReporter(),
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, bsApproachTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("TDD", "1")...), mode: mode}.play(t)

		// The Test group ends after its first row; the iteration moves on to
		// the Implementation group and does not repeat or skip it.
		requireRowsThenComplete(t, trace, []int{0, 1, 3, 4, 5})
		requireStages(t, trace, []string{"", "1.Test", "1.Implementation", "1.Implementation", ""})
	})
}

func TestBlockScope_Grouped_Complete_MidLastGroupEndsIteration(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(), bsTest("t1", ""), bsTest("t2", ""), bsImpl("i1", "COMPLETE"), bsImpl("i-unreached", ""), bsReporter(),
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, bsApproachTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("TDD", "1", "2")...), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 3, 1, 2, 3, 5})
		requireStages(t, trace, []string{
			"", "1.Test", "1.Test", "1.Implementation", "2.Test", "2.Test", "2.Implementation", "",
		})
	})
}

func TestBlockScope_Grouped_Target_RoutesAboveAndBelowWithinGroup(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(),
		bsTest("t-first", "t-third"),
		bsTest("t-second", "COMPLETE"),
		bsTest("t-third", "t-second"),
		bsImpl("i1", ""),
		bsReporter(),
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, bsApproachTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("TDD", "1")...), mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 3, 2, 4, 5})
		requireStages(t, trace, []string{
			"", "1.Test", "1.Test", "1.Test", "1.Implementation", "",
		})
	})
}

func TestBlockScope_Grouped_ApproachOrderDiffersFromTableOrder_Brownfield4Shape(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), mode)

		trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf("Implementation-First", "1", "2")...), mode: mode}.play(t)

		// The Test group is the last one Implementation-First lists, so it
		// ends each iteration.
		wantRows, wantStages := bsExpectation([]string{"1", "2"},
			[][]int{bsApproachRows["Implementation-First"], bsApproachRows["Implementation-First"]})
		requireRowsThenComplete(t, trace, wantRows)
		requireStages(t, trace, wantStages)
	})
}

// ---- The four approaches of the 4.0 brownfield shape ----

func TestBlockScope_FourApproaches_RunOnlySelectedGroupsInOrder(t *testing.T) {
	approaches := []domain.Approach{"TDD", "Implementation-First", "Implementation-Only", "Tests-Only"}
	for _, approach := range approaches {
		for _, ids := range [][]string{{"1"}, {"1", "2", "3"}} {
			t.Run(string(approach)+"_"+string(rune('0'+len(ids)))+"_iterations", func(t *testing.T) {
				forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
					aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), mode)
					its := make([]domain.IterationID, 0, len(ids))
					rowsPer := make([][]int, 0, len(ids))
					for _, id := range ids {
						its = append(its, domain.IterationID(id))
						rowsPer = append(rowsPer, bsApproachRows[approach])
					}
					wantRows, wantStages := bsExpectation(ids, rowsPer)

					trace := scopeRun{aw: aw, schedules: scheduleOf("Stage", idsOf(approach, its...)...), mode: mode}.play(t)

					requireRowsThenComplete(t, trace, wantRows)
					requireStages(t, trace, wantStages)
				})
			})
		}
	}
}

func TestBlockScope_FourApproaches_MixedInOneSchedule_EachIterationRunsItsOwnGroups(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), mode)
		schedules := scheduleOf("Stage",
			domain.ScheduleIteration{ID: "1", Approach: "Tests-Only"},
			domain.ScheduleIteration{ID: "2", Approach: "TDD"},
			domain.ScheduleIteration{ID: "3", Approach: "Implementation-Only"},
			domain.ScheduleIteration{ID: "4", Approach: "Implementation-First"})
		wantRows, wantStages := bsExpectation([]string{"1", "2", "3", "4"}, [][]int{
			bsApproachRows["Tests-Only"], bsApproachRows["TDD"],
			bsApproachRows["Implementation-Only"], bsApproachRows["Implementation-First"],
		})

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, wantRows)
		requireStages(t, trace, wantStages)
	})
}

// ---- Findings inside a block (Mode 3) ----

// bsFindingsOnce scripts COMPLETED_NEEDS_ACTION for the first n visits of the
// step the predicate selects and SUCCESS for every other step.
func bsFindingsOnce(n int, match func(domain.DispatchStep) bool) func(int, domain.DispatchStep) domain.StatusCode {
	given := 0
	return func(_ int, step domain.DispatchStep) domain.StatusCode {
		if match(step) && given < n {
			given++
			return domain.StatusCOMPLETED_NEEDS_ACTION
		}
		return domain.StatusSUCCESS
	}
}

func TestBlockScope_Findings_InGroup_RouteBackKeepsIterationAndGroup(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw:        aw,
		schedules: scheduleOf("Stage", idsOf("TDD", "1", "2")...),
		mode:      domain.ExecutionModeAutoReview,
		// Findings from the tests review of the second iteration, once.
		statusOf: bsFindingsOnce(1, func(s domain.DispatchStep) bool {
			return s.RowIndex == 2 && s.Stage == "2.Test"
		}),
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 1, 2, 3, 4, 1, 2, 1, 2, 3, 4, 5})
	requireStages(t, trace, []string{
		"", "1.Test", "1.Test", "1.Implementation", "1.Implementation",
		"2.Test", "2.Test", "2.Test", "2.Test", "2.Implementation", "2.Implementation", "",
	})
}

// Implementation-First runs the Test group (table-first) last; a route-back in
// it keeps the iteration and the group.
func TestBlockScope_Findings_InSecondListedGroup_RouteBackKeepsIterationAndGroup(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw:        aw,
		schedules: scheduleOf("Stage", idsOf("Implementation-First", "1", "2")...),
		mode:      domain.ExecutionModeAutoReview,
		// Findings from the tests review of the first iteration, once.
		statusOf: bsFindingsOnce(1, func(s domain.DispatchStep) bool {
			return s.RowIndex == 2 && s.Stage == "1.Test"
		}),
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 3, 4, 1, 2, 1, 2, 3, 4, 1, 2, 5})
	requireStages(t, trace, []string{
		"", "1.Implementation", "1.Implementation", "1.Test", "1.Test", "1.Test", "1.Test",
		"2.Implementation", "2.Implementation", "2.Test", "2.Test", "",
	})
}

// After the route-back target succeeds, routing continues by that row's own
// On Success: here a jump over a row the fixed sequence would run.
func TestBlockScope_Findings_RouteBackTargetThenRoutesByItsOnSuccess(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(),
		bsBuild("worker", "reviewer"),
		bsBuild("skipped", ""),
		{phase: "BUILD.[Stage]", agent: "reviewer", onFindings: "worker"},
		bsReporter(),
	}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw:        aw,
		schedules: scheduleOf("Stage", idsOf("", "1", "2")...),
		mode:      domain.ExecutionModeAutoReview,
		statusOf: bsFindingsOnce(1, func(s domain.DispatchStep) bool {
			return s.RowIndex == 3 && s.Stage == "2"
		}),
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 1, 3, 1, 3, 1, 3, 4})
	requireStages(t, trace, []string{"", "1", "1", "2", "2", "2", "2", ""})
}

func TestBlockScope_Findings_InGroup_ReviewLoopLimitStillStopsRouteBack(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw:        aw,
		schedules: scheduleOf("Stage", idsOf("Implementation-Only", "1")...),
		mode:      domain.ExecutionModeAutoReview,
		statusOf:  bsFindingsOnce(10, func(s domain.DispatchStep) bool { return s.RowIndex == 4 }),
		loopLimit: 2,
	}

	trace := run.play(t)

	if got, want := trace.rows(), []int{0, 3, 4, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("dispatched rows before the limit: want %v, got %v", want, got)
	}
	if trace.final.Deviation == nil || trace.final.Deviation.Info.Kind != domain.DeviationReviewLoopLimit {
		t.Fatalf("want a review-loop-limit deviation, got %s", trace.describe())
	}
}
