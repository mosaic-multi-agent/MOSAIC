package engine_test

// Resume inside blocks of WorkflowFormat 2.16 tables. The position after the
// last completed workflow step is the one scope routing gives, with iteration
// completion derived from the Execution Log: in the middle of a group, at a
// group boundary, between iterations and when Depends On reorders the
// schedule. In Modes 2 and 3 the first step Next dispatches from the same log
// is the resumed position.

import (
	"testing"

	"mosaic-run/internal/domain"
)

const resumeGroupUnchecked = -2

// resumeCase is a resume after the completed steps (given as "row:stage"
// keys, in order). With interrupted set, the last step did not complete and
// is run again.
type resumeCase struct {
	name        string
	steps       []string
	interrupted bool
	wantRow     int
	wantBlock   domain.BlockName
	wantIter    domain.IterationID
	wantGroup   int
}

// bndRunLog builds the Execution Log of steps, each one a SUCCESS entry
// carrying its recorded row.
func bndRunLog(aw domain.AdmittedWorkflow, steps []bndStep) *runLog {
	l := &runLog{}
	for _, s := range steps {
		row := aw.Table.Rows[s.row]
		l.workflowStep(row.BareAgentID(), s.stage, domain.StatusSUCCESS, s.row+1).lastInPhase(row.PhaseParsed.Name)
	}
	return l
}

// bndDispatched returns the steps as the dispatch steps Next produced for them.
func bndDispatched(aw domain.AdmittedWorkflow, steps []bndStep) []domain.DispatchStep {
	out := make([]domain.DispatchStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, domain.DispatchStep{
			RowIndex: s.row,
			Phase:    aw.Table.Rows[s.row].PhaseParsed.Name,
			Stage:    s.stage,
		})
	}
	return out
}

// runResumeCases resumes every case over aw and the schedules, and for a
// position that dispatches a row also asks Next in Modes 2 and 3.
func runResumeCases(t *testing.T, aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, cases []resumeCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps := bndSteps(t, tc.steps...)

			info, err := resumeLogIDs(aw, schedules, bndRunLog(aw, steps), tc.interrupted)

			requireResumeBlock(t, info, err, tc.wantRow, tc.interrupted, tc.wantBlock, tc.wantIter)
			if tc.wantGroup != resumeGroupUnchecked {
				requireResumeGroup(t, info, tc.wantGroup)
			}
			if tc.interrupted || tc.wantRow >= len(aw.Table.Rows) {
				return
			}
			forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
				next := itNextAfter(t, aw, mode, schedules, itLoggedSteps(bndDispatched(aw, steps)))
				if next.RowIndex != tc.wantRow || bndIteration(next.Stage) != tc.wantIter {
					t.Errorf("Next dispatches row %d iteration %q, the resume position is row %d iteration %q",
						next.RowIndex, bndIteration(next.Stage), tc.wantRow, tc.wantIter)
				}
			})
		})
	}
}

// The brownfield-tdd 4.0 shape: a Test group and an Implementation group
// closed by review rows whose On Success is COMPLETE.
// Regression coverage: the positions below coincide with the positional ones in
// the 4.0 shape. The discriminating resume tests are the early-COMPLETE,
// target-inside and reordered-schedule ones.
func TestResumeBlocks_Regression_GroupedBlock_ContinuesInsideIterationAndAcrossGroups(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	tdd := scheduleOf("Stage", idsOf("TDD", "1", "2")...)

	runResumeCases(t, aw, tdd, []resumeCase{
		{"in the middle of a group", []string{"0:", "1:1.Test"}, false, 2, "Stage", "1", 0},
		{"at the boundary of two groups", []string{"0:", "1:1.Test", "2:1.Test"}, false, 3, "Stage", "1", 1},
		{"between iterations", []string{
			"0:", "1:1.Test", "2:1.Test", "3:1.Implementation", "4:1.Implementation"}, false, 1, "Stage", "2", 0},
		{"after the last iteration", []string{
			"0:", "1:1.Test", "2:1.Test", "3:1.Implementation", "4:1.Implementation",
			"1:2.Test", "2:2.Test", "3:2.Implementation", "4:2.Implementation"}, false, 5, "", "", -1},
		{"interrupted in the second group", []string{
			"0:", "1:1.Test", "2:1.Test", "3:1.Implementation"}, true, 3, "Stage", "1", 1},
	})
}

// The first group the approach lists is the one that ran last.
func TestResumeBlocks_Regression_ImplementationFirst_ContinuesWithSecondListedGroup(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("Implementation-First", "1")...)

	runResumeCases(t, aw, schedules, []resumeCase{
		{"into the Test group", []string{"0:", "3:1.Implementation", "4:1.Implementation"}, false, 1, "Stage", "1", 0},
		{"after the Test group", []string{
			"0:", "3:1.Implementation", "4:1.Implementation", "1:1.Test", "2:1.Test"}, false, 5, "", "", -1},
	})
}

// A COMPLETE on the first row of a group ends the group there. The rows below
// it are skipped, so the position is not the next row of the table.
func TestResumeBlocks_CompleteOnRowThatIsNotLastOfGroup_ContinuesAtNextGroup(t *testing.T) {
	aw := mustAdmitScope(t, ieFirstGroupCompleteRows(), bsApproachTables(), domain.ExecutionModeAutoReview)

	t.Run("TDD continues in the Implementation group", func(t *testing.T) {
		schedules := scheduleOf("Stage", idsOf("TDD", "1", "2")...)
		runResumeCases(t, aw, schedules, []resumeCase{
			{"after the early COMPLETE", []string{"0:", "1:1.Test"}, false, 3, "Stage", "1", 1},
		})
	})
	t.Run("Tests-Only continues with the next iteration", func(t *testing.T) {
		schedules := scheduleOf("Stage", idsOf("Tests-Only", "1", "2")...)
		runResumeCases(t, aw, schedules, []resumeCase{
			{"after the early COMPLETE", []string{"0:", "1:1.Test"}, false, 1, "Stage", "2", 0},
		})
	})
}

// COMPLETE in the last group ends the iteration at that row: the next
// iteration starts, or the block is left when none remains.
func TestResumeBlocks_CompleteEndsIteration_ContinuesWithNextIterationOrLeavesBlock(t *testing.T) {
	aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	done := []string{"0:", "1:1.Test", "2:1.Test", "3:1.Implementation"}

	t.Run("another iteration remains", func(t *testing.T) {
		schedules := scheduleOf("Stage", idsOf("TDD", "1", "2")...)
		runResumeCases(t, aw, schedules, []resumeCase{
			{"next iteration", done, false, 1, "Stage", "2", 0},
		})
	})
	t.Run("last iteration", func(t *testing.T) {
		schedules := scheduleOf("Stage", idsOf("TDD", "1")...)
		runResumeCases(t, aw, schedules, []resumeCase{
			{"block is left", done, false, 5, "", "", -1},
		})
	})
}

// An On Success target skips the rows between; resume lands on the target and
// stays in the iteration.
func TestResumeBlocks_TargetInsideIteration_ContinuesAtTarget(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("first", "last"), bsBuild("skipped", ""), bsBuild("last", ""), bsReporter()}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("", "1", "2")...)

	runResumeCases(t, aw, schedules, []resumeCase{
		{"after the row with the target", []string{"0:", "1:1"}, false, 3, "Stage", "1", resumeGroupUnchecked},
		{"after the target row", []string{"0:", "1:1", "3:1"}, false, 1, "Stage", "2", resumeGroupUnchecked},
	})
}

// A (depends on C), B, C runs B, C, A: progression follows what the log shows
// completed, not the position in the schedule.
func TestResumeBlocks_ScheduleReorderedByDependsOn_FollowsCompletedIterations(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", itIteration("A", "C"), itIteration("B"), itIteration("C"))

	runResumeCases(t, aw, schedules, []resumeCase{
		{"in the middle of B", []string{"0:", "1:B"}, false, 2, "Stage", "B", resumeGroupUnchecked},
		{"after B, A is not runnable", []string{"0:", "1:B", "2:B"}, false, 1, "Stage", "C", resumeGroupUnchecked},
		{"after B and C, A runs", []string{"0:", "1:B", "2:B", "1:C", "2:C"}, false, 1, "Stage", "A", resumeGroupUnchecked},
		{"after A, the block is left", []string{
			"0:", "1:B", "2:B", "1:C", "2:C", "1:A", "2:A"}, false, 3, "", "", resumeGroupUnchecked},
		{"interrupted in the middle of C", []string{"0:", "1:B", "2:B", "1:C"}, true, 1, "Stage", "C", resumeGroupUnchecked},
	})
}

// A group ended early by an in-block On Success target: resume lands on the
// target, not on the next row of the table, and the group after it follows
// once the target row completed.
//
//	0 planner  1 t1 (-> t3)  2 t2 (skipped)  3 t3  4 i1  5 reporter
func TestResumeBlocks_TargetInsideGroup_ContinuesAtTargetThenNextGroup(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsTest("t1", "t3"), bsTest("t2", ""), bsTest("t3", ""), bsImpl("i1", ""), bsReporter()}
	aw := mustAdmitScope(t, rows, bsApproachTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("TDD", "1", "2")...)

	runResumeCases(t, aw, schedules, []resumeCase{
		{"middle of the group after the jump", []string{"0:", "1:1.Test"}, false, 3, "Stage", "1", 0},
		{"after the target row", []string{"0:", "1:1.Test", "3:1.Test"}, false, 4, "Stage", "1", 1},
	})
}

// The first listed group (Implementation) ends early by COMPLETE: the Test
// group follows although the rows below the COMPLETE row are skipped.
//
//	0 planner  1 t1  2 t2  3 i1 (COMPLETE)  4 skipped  5 reporter
func TestResumeBlocks_ImplementationFirstCompleteEarly_ContinuesWithTestGroup(t *testing.T) {
	aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	schedules := scheduleOf("Stage", idsOf("Implementation-First", "1", "2")...)

	runResumeCases(t, aw, schedules, []resumeCase{
		{"after the early COMPLETE", []string{"0:", "3:1.Implementation"}, false, 1, "Stage", "1", 0},
		{"after the Test group", []string{
			"0:", "3:1.Implementation", "1:1.Test", "2:1.Test"}, false, 3, "Stage", "2", 1},
	})
}

// Re-entry after an iteration ended early by COMPLETE: only the uncompleted
// iterations run, in a schedule that Depends On reorders, with groups.
//
//	A (depends on C), B, C with the TDD approach, each ending early in Implementation
func TestResumeBlocks_ReorderedScheduleWithGroups_FollowsCompletedIterations(t *testing.T) {
	aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	tdd := func(id domain.IterationID, dependsOn ...domain.IterationID) domain.ScheduleIteration {
		return domain.ScheduleIteration{ID: id, Approach: "TDD", DependsOn: dependsOn}
	}
	schedules := scheduleOf("Stage", tdd("A", "C"), tdd("B"), tdd("C"))
	b := []string{"0:", "1:B.Test", "2:B.Test", "3:B.Implementation"}

	runResumeCases(t, aw, schedules, []resumeCase{
		{"after B ended early, C runs", b, false, 1, "Stage", "C", 0},
		{"after B and C, A runs", append(append([]string{}, b...),
			"1:C.Test", "2:C.Test", "3:C.Implementation"), false, 1, "Stage", "A", 0},
	})
}
