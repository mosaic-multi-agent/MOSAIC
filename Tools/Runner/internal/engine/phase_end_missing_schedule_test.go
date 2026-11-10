package engine_test

// Phase end for a block that has no entry in the schedules (its Source is
// missing, or its schedule was found defective). The answer would need the
// schedule of that block, so no PHASE_END fires: neither after a row of the
// block, nor after a run-scope row whose routed-to position enters the block.
// The answer is the same in Modes 1, 2 and 3. The steps are the ones of a run
// that had the schedule; only the schedules handed to the helper are empty.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// bndEveryMode runs fn once per execution mode, Mode 1 included.
func bndEveryMode(t *testing.T, fn func(t *testing.T, mode domain.ExecutionMode)) {
	t.Helper()
	modes := append(append([]domain.ExecutionMode{}, autoModes...), domain.ExecutionModeOrchestrated)
	for _, mode := range modes {
		t.Run(modeName(mode), func(t *testing.T) { fn(t, mode) })
	}
}

// missingScheduleRows is a planner, a two-row block on another phase, and a
// reporter. With the schedule present the phase ends after 0, 2 (last
// iteration) and 3.
func missingScheduleRows() []scopeRow {
	return []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
}

// The completed row belongs to the block without a schedule: by name checker is
// the last row of BUILD, but the end of the last iteration cannot be known.
func TestPhaseEnd_BlockRowWithoutSchedule_IsNeverPhaseEnd(t *testing.T) {
	bndEveryMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, missingScheduleRows(), stageBlockTables(), mode)
		steps := bndSteps(t, "1:1", "2:1", "1:2", "2:2")

		got := bndPhaseEnds(aw, domain.BlockSchedules{}, mode, steps)

		requireEnds(t, "phase end", got, []string{})
	})
}

// The completed row is outside the block, and the position routed to next
// enters it: planner is the only row of PLANNING, but whether the run leaves
// the phase depends on the block's iterations.
func TestPhaseEnd_RunScopeRowEnteringBlockWithoutSchedule_IsNotPhaseEnd(t *testing.T) {
	bndEveryMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, missingScheduleRows(), stageBlockTables(), mode)
		steps := bndSteps(t, "0:")

		got := bndPhaseEnds(aw, domain.BlockSchedules{}, mode, steps)

		requireEnds(t, "phase end", got, []string{})
	})
}

// Rows that do not need the missing schedule keep their answer: the reporter,
// routed to run completion, still ends its phase.
func TestPhaseEnd_BlockWithoutSchedule_RowAfterTheBlockStillEndsPhase(t *testing.T) {
	bndEveryMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, missingScheduleRows(), stageBlockTables(), mode)
		steps := bndSteps(t, "0:", "1:1", "2:1", "3:")

		got := bndPhaseEnds(aw, domain.BlockSchedules{}, mode, steps)

		requireEnds(t, "phase end", got, []string{"3:"})
	})
}
