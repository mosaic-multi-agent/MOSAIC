package engine_test

// A recorded Iteration value is read with the recorded row, in the 3.0 spelling
// "{ID}.{Group}" (grouped block) or "{ID}" verbatim (ungrouped block). Position
// resolution, resume, iteration completion and in-block routing give the same
// result for a log recorded in that spelling, with non-numeric IDs. The pre-3.0
// "{Group}.{ID}" is a value the row does not accept, and "Stage-{ID}" on an
// ungrouped row is the ordinary ID "Stage-{ID}".
//
// The case lists below are written with step keys "row:Group.ID" and are
// resumed with every value rewritten to the 3.0 spelling.

import (
	"errors"
	"testing"

	"mosaic-run/internal/domain"
)

// ivRunGroupedForms resumes every case over a grouped block with every value
// rewritten to the 3.0 spelling.
func ivRunGroupedForms(t *testing.T, aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, cases []resumeCase) {
	t.Helper()
	runResumeCases(t, aw, schedules, ivConvertCases(cases, ivKeyToThreeOh))
}

func TestIterationValueForms_GroupedBlock_SameResumeAndRoutingForTheThreeOhSpelling(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	tdd := scheduleOf("Stage", idsOf("TDD", "A", "B")...)

	ivRunGroupedForms(t, aw, tdd, []resumeCase{
		{"in the middle of a group", []string{"0:", "1:Test.A"}, false, 2, "Stage", "A", 0},
		{"at the boundary of two groups", []string{"0:", "1:Test.A", "2:Test.A"}, false, 3, "Stage", "A", 1},
		{"between iterations", []string{
			"0:", "1:Test.A", "2:Test.A", "3:Implementation.A", "4:Implementation.A"}, false, 1, "Stage", "B", 0},
		{"after the last iteration", []string{
			"0:", "1:Test.A", "2:Test.A", "3:Implementation.A", "4:Implementation.A",
			"1:Test.B", "2:Test.B", "3:Implementation.B", "4:Implementation.B"}, false, 5, "", "", -1},
		{"interrupted in the second group", []string{
			"0:", "1:Test.A", "2:Test.A", "3:Implementation.A"}, true, 3, "Stage", "A", 1},
	})
}

func TestIterationValueForms_GroupedBlock_ImplementationFirstInTheThreeOhSpelling(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("Implementation-First", "alpha")...)

	ivRunGroupedForms(t, aw, schedules, []resumeCase{
		{"into the Test group", []string{
			"0:", "3:Implementation.alpha", "4:Implementation.alpha"}, false, 1, "Stage", "alpha", 0},
		{"after the Test group", []string{
			"0:", "3:Implementation.alpha", "4:Implementation.alpha", "1:Test.alpha", "2:Test.alpha"},
			false, 5, "", "", -1},
	})
}

// An On Success target skips the rows between; the position stays in the
// iteration whichever spelling the log holds.
func TestIterationValueForms_GroupedBlock_TargetInsideGroupInTheThreeOhSpelling(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsTest("t1", "t3"), bsTest("t2", ""), bsTest("t3", ""), bsImpl("i1", ""), bsReporter()}
	aw := mustAdmitScope(t, rows, bsApproachTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("TDD", "A", "B")...)

	ivRunGroupedForms(t, aw, schedules, []resumeCase{
		{"middle of the group after the jump", []string{"0:", "1:Test.A"}, false, 3, "Stage", "A", 0},
		{"after the target row", []string{"0:", "1:Test.A", "3:Test.A"}, false, 4, "Stage", "A", 1},
	})
}

// A (depends on C), B, C runs B, C, A: completion is derived from the values
// the log holds, so the 3.0 spelling must complete the same iterations.
func TestIterationValueForms_GroupedBlock_ReorderedScheduleInTheThreeOhSpelling(t *testing.T) {
	aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	tdd := func(id domain.IterationID, dependsOn ...domain.IterationID) domain.ScheduleIteration {
		return domain.ScheduleIteration{ID: id, Approach: "TDD", DependsOn: dependsOn}
	}
	schedules := scheduleOf("Stage", tdd("A", "C"), tdd("B"), tdd("C"))
	b := []string{"0:", "1:Test.B", "2:Test.B", "3:Implementation.B"}

	ivRunGroupedForms(t, aw, schedules, []resumeCase{
		{"after B ended early, C runs", b, false, 1, "Stage", "C", 0},
		{"after B and C, A runs", append(append([]string{}, b...),
			"1:Test.C", "2:Test.C", "3:Implementation.C"), false, 1, "Stage", "A", 0},
	})
}

// A pre-3.0 value on the last row of the last group completes no iteration:
// it is not an error, and the iteration runs again once the others are done.
func TestIterationValueForms_GroupedBlock_PreThreeOhEntryOfTheLastRow_CompletesNoIteration(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	tdd := scheduleOf("Stage", idsOf("TDD", "A", "B")...)

	runResumeCases(t, aw, tdd, []resumeCase{
		{"the iteration with the pre-3.0 entry runs again", []string{
			"0:", "1:A.Test", "2:A.Test", "3:A.Implementation", "4:Implementation.A",
			"1:B.Test", "2:B.Test", "3:B.Implementation", "4:B.Implementation"}, false, 1, "Stage", "A", 0},
	})
}

// The pre-3.0 "{Group}.{ID}" value on the entry that defines the position is a
// value the row does not accept.
func TestIterationValueForms_GroupedBlock_PreThreeOhLastEntry_PositionUnresolved(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("TDD", "A")...)

	_, err := resumeLogIDs(aw, schedules, bndRunLog(aw, bndSteps(t, "0:", "1:Test.A")), false)

	var perr *domain.PositionUnresolvedError
	if !errors.As(err, &perr) {
		t.Fatalf("want a *PositionUnresolvedError, got %v", err)
	}
	if perr.Cause != domain.CauseRecordedRowInvalid {
		t.Errorf("position cause: want %d, got %d (%v)", domain.CauseRecordedRowInvalid, perr.Cause, perr)
	}
}

// An entry of the last row of a group whose value the row does not accept
// completes no iteration: it is not an error, and the iteration runs again once
// the others are done instead of being skipped.
func TestIterationValueForms_GroupedBlock_UnreadableEntryOfTheLastRow_CompletesNoIteration(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	tdd := scheduleOf("Stage", idsOf("TDD", "A", "B")...)

	ivRunGroupedForms(t, aw, tdd, []resumeCase{
		{"the iteration with the unreadable entry runs again", []string{
			"0:", "1:Test.A", "2:Test.A", "3:Implementation.A", "4:Wrong.A",
			"1:Test.B", "2:Test.B", "3:Implementation.B", "4:Implementation.B"}, false, 1, "Stage", "A", 0},
	})
}

func TestIterationValueForms_UngroupedBlock_PlainIDsResumeAndRoute(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", itIteration("A", "C"), itIteration("B"), itIteration("C"))
	cases := []resumeCase{
		{"in the middle of B", []string{"0:", "1:B"}, false, 2, "Stage", "B", resumeGroupUnchecked},
		{"after B, A is not runnable", []string{"0:", "1:B", "2:B"}, false, 1, "Stage", "C", resumeGroupUnchecked},
		{"after B and C, A runs", []string{"0:", "1:B", "2:B", "1:C", "2:C"}, false, 1, "Stage", "A", resumeGroupUnchecked},
		{"after A, the block is left", []string{
			"0:", "1:B", "2:B", "1:C", "2:C", "1:A", "2:A"}, false, 3, "", "", resumeGroupUnchecked},
		{"interrupted in the middle of C", []string{"0:", "1:B", "2:B", "1:C"}, true, 1, "Stage", "C", resumeGroupUnchecked},
	}

	runResumeCases(t, aw, schedules, cases)
}

// "Stage-B" is an ordinary schedule ID: the value is read verbatim as the ID
// "Stage-B" and never as "B".
func TestIterationValueForms_UngroupedBlock_StageDashIDsAreOrdinaryIDs(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", itIteration("Stage-A", "Stage-C"), itIteration("Stage-B"), itIteration("Stage-C"))

	runResumeCases(t, aw, schedules, []resumeCase{
		{"in the middle of Stage-B", []string{"0:", "1:Stage-B"}, false, 2, "Stage", "Stage-B", resumeGroupUnchecked},
		{"after Stage-B, Stage-A is not runnable", []string{
			"0:", "1:Stage-B", "2:Stage-B"}, false, 1, "Stage", "Stage-C", resumeGroupUnchecked},
		{"after Stage-B and Stage-C, Stage-A runs", []string{
			"0:", "1:Stage-B", "2:Stage-B", "1:Stage-C", "2:Stage-C"}, false, 1, "Stage", "Stage-A", resumeGroupUnchecked},
		{"after Stage-A, the block is left", []string{
			"0:", "1:Stage-B", "2:Stage-B", "1:Stage-C", "2:Stage-C", "1:Stage-A", "2:Stage-A"},
			false, 3, "", "", resumeGroupUnchecked},
	})
}

// A grouped value on an ungrouped row, or an ID without group on a grouped
// row, is not a value that row accepts: the position cannot be resolved.
func TestIterationValueForms_ValueTheRowDoesNotAccept_PositionUnresolved(t *testing.T) {
	grouped := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	ungroupedRows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsReporter()}
	ungrouped := mustAdmitScope(t, ungroupedRows, stageBlockTables(), domain.ExecutionModeAutoReview)
	cases := []struct {
		name  string
		aw    domain.AdmittedWorkflow
		steps []string
		want  domain.PositionUnresolvedCause
	}{
		{"3.0 group that is not the row's", grouped, []string{"0:", "1:A.Implementation"}, domain.CauseRecordedRowInvalid},
		{"ID without group on a grouped row", grouped, []string{"0:", "1:A"}, domain.CauseRecordedRowInvalid},
		{"group on an ungrouped row", ungrouped, []string{"0:", "1:A.Test"}, domain.CauseRecordedRowInvalid},
		{"empty value on a grouped row", grouped, []string{"0:", "1:"}, domain.CauseStagedRowWithoutStage},
		{"ID on a row outside every block", grouped, []string{"0:A"}, domain.CauseRecordedRowInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schedules := scheduleOf("Stage", idsOf("TDD", "A")...)

			_, err := resumeLogIDs(tc.aw, schedules, bndRunLog(tc.aw, bndSteps(t, tc.steps...)), false)

			var perr *domain.PositionUnresolvedError
			if !errors.As(err, &perr) {
				t.Fatalf("want a *PositionUnresolvedError, got %v", err)
			}
			if perr.Cause != tc.want {
				t.Errorf("position cause: want %d, got %d (%v)", tc.want, perr.Cause, perr)
			}
		})
	}
}
