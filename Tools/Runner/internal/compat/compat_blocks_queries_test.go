package compat_test

// Block query edge cases and block placement at the ends of a 2.16 table:
// GroupOf for an ungrouped block and for rows outside the block, Contains at
// its boundaries, and the single-range pre/post fields when the block starts
// at row 0 or ends the table.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func ungroupedStageTable(t *testing.T) domain.RoutingTable {
	t.Helper()
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	return mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))
}

func TestAdmitBlocks_GroupOf_UngroupedBlockHoldsRowsInTheOneUnnamedGroup(t *testing.T) {
	aw := admitIn(t, ungroupedStageTable(t), domain.ExecutionModeAuto)
	b, ok := aw.BlockByName("Stage")
	if !ok {
		t.Fatal(`BlockByName("Stage"): want the block`)
	}

	for _, row := range []int{1, 2} {
		g, found := b.GroupOf(row)
		if !found || g.Name != "" || g.StartRow != 1 || g.EndRow != 3 {
			t.Errorf("GroupOf(%d): want the unnamed group [1,3), got %+v (found=%v)", row, g, found)
		}
	}
}

func TestAdmitBlocks_GroupOf_RowOutsideTheBlockIsNotFound(t *testing.T) {
	aw := admitIn(t, ungroupedStageTable(t), domain.ExecutionModeAuto)
	b, _ := aw.BlockByName("Stage")

	for _, row := range []int{-1, 0, 3, 4, 99} {
		g, found := b.GroupOf(row)
		if found || g.Name != "" || g.StartRow != 0 || g.EndRow != 0 {
			t.Errorf("GroupOf(%d): want (ExecutionGroup{}, false), got %+v (found=%v)", row, g, found)
		}
	}
}

func TestAdmitBlocks_Contains_BoundariesAreStartInclusiveEndExclusive(t *testing.T) {
	aw := admitIn(t, ungroupedStageTable(t), domain.ExecutionModeAuto)
	b, _ := aw.BlockByName("Stage")

	cases := []struct {
		row  int
		want bool
	}{{0, false}, {1, true}, {2, true}, {3, false}, {-1, false}}
	for _, tc := range cases {
		if got := b.Contains(tc.row); got != tc.want {
			t.Errorf("Contains(%d) with block [%d,%d): want %v, got %v", tc.row, b.StartRow, b.EndRow, tc.want, got)
		}
	}
}

func TestAdmitBlocks_BlockEndsTheTable_PostRangeIsEmpty(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))
	wantGroups := []domain.ExecutionGroup{{Name: "", StartRow: 1, EndRow: 3}}

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages", StartRow: 1, EndRow: 3, Groups: wantGroups,
		})
		requireOneBlockRange(t, aw, 1, 3, 3, wantGroups, false)
	})
}

func TestAdmitBlocks_BlockStartsTheTable_PreRangeIsEmpty(t *testing.T) {
	rows := []wfRow{
		{phase: "EXECUTION.[Stage]", agent: "implementer", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))
	wantGroups := []domain.ExecutionGroup{{Name: "", StartRow: 0, EndRow: 2}}

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages", StartRow: 0, EndRow: 2, Groups: wantGroups,
		})
		requireOneBlockRange(t, aw, 0, 2, 3, wantGroups, false)
	})
}
