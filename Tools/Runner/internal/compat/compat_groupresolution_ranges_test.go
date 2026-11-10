package compat_test

// Tests for the rows before and after the block, three or more groups, generic
// agent and group names, and ungrouped blocks with any agent mix, on
// WorkflowFormat 2.16 fixtures.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// ---- Rows before and after the block ----

func TestAdmit_CatalogShapes_RowsBeforeAndAfterTheBlock(t *testing.T) {
	cases := []struct {
		name                         string
		content                      string
		pre, postStart, postEnd      int
		wantBlockStart, wantBlockEnd int
	}{
		{"brownfield-tdd-build-verified", brownfieldBuildVerifiedContent, 7, 13, 13, 7, 13},
		{"brownfield-tdd", brownfieldTDDContent, 7, 11, 12, 7, 11},
		{"greenfield-tdd", greenFieldTDDContent, 8, 12, 13, 8, 12},
		{"quick-fix", quickFixContent, 2, 3, 4, 2, 3},
		{"implementation-only", implOnlyContent, 0, 2, 3, 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := mustAdmit(t, mustParseShape(t, tc.content, tc.name, "4.0"))

			b := stageBlock(t, aw)
			if b.StartRow != tc.wantBlockStart || b.EndRow != tc.wantBlockEnd {
				t.Errorf("block rows: want [%d,%d), got [%d,%d)", tc.wantBlockStart, tc.wantBlockEnd, b.StartRow, b.EndRow)
			}
			requireOneBlockRange(t, aw, tc.pre, tc.postStart, tc.postEnd, b.Groups, b.Grouped)
		})
	}
}

// ---- Three or more groups ----

func TestAdmit_ThreeGroups_AreAdmittedInEveryModeWithTheirRanges(t *testing.T) {
	table := mustParseShape(t, threeGroupsContent, "three-groups", "4.0")
	want := []domain.ExecutionGroup{
		{Name: "Alpha", StartRow: 1, EndRow: 2},
		{Name: "Beta", StartRow: 2, EndRow: 3},
		{Name: "Gamma", StartRow: 3, EndRow: 4},
	}

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages",
			StartRow: 1, EndRow: 4, Grouped: true, Groups: want,
			Approaches: domain.ApproachTable{Rows: []domain.ApproachSequence{
				{Block: "Stage", Approach: "AllGroups", Groups: []domain.GroupName{"Alpha", "Beta", "Gamma"}},
			}},
		})
		if !aw.Blocks[0].Grouped {
			t.Error("Grouped: want true for a three-group workflow")
		}
	})
}

// ---- Arbitrary group names and generic agent identifiers ----

func TestAdmit_GenericAgentsWithGroupNotation_GroupsComeFromThePhaseColumn(t *testing.T) {
	table := mustParseShape(t, genericAgentGroupsContent, "generic-agent-groups", "4.0")

	aw := admitIn(t, table, domain.ExecutionModeAuto)

	want := []domain.ExecutionGroup{
		{Name: "Analysis", StartRow: 1, EndRow: 3},
		{Name: "Synthesis", StartRow: 3, EndRow: 4},
	}
	requireBlocks(t, aw, domain.AdmittedBlock{
		Name: "Stage", Source: "Plan.md", Section: "Stages",
		StartRow: 1, EndRow: 4, Grouped: true, Groups: want,
		Approaches: domain.ApproachTable{Rows: []domain.ApproachSequence{
			{Block: "Stage", Approach: "Forward", Groups: []domain.GroupName{"Analysis", "Synthesis"}},
		}},
	})
}

// ---- Ungrouped blocks with any agent mix ----

func TestAdmit_UngroupedBlockWithMixedAgents_IsOneImplicitGroup(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "test-writer-tdd"},
		{phase: "EXECUTION.[Stage]", agent: "unknown-agent"},
		{phase: "EXECUTION.[Stage]", agent: "implementation-tdd"},
		{phase: "EXECUTION.[Stage]", agent: "unknown-agent-2"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))

	aw := admitIn(t, table, domain.ExecutionModeAuto)

	wantGroups := []domain.ExecutionGroup{{Name: "", StartRow: 1, EndRow: 5}}
	requireBlocks(t, aw, domain.AdmittedBlock{
		Name: "Stage", Source: "Plan.md", Section: "Stages", StartRow: 1, EndRow: 5, Groups: wantGroups,
	})
	if aw.Blocks[0].Grouped {
		t.Error("Grouped: want false for an ungrouped block")
	}
}

func TestAdmit_UngroupedBlockWithAlternatingAgents_IsAdmittedThroughModeQualifiers(t *testing.T) {
	// The same two agents alternate; each repeat carries a mode qualifier so
	// every Subagent cell is distinct.
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "test-writer-tdd"},
		{phase: "EXECUTION.[Stage]", agent: "implementation-tdd"},
		{phase: "EXECUTION.[Stage]", agent: "test-writer-tdd(fix)"},
		{phase: "EXECUTION.[Stage]", agent: "implementation-tdd(fix)"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages", StartRow: 1, EndRow: 5,
			Groups: []domain.ExecutionGroup{{Name: "", StartRow: 1, EndRow: 5}},
		})
	})
}
