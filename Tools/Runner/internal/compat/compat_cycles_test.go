package compat_test

// The A15 cycle check of WorkflowFormat 2.16 admission. In Modes 2 and 3 (and
// the unset sentinel) a table whose On Success, NEXT, block entry and block exit
// routes contain a cycle is refused, because the engine would route it forever.
// On Findings routes do not count. Mode 1 is unchanged: the consultant decides
// each route, so the same tables are admitted.

import "testing"

func a15Refusal(rows ...int) wantRoute {
	return wantRoute{rules: []string{"A15"}, rows: rows}
}

type cycleCase struct {
	name   string
	rows   []rrow
	tables string
	want   wantRoute
}

func cyclicCases() []cycleCase {
	return []cycleCase{
		{
			name: "run-scope target above that leads back",
			rows: []rrow{
				{phase: "RESEARCH", agent: "agent-a"},
				{phase: "PLANNING", agent: "agent-b"},
				{phase: "PLANNING", agent: "agent-c", onSuccess: "agent-a"},
			},
			want: a15Refusal(1, 2, 3),
		},
		{
			name: "row targeting itself",
			rows: []rrow{
				{phase: "RESEARCH", agent: "agent-a"},
				{phase: "PLANNING", agent: "agent-b", onSuccess: "agent-b"},
			},
			want: a15Refusal(2),
		},
		{
			name: "cycle inside one group of a block",
			rows: []rrow{
				{phase: "PLANNING", agent: "planner", output: "Plan.md"},
				{phase: "EXECUTION.[Stage].Test", agent: "t-a"},
				{phase: "EXECUTION.[Stage].Test", agent: "t-b", onSuccess: "t-a"},
				{phase: "EXECUTION.[Stage].Implementation", agent: "i-a"},
				{phase: "REVIEW", agent: "test-runner"},
			},
			tables: groupedStageTables(),
			want:   a15Refusal(2, 3),
		},
		{
			name: "cycle inside an ungrouped block",
			rows: []rrow{
				{phase: "PLANNING", agent: "planner", output: "Plan.md"},
				{phase: "EXECUTION.[Stage]", agent: "impl-a"},
				{phase: "EXECUTION.[Stage]", agent: "impl-b", onSuccess: "impl-a"},
				{phase: "REVIEW", agent: "test-runner"},
			},
			tables: stageBlockTable(),
			want:   a15Refusal(2, 3),
		},
		{
			name: "cycle through a block counted as one step",
			rows: []rrow{
				{phase: "PLANNING", agent: "planner", output: "Plan.md"},
				{phase: "EXECUTION.[Stage]", agent: "impl-a"},
				{phase: "EXECUTION.[Stage]", agent: "impl-b"},
				{phase: "REVIEW", agent: "test-runner", onSuccess: "planner"},
			},
			tables: stageBlockTable(),
			want:   a15Refusal(1, 2, 3, 4),
		},
		{
			// The block exits to row 4 below it, and row 4 enters the block again.
			name: "cycle through a backward block entry",
			rows: []rrow{
				{phase: "PLANNING", agent: "planner", output: "Plan.md"},
				{phase: "EXECUTION.[Stage]", agent: "impl-a"},
				{phase: "EXECUTION.[Stage]", agent: "impl-b"},
				{phase: "REVIEW", agent: "test-runner", onSuccess: "[Stage]"},
			},
			tables: stageBlockTable(),
			want:   a15Refusal(2, 3, 4),
		},
	}
}

func TestAdmitCycles_OnSuccessCycle_RefusedInEngineRoutedModes(t *testing.T) {
	for _, tc := range cyclicCases() {
		t.Run(tc.name, func(t *testing.T) {
			table := parseRows(t, tc.rows, tc.tables)

			requireRefusedIn(t, table, engineRoutedModes, tc.want)
		})
	}
}

func TestAdmitCycles_OnSuccessCycle_AdmittedUnchangedInOrchestratedMode(t *testing.T) {
	for _, tc := range cyclicCases() {
		t.Run(tc.name, func(t *testing.T) {
			table := parseRows(t, tc.rows, tc.tables)

			requireAdmittedIn(t, table, orchestratedOnly)
		})
	}
}

func TestAdmitCycles_ForwardJumpWithoutCycle_AdmittedInEveryMode(t *testing.T) {
	// Row 1 jumps to row 4; rows 2 and 3 use NEXT and lead on to row 4.
	rows := []rrow{
		{phase: "RESEARCH", agent: "agent-a", onSuccess: "agent-d"},
		{phase: "RESEARCH", agent: "agent-b"},
		{phase: "PLANNING", agent: "agent-c"},
		{phase: "PLANNING", agent: "agent-d"},
		{phase: "REVIEW", agent: "agent-e"},
	}
	table := parseRows(t, rows, "")

	requireAdmittedIn(t, table, allModes)
}

func TestAdmitCycles_TargetAboveThatLeavesThroughComplete_AdmittedInEveryMode(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "agent-a", onSuccess: "agent-b"},
		{phase: "PLANNING", agent: "agent-b", onSuccess: "COMPLETE"},
		{phase: "PLANNING", agent: "agent-c", onSuccess: "agent-a"},
	}
	table := parseRows(t, rows, "")

	requireAdmittedIn(t, table, allModes)
}

func TestAdmitCycles_OnFindingsRouteBackAbove_AdmittedInEveryMode(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "agent-a"},
		{phase: "RESEARCH", agent: "agent-b", onFindings: "agent-a"},
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b", onFindings: "impl-a"},
		{phase: "REVIEW", agent: "test-runner", onFindings: "planner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	requireAdmittedIn(t, table, allModes)
}

func TestAdmitCycles_OnFindingsRouteBackInsideGroup_AdmittedInEveryMode(t *testing.T) {
	rows := groupedStageRows()
	rows[2].onFindings = "t-a"
	rows[4].onFindings = "i-a"
	table := parseRows(t, rows, groupedStageTables())

	requireAdmittedIn(t, table, allModes)
}
