package engine_test

// Resume positions for each kind of On Success entry that scope routing must
// handle: entering a block, COMPLETE on a run-scope row, a Mode 1 fork list,
// and a block that has no entry in the schedules. In Modes 2 and 3 the first
// step Next dispatches from the same log is the resumed position, where Next
// dispatches at all.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// resumeKindRows enters the block Stage by an explicit block entry.
//
//	0 planner (-> [Stage])  1 worker  2 checker  3 reporter
func resumeKindRows() []scopeRow {
	rows := missingScheduleRows()
	rows[0].onSuccess = "[Stage]"
	return rows
}

// A block entry runs the first runnable uncompleted iteration, not the first
// one of the schedule. Entering again after iteration 1 completed continues with
// iteration 2; with every iteration completed, or none scheduled, the block is
// left at once.
func TestResumeKinds_BlockEntry_ContinuesWithFirstUncompletedIterationOrLeavesBlock(t *testing.T) {
	aw := mustAdmitScope(t, resumeKindRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	reentered := []string{"0:", "1:1", "2:1", "0:"}

	t.Run("an iteration remains", func(t *testing.T) {
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
		runResumeCases(t, aw, schedules, []resumeCase{
			{"entered again after iteration 1", reentered, false, 1, "Stage", "2", resumeGroupUnchecked},
		})
	})
	t.Run("every iteration completed", func(t *testing.T) {
		schedules := scheduleOf("Stage", idsOf("", "1")...)
		runResumeCases(t, aw, schedules, []resumeCase{
			{"block is left", reentered, false, 3, "", "", resumeGroupUnchecked},
		})
	})
	t.Run("schedule without iterations", func(t *testing.T) {
		runResumeCases(t, aw, scheduleOf("Stage"), []resumeCase{
			{"block is left at once", []string{"0:"}, false, 3, "", "", resumeGroupUnchecked},
		})
	})
}

// COMPLETE on a run-scope row completes the run, although a row follows it in
// the table (it is reachable only through the On Findings of the COMPLETE row).
//
//	0 planner  1 scout (COMPLETE, findings -> tail)  2 tail
func TestResumeKinds_CompleteOnRunScopeRow_ResumesAsRunComplete(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(),
		{phase: "EXECUTION", agent: "scout", onSuccess: "COMPLETE", onFindings: "tail"},
		{phase: "REPORT", agent: "tail"},
	}
	aw := mustAdmitScope(t, rows, "", domain.ExecutionModeAutoReview)

	runResumeCases(t, aw, nil, []resumeCase{
		{"after the COMPLETE row", []string{"0:", "1:"}, false, len(rows), "", "", -1},
	})
}

// In Mode 1 the consultant chooses the route; the informational position is the
// first listed target of a fork row, not the next row of the table.
//
//	0 planner (-> scout)  1 prep  2 scout (fork)  3 writer  4 merge
func TestResumeKinds_Mode1ForkRow_ResumesAtFirstListedTarget(t *testing.T) {
	cases := []struct {
		name    string
		targets string
		wantRow int
	}{
		{"first target is an earlier row", "prep, writer", 1},
		{"first target is the next row", "writer, prep", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := bndAdmitRaw(t, forkTargetInOwnPhaseContent(tc.targets, false), domain.ExecutionModeOrchestrated)
			steps := bndSteps(t, "0:", "2:")

			info, err := resumeLogIDs(aw, nil, bndRunLog(aw, steps), false)

			requireResumeBlock(t, info, err, tc.wantRow, false, "", "")
			requireResumeGroup(t, info, -1)
		})
	}
}

// Regression coverage: the positional resume already answers this way, so this
// protects the rule rather than proving new behavior. A block with no entry in
// the schedules is not an error of the resume: the
// position is the block's first row with no iteration and no group, and the
// stop for the missing schedule comes from the next Next call. Where the
// position leaves the iteration's scope the table's next row is not the answer.
//
//	0 planner  1 worker  2 checker  3 reporter
func TestResumeKinds_Regression_BlockWithoutScheduleEntry_ResumesAtBlockStartWithoutIteration(t *testing.T) {
	aw := mustAdmitScope(t, missingScheduleRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	cases := []struct {
		name  string
		steps []string
	}{
		{"block entered from a run-scope row", []string{"0:"}},
		{"iteration ended inside the block", []string{"0:", "1:1", "2:1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps := bndSteps(t, tc.steps...)

			info, err := resumeLogIDs(aw, domain.BlockSchedules{}, bndRunLog(aw, steps), false)

			requireResumeBlock(t, info, err, 1, false, "Stage", "")
			requireResumeGroup(t, info, -1)
		})
	}
}
