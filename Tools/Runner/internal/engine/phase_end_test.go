package engine_test

// Phase end (the answer behind PHASE_END) on WorkflowFormat 2.16 tables in
// Modes 2 and 3. A step that returned SUCCESS is the last of its phase when the
// position the engine routes to next is in another phase, or the run
// completes. For a block row that is the end of the block's last remaining
// iteration followed by a row of another phase; table order and the order of
// the schedule do not decide it.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// A block on a phase other than EXECUTION followed by a run-scope row of the
// same phase: the block's last row of the last iteration is not the phase end,
// the trailing row is.
func TestPhaseEnd_BlockOnOtherPhaseFollowedBySamePhaseRow_OnlyTrailingRowEndsPhase(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(),
		bsBuild("worker", ""),
		bsBuild("checker", ""),
		{phase: "BUILD", agent: "wrap-up"},
		bsReporter(),
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndPhaseEnds(aw, schedules, mode, steps)

		requireEnds(t, "phase end", got, []string{"0:", "3:", "4:"})
	})
}

// Regression coverage: a block alone on a phase other than EXECUTION. Only the
// last row of the last remaining iteration ends the phase; the positional
// answer is the same here, so this is not evidence for the routing rule.
func TestPhaseEnd_Regression_BlockOnOtherPhase_OnlyLastRowOfLastIterationEndsPhase(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndPhaseEnds(aw, schedules, mode, steps)

		requireEnds(t, "phase end", got, []string{"0:", "2:2", "3:"})
	})
}

// Two blocks on one phase: the first block's exit continues in the same phase,
// so only the second block's last row ends it.
func TestPhaseEnd_SiblingBlocksOnOnePhase_FirstBlockExitIsNotPhaseEnd(t *testing.T) {
	rows, tables := mode1SiblingBlockRows()
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, tables, mode)
		schedules := joinSchedules(
			scheduleOf("Stage", idsOf("", "1", "2")...),
			scheduleOf("Audit", idsOf("", "x1", "x2")...))
		steps := bndPlay(t, aw, mode, schedules)

		got := bndPhaseEnds(aw, schedules, mode, steps)

		requireEnds(t, "phase end", got, []string{"0:", "4:x2", "5:"})
	})
}

// With A depending on C the block runs B, C, A. The phase ends after A, the
// last to run, not after C, the last entry of the schedule.
func TestPhaseEnd_ScheduleReorderedByDependsOn_EndsAfterLastIterationToRun(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage", itIteration("A", "C"), itIteration("B"), itIteration("C"))
		steps := bndPlay(t, aw, mode, schedules)

		got := bndPhaseEnds(aw, schedules, mode, steps)

		requireEnds(t, "phase end", got, []string{"0:", "2:A", "3:"})
	})
}

// A group ended by COMPLETE on a row that is not the group's last row: that
// row, in the last remaining iteration, is the phase end.
func TestPhaseEnd_GroupEndedEarlyByComplete_EndsAtThatRow(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, icGroupedCompleteRows(), bsApproachTables(), mode)
		schedules := scheduleOf("Stage", idsOf("TDD", "1", "2")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndPhaseEnds(aw, schedules, mode, steps)

		requireEnds(t, "phase end", got, []string{"0:", "3:2.Implementation", "5:"})
	})
}

// EXECUTION holds a run-scope row before the block and one after it. The row
// before is not the phase end, and neither is the block's last row: the run
// continues at the row after the block, in the same phase.
func TestPhaseEnd_RunScopeRowsBeforeAndAfterBlock_OnlyTrailingRowEndsPhase(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(),
		{phase: "EXECUTION", agent: "prep"},
		{phase: "EXECUTION.[Stage]", agent: "worker"},
		{phase: "EXECUTION.[Stage]", agent: "checker"},
		{phase: "EXECUTION", agent: "wrap-up"},
		bsReporter(),
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
		steps := bndPlay(t, aw, mode, schedules)

		got := bndPhaseEnds(aw, schedules, mode, steps)

		requireEnds(t, "phase end", got, []string{"0:", "4:", "5:"})
	})
}

// A run-scope row whose On Success jumps to a row of another phase ends its
// phase even though a later row carries the same phase name.
//
//	0 planner  1 scout (-> reporter)  2 extra (reached from reporter)  3 reporter (-> extra)
func TestPhaseEnd_RunScopeTargetLeavingPhase_EndsPhaseDespiteLaterRowOfSamePhase(t *testing.T) {
	rows := []scopeRow{
		bsPlanner(),
		{phase: "EXECUTION", agent: "scout", onSuccess: "reporter"},
		{phase: "EXECUTION", agent: "extra", onSuccess: "COMPLETE"},
		{phase: "REPORT", agent: "reporter", onSuccess: "extra"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)
		steps := bndPlay(t, aw, mode, nil)

		got := bndPhaseEnds(aw, nil, mode, steps)

		requireEnds(t, "phase end", got, []string{"0:", "1:", "3:", "2:"})
	})
}

// Regression coverage: a run-scope row routed to a row of its own phase is not
// the phase end; the row after it, routed to another phase, is. The positional
// answer is the same here, so this is not evidence for the routing rule.
func TestPhaseEnd_Regression_RunScopeRowFollowedBySamePhase_IsNotPhaseEnd(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "first"},
		{phase: "PLANNING", agent: "second"},
		{phase: "REPORT", agent: "reporter"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)
		steps := bndPlay(t, aw, mode, nil)

		got := bndPhaseEnds(aw, nil, mode, steps)

		requireEnds(t, "phase end", got, []string{"1:", "2:"})
	})
}
