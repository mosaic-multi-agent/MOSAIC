package compat_test

// Scope checks of WorkflowFormat 2.16 admission: a [Name] entry in On Success
// names a declared block that is a child of the row's scope, and every target in
// On Success, On Findings and Waits For names a row of the row's own scope (the
// run, or one group or ungrouped iteration of a block).

import (
	"testing"

	"mosaic-run/internal/domain"
)

type scopeRefusalCase struct {
	name   string
	rows   []rrow
	tables string
	modes  []domain.ExecutionMode
	want   wantRoute
}

func scopeRefusalCases() []scopeRefusalCase {
	a12 := func(row int, token string) wantRoute {
		return wantRoute{rules: []string{"A12"}, rows: []int{row}, tokens: []string{token}}
	}
	return []scopeRefusalCase{
		{
			name: "block entry naming an undeclared block",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[0].onSuccess = "[Nope]"
				return r
			}(),
			tables: stageBlockTable(), modes: allModes,
			// An undeclared name is a table no parse could produce: the shape check
			// refuses it before any A-rule, naming only the row (as the block
			// admission refusal tests require).
			want: wantRoute{rules: []string{""}, rows: []int{1}, noToken: true},
		},
		{
			name: "block entry from a block row naming a sibling block",
			rows: func() []rrow {
				r := twoBlockRows()
				r[1].onSuccess = "[Audit]"
				return r
			}(),
			tables: twoBlockTables(), modes: allModes,
			want: wantRoute{rules: []string{"A11"}, rows: []int{2}, tokens: []string{"Audit"}},
		},
		{
			name: "run-scope row targeting the first row of a block",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[0].onSuccess = "impl-a"
				return r
			}(),
			tables: stageBlockTable(), modes: allModes, want: a12(1, "impl-a"),
		},
		{
			name: "run-scope row targeting a later row of a block",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[0].onSuccess = "impl-b"
				return r
			}(),
			tables: stageBlockTable(), modes: allModes, want: a12(1, "impl-b"),
		},
		{
			name: "block row targeting a run-scope row",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[2].onSuccess = "test-runner"
				return r
			}(),
			tables: stageBlockTable(), modes: allModes, want: a12(3, "test-runner"),
		},
		{
			name: "target from one group into another",
			rows: func() []rrow {
				r := groupedStageRows()
				r[1].onSuccess = "i-a"
				return r
			}(),
			tables: groupedStageTables(), modes: allModes, want: a12(2, "i-a"),
		},
		{
			name: "On Findings target from one group into another",
			rows: func() []rrow {
				r := groupedStageRows()
				r[3].onFindings = "t-a"
				return r
			}(),
			tables: groupedStageTables(), modes: allModes, want: a12(4, "t-a"),
		},
		{
			name: "On Findings target from a row after a block into a block row",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[3].onFindings = "impl-a"
				return r
			}(),
			tables: stageBlockTable(), modes: allModes, want: a12(4, "impl-a"),
		},
		{
			name: "On Findings target from a block row to a run-scope row",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[2].onFindings = "planner"
				return r
			}(),
			tables: stageBlockTable(), modes: allModes, want: a12(3, "planner"),
		},
		{
			name: "Waits For from a run-scope row into a block row",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[3].waitsFor = "impl-a"
				return r
			}(),
			tables: stageBlockTable(), modes: orchestratedOnly, want: a12(4, "impl-a"),
		},
		{
			// Fork entries are targets too; in Mode 1 the fork itself stays admitted, so
			// the refusal is the scope check. The entries sit in another phase than the
			// fork row, so they are no fork branches of the boundary check.
			name: "fork entries from a run-scope row naming block rows",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[0].onSuccess = "impl-a, impl-b"
				return r
			}(),
			tables: stageBlockTable(), modes: orchestratedOnly,
			want: wantRoute{rules: []string{"A12"}, rows: []int{1}, tokens: []string{"impl-a", "impl-b"}},
		},
		{
			name: "Waits For from a block row to a run-scope row",
			rows: func() []rrow {
				r := ungroupedStageRows()
				r[2].waitsFor = "planner"
				return r
			}(),
			tables: stageBlockTable(), modes: orchestratedOnly, want: a12(3, "planner"),
		},
	}
}

func TestAdmitScopes_OutOfScopeReferences_AreRefusedWithRuleRowAndToken(t *testing.T) {
	for _, tc := range scopeRefusalCases() {
		t.Run(tc.name, func(t *testing.T) {
			table := parseRows(t, tc.rows, tc.tables)

			requireRefusedIn(t, table, tc.modes, tc.want)
		})
	}
}

func TestAdmitScopes_InScopeTargetsAboveAndBelow_AreAdmitted(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "r-a", onSuccess: "r-b"},                 // target below
		{phase: "RESEARCH", agent: "r-b", onFindings: "r-a"},                // On Findings above
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},            // NEXT enters the block
		{phase: "EXECUTION.[Stage]", agent: "impl-a", onSuccess: "impl-b"},  // target below, in block
		{phase: "EXECUTION.[Stage]", agent: "impl-b", onFindings: "impl-a"}, // On Findings above, in block
		{phase: "REVIEW", agent: "test-runner", onFindings: "planner"},      // On Findings above, run scope
	}
	table := parseRows(t, rows, stageBlockTable())

	aw := requireAdmittedIn(t, table, allModes)

	if len(aw.Blocks) != 1 || aw.Blocks[0].Name != "Stage" {
		t.Errorf("want the one block Stage in the admitted model, got %+v", aw.Blocks)
	}
}

func TestAdmitScopes_BlockEntryFromRunScopeRow_IsAdmitted(t *testing.T) {
	rows := ungroupedStageRows()
	rows[0].onSuccess = "[Stage]"
	table := parseRows(t, rows, stageBlockTable())

	aw := requireAdmittedIn(t, table, allModes)

	if len(aw.Blocks) != 1 {
		t.Errorf("want one block in the admitted model, got %d", len(aw.Blocks))
	}
}

func TestAdmitScopes_TargetsWithinOneGroupAboveAndBelow_AreAdmitted(t *testing.T) {
	rows := groupedStageRows()
	rows[1].onSuccess = "t-b"
	rows[2].onFindings = "t-a"
	rows[4].onFindings = "i-a"
	table := parseRows(t, rows, groupedStageTables())

	requireAdmittedIn(t, table, allModes)
}
