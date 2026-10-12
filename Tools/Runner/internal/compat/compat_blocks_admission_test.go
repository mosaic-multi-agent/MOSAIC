package compat_test

// Admission of the WorkflowFormat 2.16 block structure: blocks on any phase,
// grouped and ungrouped blocks, sibling blocks, mode-qualified Subagent cells
// and deployed tables with a Row column, in every run mode. Each test asserts
// the per-block admitted model.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// brownfield40Rows is the row shape of a grouped TDD build workflow: two
// pre-block rows, a Stage block with groups Test and Implementation (each
// repeating build-review through a mode qualifier), one post-block row.
func brownfield40Rows() []wfRow {
	return []wfRow{
		{phase: "RESEARCH", agent: "codebase-research"},
		{phase: "PLANNING", agent: "planner-tdd-soft", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer-tdd"},
		{phase: "EXECUTION.[Stage].Test", agent: "build-review(tests)"},
		{phase: "EXECUTION.[Stage].Test", agent: "tests-review-tdd"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementation-tdd"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "build-review(implementation)"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementation-review"},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

func brownfield40Tables() string {
	return blocksSection("| Stage | Plan.md | Stages |") + groupsSection(
		"| Stage | TDD | Test, Implementation |",
		"| Stage | Implementation-First | Implementation, Test |",
		"| Stage | Implementation-Only | Implementation |",
		"| Stage | Tests-Only | Test |",
	)
}

func brownfield40Approaches() []domain.ApproachSequence {
	return []domain.ApproachSequence{
		{Block: "Stage", Approach: "TDD", Groups: []domain.GroupName{"Test", "Implementation"}},
		{Block: "Stage", Approach: "Implementation-First", Groups: []domain.GroupName{"Implementation", "Test"}},
		{Block: "Stage", Approach: "Implementation-Only", Groups: []domain.GroupName{"Implementation"}},
		{Block: "Stage", Approach: "Tests-Only", Groups: []domain.GroupName{"Test"}},
	}
}

func brownfield40Block() domain.AdmittedBlock {
	return domain.AdmittedBlock{
		Name: "Stage", Source: "Plan.md", Section: "Stages",
		StartRow: 2, EndRow: 8, Grouped: true,
		Groups: []domain.ExecutionGroup{
			{Name: "Test", StartRow: 2, EndRow: 5},
			{Name: "Implementation", StartRow: 5, EndRow: 8},
		},
		Approaches: domain.ApproachTable{Rows: brownfield40Approaches()},
	}
}

func TestAdmitBlocks_GroupedBrownfieldShape_ModelInEveryMode(t *testing.T) {
	table := mustParse216(t, region(brownfield40Rows(), false, brownfield40Tables()))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, brownfield40Block())
		requireOneBlockRange(t, aw, 2, 8, 9, brownfield40Block().Groups, true)
		if got := aw.Table.ApproachTable.Rows; !reflect.DeepEqual(got, brownfield40Approaches()) {
			t.Errorf("ApproachTable.Rows: want the table's four approach rows, got %+v", got)
		}
	})
}

func TestAdmitBlocks_GroupedBlock_IsNeverAdmittedAsBare(t *testing.T) {
	table := mustParse216(t, region(brownfield40Rows(), false, brownfield40Tables()))

	aw := admitIn(t, table, domain.ExecutionModeAuto)

	if len(aw.Blocks) != 1 {
		t.Fatalf("want one block, got %d", len(aw.Blocks))
	}
	b := aw.Blocks[0]
	if !b.Grouped {
		t.Error("a block whose rows carry groups must be admitted as grouped")
	}
	for _, g := range b.Groups {
		if g.Name == "" {
			t.Errorf("a grouped block must hold only named groups, got %+v", b.Groups)
		}
	}
	if len(b.Groups) != 2 {
		t.Errorf("want groups Test and Implementation, got %+v", b.Groups)
	}
}

func TestAdmitBlocks_BlockQueries_FindBlockGroupAndRows(t *testing.T) {
	table := mustParse216(t, region(brownfield40Rows(), false, brownfield40Tables()))
	aw := admitIn(t, table, domain.ExecutionModeAuto)

	cases := []struct {
		row       int
		inBlock   bool
		wantGroup domain.GroupName
	}{
		{0, false, ""}, {1, false, ""}, {2, true, "Test"}, {4, true, "Test"},
		{5, true, "Implementation"}, {7, true, "Implementation"}, {8, false, ""}, {-1, false, ""}, {99, false, ""},
	}
	for _, tc := range cases {
		b, ok := aw.BlockOf(tc.row)
		if ok != tc.inBlock {
			t.Errorf("BlockOf(%d): want found=%v, got %v", tc.row, tc.inBlock, ok)
			continue
		}
		if !ok {
			continue
		}
		if b.Name != "Stage" || !b.Contains(tc.row) {
			t.Errorf("BlockOf(%d): want block Stage containing the row, got %+v", tc.row, b)
		}
		if g, found := b.GroupOf(tc.row); !found || g.Name != tc.wantGroup {
			t.Errorf("GroupOf(%d): want group %q, got %+v (found=%v)", tc.row, tc.wantGroup, g, found)
		}
	}
	if b, ok := aw.BlockByName("Stage"); !ok || b.Name != "Stage" {
		t.Errorf(`BlockByName("Stage"): want the block, got %+v (found=%v)`, b, ok)
	}
	if _, ok := aw.BlockByName("stage"); ok {
		t.Error(`BlockByName("stage"): block names are case-sensitive, want not found`)
	}
}

func TestAdmitBlocks_UngroupedBlock_ModelInEveryMode(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))
	wantGroups := []domain.ExecutionGroup{{Name: "", StartRow: 1, EndRow: 3}}

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages",
			StartRow: 1, EndRow: 3, Grouped: false, Groups: wantGroups,
		})
		requireOneBlockRange(t, aw, 1, 3, 4, wantGroups, false)
	})
}

func TestAdmitBlocks_SiblingBlocks_GroupedAndUngroupedInEveryMode(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "INTEGRATION", agent: "integrator"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
		{phase: "REVIEW.[Audit]", agent: "audit-fixer"},
		{phase: "REPORT", agent: "reporter"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |") +
		groupsSection("| Stage | TDD | Test, Implementation |")
	table := mustParse216(t, region(rows, false, tables))
	wantStage := domain.AdmittedBlock{
		Name: "Stage", Source: "Plan.md", Section: "Stages",
		StartRow: 1, EndRow: 3, Grouped: true,
		Groups: []domain.ExecutionGroup{
			{Name: "Test", StartRow: 1, EndRow: 2},
			{Name: "Implementation", StartRow: 2, EndRow: 3},
		},
		Approaches: domain.ApproachTable{Rows: []domain.ApproachSequence{
			{Block: "Stage", Approach: "TDD", Groups: []domain.GroupName{"Test", "Implementation"}},
		}},
	}
	wantAudit := domain.AdmittedBlock{
		Name: "Audit", Source: "Audit.md", Section: "Items",
		StartRow: 4, EndRow: 6, Grouped: false,
		Groups: []domain.ExecutionGroup{{Name: "", StartRow: 4, EndRow: 6}},
	}

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, wantStage, wantAudit)
	})
}

func TestAdmitBlocks_SiblingGroupedBlocks_EachKeepsItsOwnApproachRows(t *testing.T) {
	// Both blocks use the approach token Full and the group name Test: groups
	// and approach rows are scoped per block.
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "stage-test"},
		{phase: "EXECUTION.[Stage].Impl", agent: "stage-impl"},
		{phase: "REVIEW.[Audit].Scan", agent: "audit-scan"},
		{phase: "REVIEW.[Audit].Test", agent: "audit-test"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |") + groupsSection(
		"| Stage | Full | Test, Impl |",
		"| Audit | Full | Scan, Test |",
	)
	table := mustParse216(t, region(rows, false, tables))

	aw := admitIn(t, table, domain.ExecutionModeAuto)

	requireBlocks(t, aw,
		domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages", StartRow: 1, EndRow: 3, Grouped: true,
			Groups: []domain.ExecutionGroup{{Name: "Test", StartRow: 1, EndRow: 2}, {Name: "Impl", StartRow: 2, EndRow: 3}},
			Approaches: domain.ApproachTable{Rows: []domain.ApproachSequence{
				{Block: "Stage", Approach: "Full", Groups: []domain.GroupName{"Test", "Impl"}},
			}},
		},
		domain.AdmittedBlock{
			Name: "Audit", Source: "Audit.md", Section: "Items", StartRow: 3, EndRow: 5, Grouped: true,
			Groups: []domain.ExecutionGroup{{Name: "Scan", StartRow: 3, EndRow: 4}, {Name: "Test", StartRow: 4, EndRow: 5}},
			Approaches: domain.ApproachTable{Rows: []domain.ApproachSequence{
				{Block: "Audit", Approach: "Full", Groups: []domain.GroupName{"Scan", "Test"}},
			}},
		},
	)
}

func TestAdmitBlocks_BlockOnAnyPhaseName_AdmittedInEveryMode(t *testing.T) {
	for _, phase := range []string{"EXECUTION", "REVIEW", "PLANNING", "RESEARCH", "DESIGN", "PHASE_2"} {
		t.Run(phase, func(t *testing.T) {
			rows := []wfRow{
				{phase: "INTAKE", agent: "intake", output: "Items.md"},
				{phase: phase + ".[Audit]", agent: "auditor"},
				{phase: phase + ".[Audit]", agent: "audit-fixer"},
				{phase: "REPORT", agent: "reporter"},
			}
			table := mustParse216(t, region(rows, false, blocksSection("| Audit | Items.md | Items |")))
			wantGroups := []domain.ExecutionGroup{{Name: "", StartRow: 1, EndRow: 3}}

			forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
				aw := admitIn(t, table, mode)

				requireBlocks(t, aw, domain.AdmittedBlock{
					Name: "Audit", Source: "Items.md", Section: "Items",
					StartRow: 1, EndRow: 3, Groups: wantGroups,
				})
				requireOneBlockRange(t, aw, 1, 3, 4, wantGroups, false)
			})
		})
	}
}

func TestAdmitBlocks_SameGroupNameInTwoBlocks_Admitted(t *testing.T) {
	// A1 is scoped to one block: a group name used again by another block is
	// not a re-opened group.
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "stage-test"},
		{phase: "EXECUTION.[Stage].Impl", agent: "stage-impl"},
		{phase: "REVIEW.[Audit].Test", agent: "audit-test"},
		{phase: "REVIEW.[Audit].Impl", agent: "audit-impl"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |") + groupsSection(
		"| Stage | Full | Test, Impl |",
		"| Audit | Full | Test, Impl |",
	)

	aw := admitIn(t, mustParse216(t, region(rows, false, tables)), domain.ExecutionModeAuto)

	requireBlocks(t, aw,
		domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages", StartRow: 1, EndRow: 3, Grouped: true,
			Groups: []domain.ExecutionGroup{{Name: "Test", StartRow: 1, EndRow: 2}, {Name: "Impl", StartRow: 2, EndRow: 3}},
			Approaches: domain.ApproachTable{Rows: []domain.ApproachSequence{
				{Block: "Stage", Approach: "Full", Groups: []domain.GroupName{"Test", "Impl"}},
			}},
		},
		domain.AdmittedBlock{
			Name: "Audit", Source: "Audit.md", Section: "Items", StartRow: 3, EndRow: 5, Grouped: true,
			Groups: []domain.ExecutionGroup{{Name: "Test", StartRow: 3, EndRow: 4}, {Name: "Impl", StartRow: 4, EndRow: 5}},
			Approaches: domain.ApproachTable{Rows: []domain.ApproachSequence{
				{Block: "Audit", Approach: "Full", Groups: []domain.GroupName{"Test", "Impl"}},
			}},
		},
	)
}

func TestAdmitBlocks_RepeatedAgentThroughModeQualifiers_AdmittedInEveryMode(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD", agent: "build-review(tests)"},
		{phase: "EXECUTION.[Stage]", agent: "build-review(implementation)"},
		{phase: "EXECUTION.[Stage]", agent: "build-review(fix)"},
		{phase: "REVIEW", agent: "build-review(final)"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireBlocks(t, aw, domain.AdmittedBlock{
			Name: "Stage", Source: "Plan.md", Section: "Stages", StartRow: 2, EndRow: 4,
			Groups: []domain.ExecutionGroup{{Name: "", StartRow: 2, EndRow: 4}},
		})
		for i, r := range aw.Table.Rows {
			wantBare := "build-review"
			if !strings.HasPrefix(rows[i].agent, "build-review(") {
				wantBare = rows[i].agent
			}
			if r.BareAgentID() != wantBare {
				t.Errorf("Rows[%d].BareAgentID(): want %q, got %q", i, wantBare, r.BareAgentID())
			}
			if r.Agent != rows[i].agent {
				t.Errorf("Rows[%d].Agent: want the whole cell %q, got %q", i, rows[i].agent, r.Agent)
			}
		}
	})
}

func TestAdmitBlocks_DeployedRowColumn_GivesTheSameModel(t *testing.T) {
	withRow := mustParse216(t, region(brownfield40Rows(), true, brownfield40Tables()))
	without := mustParse216(t, region(brownfield40Rows(), false, brownfield40Tables()))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		got := admitIn(t, withRow, mode)

		requireBlocks(t, got, brownfield40Block())
		want := admitIn(t, without, mode)
		if !reflect.DeepEqual(got.Blocks, want.Blocks) {
			t.Errorf("a deployed Row column must not change the model:\nwith:    %+v\nwithout: %+v", got.Blocks, want.Blocks)
		}
	})
}

func TestAdmitBlocks_TableWithoutBlocks_AdmittedWithNoBlockInEveryMode(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner"},
		{phase: "REVIEW", agent: "reviewer"},
	}
	table := mustParse216(t, region(rows, false))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		if len(aw.Blocks) != 0 {
			t.Errorf("Blocks: want none for a table with no block, got %+v", aw.Blocks)
		}
		if _, ok := aw.BlockOf(0); ok {
			t.Error("BlockOf(0): a run-scope row is in no block")
		}
	})
}
