package engine_test

// Initial dispatch (the first decision of a run) of a table whose row 1 itself
// begins a block, the shape of a workflow that has no rows before its block.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// blockFirstRows: rows 1-2 are the ungrouped block Stage on EXECUTION, row 3
// follows it. The block's source is a seed: row 1 begins the block.
func blockFirstRows() []scopeRow {
	return []scopeRow{
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "impl-checker"},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

// firstDecision calls engine.Next for the first decision of a run.
func firstDecision(aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, mode domain.ExecutionMode) domain.EngineDecision {
	return engine.Next(engine.NextInput{
		Workflow:  aw,
		Schedules: schedules,
		State:     emptyState(),
		Agents:    scopeAgents(aw),
		Now:       fixedNow,
		Mode:      mode,
	})
}

func TestInitialDispatch_RowOneBeginsBlock_DispatchesFirstRowOfFirstIteration(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, blockFirstRows(), stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "alpha", "beta")...)

		step := requireDispatch(t, firstDecision(aw, schedules, mode))

		if step.RowIndex != 0 {
			t.Errorf("RowIndex: want 0, got %d", step.RowIndex)
		}
		if step.Stage != "alpha" {
			t.Errorf("recorded stage value: want the first iteration alpha, got %q", step.Stage)
		}
		if got := agentName(step.Request.AgentInstanceID); got != "implementer" {
			t.Errorf("agent: want implementer, got %s", got)
		}
	})
}

func TestInitialDispatch_RowOneBeginsGroupedBlock_StartsAtFirstGroupOfFirstApproach(t *testing.T) {
	rows := []scopeRow{
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, groupedStageTables(), mode)
		schedules := scheduleOf("Stage", domain.ScheduleIteration{ID: "1", Approach: "Implementation-First"})

		step := requireDispatch(t, firstDecision(aw, schedules, mode))

		if step.RowIndex != 1 {
			t.Errorf("RowIndex: want 1 (the Implementation group comes first), got %d", step.RowIndex)
		}
		if want := domain.FormatIterationValue(domain.IterationValue{ID: "1", Group: "Implementation"}); step.Stage != want {
			t.Errorf("recorded stage value: want %q, got %q", want, step.Stage)
		}
	})
}

func TestInitialDispatch_RowOneBeginsBlock_EmptyScheduleExitsAtOnceAndContinuesAfterBlock(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, blockFirstRows(), stageBlockTables(), mode)

		step := requireDispatch(t, firstDecision(aw, scheduleOf("Stage"), mode))

		if step.RowIndex != 2 {
			t.Errorf("RowIndex: want 2 (the row after the block), got %d", step.RowIndex)
		}
		if step.Stage != "" {
			t.Errorf("a row outside every block records no stage, got %q", step.Stage)
		}
	})
}

// The same entry decisions on a block that is not named EXECUTION.
func TestInitialDispatch_RowOneBeginsBlockOnOtherPhase_EnteredOrExitedByItsSchedule(t *testing.T) {
	rows := []scopeRow{
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "BUILD.[Stage]", agent: "checker"},
		{phase: "REPORT", agent: "reporter"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		entered := requireDispatch(t, firstDecision(aw, scheduleOf("Stage", idsOf("", "alpha")...), mode))
		exited := requireDispatch(t, firstDecision(aw, scheduleOf("Stage"), mode))

		if entered.RowIndex != 0 || entered.Stage != "alpha" {
			t.Errorf("non-empty schedule: want row 0 of iteration alpha, got row %d stage %q", entered.RowIndex, entered.Stage)
		}
		if exited.RowIndex != 2 || exited.Stage != "" {
			t.Errorf("empty schedule: want the row after the block, got row %d stage %q", exited.RowIndex, exited.Stage)
		}
	})
}

func TestInitialDispatch_TableIsOneBlock_EmptyScheduleCompletesRunAtStart(t *testing.T) {
	rows := []scopeRow{
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "impl-checker"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)

		dec := firstDecision(aw, scheduleOf("Stage"), mode)

		requireComplete(t, dec)
	})
}

func TestInitialDispatch_RowOneBeginsBlock_EmptyScheduleEntersFollowingSiblingBlock(t *testing.T) {
	rows := []scopeRow{
		{phase: "EXECUTION.[Stage]", agent: "impl-a", input: "Audit.md"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
		{phase: "REPORT", agent: "reporter"},
	}
	tables := scopeBlocks(stageBlockRow, "| Audit | Audit.md | Items |")
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, tables, mode)
		schedules := joinSchedules(scheduleOf("Stage"), scheduleOf("Audit", idsOf("", "x1")...))

		step := requireDispatch(t, firstDecision(aw, schedules, mode))

		if step.RowIndex != 1 || step.Stage != "x1" {
			t.Errorf("want the auditor row of iteration x1, got row %d stage %q", step.RowIndex, step.Stage)
		}
	})
}
