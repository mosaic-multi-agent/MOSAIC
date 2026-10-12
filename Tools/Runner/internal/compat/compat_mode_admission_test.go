package compat_test

// Mode-dependent admission on WorkflowFormat 2.16 tables.
//
//   - Request-construction checks that 2.16 keeps (group contiguity) refuse in
//     every run mode.
//   - A fork and a join are refused in the modes where the engine routes (auto,
//     auto-review, and the unset sentinel, which behaves as the strictest set)
//     and admitted in orchestrated mode; those tests are in
//     compat_parallel_test.go.
//   - In orchestrated mode a fork inside a group is admitted and the group's
//     row range is unchanged.
//
// The 1.x request-construction checks are in compat_legacy_admission_test.go.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// allModes lists every run mode Admit can be called with, including the
// unset sentinel.
var allModes = []domain.ExecutionMode{
	domain.ExecutionModeOrchestrated,
	domain.ExecutionModeAuto,
	domain.ExecutionModeAutoReview,
	domain.ExecutionModeUnset,
}

// engineRoutedModes are the modes in which the engine routes On Success itself.
var engineRoutedModes = []domain.ExecutionMode{
	domain.ExecutionModeAuto,
	domain.ExecutionModeAutoReview,
	domain.ExecutionModeUnset,
}

func modeName(m domain.ExecutionMode) string {
	if m == domain.ExecutionModeUnset {
		return "unset"
	}
	return string(m)
}

// ---- Request-construction checks refuse in every mode ----

func TestAdmit_RequestConstructionCheck_NonContiguousGroups_RefusedInEveryMode(t *testing.T) {
	// Alpha, Beta, Alpha: row 4 re-opens group Alpha after Beta began.
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Alpha", agent: "agent-a"},
		{phase: "EXECUTION.[Stage].Beta", agent: "agent-b"},
		{phase: "EXECUTION.[Stage].Alpha", agent: "agent-c"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |") + groupsSection("| Stage | Forward | Alpha, Beta |")
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A1"}, row: 4, tokens: []string{"Alpha"}})
}

// ---- Fork and join inside a group, Mode 1 ----

func TestAdmit_ForkAndJoinInsideGroup_OrchestratedMode_KeepsTheGroupRowRange(t *testing.T) {
	// The fork is joined at the last row of its group, so no fork branch ends
	// the group. Rows: 1 planner, 2 t-a (forks to t-b and t-c), 3 t-b, 4 t-c,
	// 5 t-d (join), 6 i-a (Implementation group).
	table := parseRows(t, forkJoinGroupRows(), groupedStageTables())

	aw := admitIn(t, table, domain.ExecutionModeOrchestrated)

	b, ok := aw.BlockByName("Stage")
	if !ok {
		t.Fatal(`BlockByName("Stage"): want found=true`)
	}
	g, ok := groupNamed(b, "Test")
	if !ok {
		t.Fatal(`group "Test": want found=true`)
	}
	if g.StartRow != 1 || g.EndRow != 5 {
		t.Errorf("Test group rows: want [1,5), got [%d,%d)", g.StartRow, g.EndRow)
	}
	if got, ok := b.GroupOf(4); !ok || got.Name != "Test" {
		t.Errorf("GroupOf(4): want the Test group (the join row), got %+v (found=%v)", got, ok)
	}
}
