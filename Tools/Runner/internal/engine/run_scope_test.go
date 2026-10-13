package engine_test

// Run-scope On Success routing on WorkflowFormat 2.16 tables in Modes 2 and 3:
// a target dispatches exactly the row it names (above or below), NEXT moves to
// the row below, [Name] enters a block, COMPLETE ends the run.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// Regression coverage of unique, unqualified targets; the qualified cases at
// the end of this file are the discriminating ones for row-exact targets.
func TestRunScope_TargetBelow_DispatchesNamedRowAndSkipsRowsBetween(t *testing.T) {
	rows := []scopeRow{
		{phase: "RESEARCH", agent: "scout", onSuccess: "builder"},
		{phase: "RESEARCH", agent: "skipped-row", onFindings: "scout"},
		{phase: "PLANNING", agent: "builder"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		trace := scopeRun{aw: aw, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 2})
	})
}

func TestRunScope_TargetAbove_DispatchesNamedRow(t *testing.T) {
	rows := []scopeRow{
		{phase: "RESEARCH", agent: "scout", onSuccess: "closer"},
		{phase: "RESEARCH", agent: "recorder", onSuccess: "COMPLETE"},
		{phase: "PLANNING", agent: "closer", onSuccess: "recorder"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		trace := scopeRun{aw: aw, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 2, 1})
	})
}

func TestRunScope_Next_DispatchesRowBelow(t *testing.T) {
	rows := []scopeRow{
		{phase: "RESEARCH", agent: "scout"},
		{phase: "PLANNING", agent: "planner"},
		{phase: "REVIEW", agent: "checker"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		trace := scopeRun{aw: aw, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2})
	})
}

func TestRunScope_Next_OnLastRowOfTable_CompletesRun(t *testing.T) {
	rows := []scopeRow{
		{phase: "RESEARCH", agent: "scout"},
		{phase: "PLANNING", agent: "planner", onSuccess: "NEXT"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		trace := scopeRun{aw: aw, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1})
	})
}

// Regression coverage: the block on EXECUTION is the one the 1.x engine knows;
// the sibling-block test below is the discriminating one.
func TestRunScope_Next_IntoRowThatBeginsBlock_EntersBlockAtFirstIteration(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "BUILD.[Stage]", agent: "checker"},
		{phase: "REPORT", agent: "reporter"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2, 3})
		requireStages(t, trace, []string{"", "1", "1", "2", "2", ""})
	})
}

// A plain row sits between two sibling blocks, neither on EXECUTION: NEXT from
// that row enters the second block at its first iteration and first row.
func TestRunScope_Next_FromPlainRowBetweenSiblingBlocks_EntersSecondBlockAtFirstIteration(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "CHECK", agent: "mid"},
		{phase: "VERIFY.[Audit]", agent: "auditor"},
		{phase: "VERIFY.[Audit]", agent: "audit-fixer"},
		{phase: "REPORT", agent: "reporter"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, siblingTables(), mode)
		schedules := siblingSchedules([]domain.IterationID{"1", "2"}, []domain.IterationID{"x1", "x2"})

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 1, 2, 3, 4, 3, 4, 5})
		requireStages(t, trace, []string{"", "1", "2", "", "x1", "x1", "x2", "x2", ""})
	})
}

func TestRunScope_BlockEntry_GroupedBlockStartsAtFirstGroupOfFirstIterationsApproach(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md", onSuccess: "[Stage]"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-checker"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "impl-checker"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, groupedStageTables(), mode)
		schedules := scheduleOf("Stage", domain.ScheduleIteration{ID: "1", Approach: "Implementation-First"},
			domain.ScheduleIteration{ID: "2", Approach: "TDD"})

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		// Iteration 1 starts at the Implementation group (first of its
		// approach), not at the group that comes first in the table.
		requireRowsThenComplete(t, trace, []int{0, 3, 4, 1, 2, 1, 2, 3, 4, 5})
		requireStages(t, trace, []string{
			"", "1.Implementation", "1.Implementation", "1.Test", "1.Test",
			"2.Test", "2.Test", "2.Implementation", "2.Implementation", "",
		})
	})
}

func TestRunScope_BlockEntry_NamedBlockIsEnteredEvenWhenRowsLieBetween(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", input: "Plan.md", output: "Audit.md", onSuccess: "[Audit]"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
		{phase: "REPORT", agent: "reporter"},
	}
	tables := scopeBlocks(stageBlockRow, "| Audit | Audit.md | Items |")
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, tables, mode)
		schedules := joinSchedules(
			scheduleOf("Stage", idsOf("", "1")...),
			scheduleOf("Audit", idsOf("", "x1")...),
		)

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		// The Stage block between the planner and the Audit block never runs.
		requireRowsThenComplete(t, trace, []int{0, 3, 4})
		requireStages(t, trace, []string{"", "x1", ""})
	})
}

func TestRunScope_Complete_EndsRunWithRowsBelow(t *testing.T) {
	rows := []scopeRow{
		{phase: "RESEARCH", agent: "scout", onSuccess: "COMPLETE"},
		{phase: "PLANNING", agent: "planner", onFindings: "scout"},
		{phase: "REVIEW", agent: "checker"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		trace := scopeRun{aw: aw, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0})
	})
}

// The two tests below repeat one bare agent under different qualifiers, so a
// lookup by agent name lands on a different row than the cell the entry names.

func TestRunScope_TargetAbove_DispatchesNamedQualifiedRowNotNearerBelowOfSameAgent(t *testing.T) {
	rows := []scopeRow{
		{phase: "RESEARCH", agent: "starter", onSuccess: "jumper"},
		{phase: "RESEARCH", agent: "x(a)", onSuccess: "COMPLETE"},
		{phase: "PLANNING", agent: "filler"},
		{phase: "PLANNING", agent: "jumper", onSuccess: "x(a)"},
		{phase: "REVIEW", agent: "x(b)", onSuccess: "COMPLETE"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		trace := scopeRun{aw: aw, mode: mode}.play(t)

		// x(b) lies nearer below the jumper and shares the bare agent of
		// x(a); it must not be dispatched.
		requireRowsThenComplete(t, trace, []int{0, 3, 1})
	})
}

func TestRunScope_TargetBelow_DispatchesNamedQualifiedRowNotNearerBelowOfSameAgent(t *testing.T) {
	rows := []scopeRow{
		{phase: "RESEARCH", agent: "jumper", onSuccess: "x(a)"},
		{phase: "RESEARCH", agent: "x(b)", onSuccess: "COMPLETE"},
		{phase: "PLANNING", agent: "filler"},
		{phase: "REVIEW", agent: "x(a)", onSuccess: "COMPLETE"},
	}
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, rows, "", mode)

		trace := scopeRun{aw: aw, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 3})
	})
}
