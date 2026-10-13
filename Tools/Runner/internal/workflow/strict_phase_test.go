package workflow_test

// Strict Phase cell parsing (WorkflowFormat 2.16 sec. 6.2): plain phases,
// block references on any phase name, grouped rows and nested rows.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestParseStrict_PhaseCell_RowsCarryBlocksGroupAndName(t *testing.T) {
	cases := []struct {
		phase      string
		wantName   string
		wantBlocks []string
		wantGroup  domain.GroupName
	}{
		{"PLANNING", "PLANNING", nil, ""},
		{"EXECUTION.[Stage]", "EXECUTION", []string{"Stage"}, ""},
		{"REVIEW.[Correction]", "REVIEW", []string{"Correction"}, ""},
		{"RESEARCH.[Product]", "RESEARCH", []string{"Product"}, ""},
		{"EXECUTION.[Stage].Test", "EXECUTION", []string{"Stage"}, "Test"},
		{"EXECUTION.[Stage].Implementation", "EXECUTION", []string{"Stage"}, "Implementation"},
		{"EXECUTION.[Stage].2nd-pass", "EXECUTION", []string{"Stage"}, "2nd-pass"},
		{"DESIGN.[Domain].[Stage]", "DESIGN", []string{"Domain", "Stage"}, ""},
		{"EXECUTION.[Domain].[Stage].Test", "EXECUTION", []string{"Domain", "Stage"}, "Test"},
		{"PHASE_2.[Stage]", "PHASE_2", []string{"Stage"}, ""},
		{"A", "A", nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.phase, func(t *testing.T) {
			// Act
			table := mustParseStrict(t, strictRegion(strictPhases(tc.phase)))

			// Assert
			if len(table.Rows) != 1 {
				t.Fatalf("want 1 row, got %d", len(table.Rows))
			}
			row := table.Rows[0]
			if row.Phase != tc.phase {
				t.Errorf("Phase: want literal %q, got %q", tc.phase, row.Phase)
			}
			if row.PhaseParsed.Name != tc.wantName {
				t.Errorf("PhaseParsed.Name: want %q, got %q", tc.wantName, row.PhaseParsed.Name)
			}
			if got := blockNames(row.PhaseParsed.Blocks); !equalStrings(got, tc.wantBlocks) {
				t.Errorf("PhaseParsed.Blocks: want %v outermost first, got %v", tc.wantBlocks, got)
			}
			if row.PhaseParsed.Group != tc.wantGroup {
				t.Errorf("PhaseParsed.Group: want %q, got %q", tc.wantGroup, row.PhaseParsed.Group)
			}
			if wantStaged := len(tc.wantBlocks) > 0; row.PhaseParsed.IsStaged != wantStaged {
				t.Errorf("PhaseParsed.IsStaged: want %v (row in a block), got %v", wantStaged, row.PhaseParsed.IsStaged)
			}
		})
	}
}

func TestParseStrict_MixedPhases_EachRowKeepsItsOwnBlocksAndGroup(t *testing.T) {
	// Arrange: rows outside, inside, grouped and nested, in one table.
	rows := strictPhases(
		"PLANNING",
		"EXECUTION.[Stage].Test",
		"EXECUTION.[Stage].Implementation",
		"REVIEW",
		"REVIEW.[Correction]",
	)

	// Act
	table := mustParseStrict(t, strictRegion(rows))

	// Assert
	want := []struct {
		blocks []string
		group  domain.GroupName
	}{
		{nil, ""},
		{[]string{"Stage"}, "Test"},
		{[]string{"Stage"}, "Implementation"},
		{nil, ""},
		{[]string{"Correction"}, ""},
	}
	if len(table.Rows) != len(want) {
		t.Fatalf("want %d rows, got %d", len(want), len(table.Rows))
	}
	for i, w := range want {
		row := table.Rows[i]
		if row.Index != i {
			t.Errorf("Rows[%d].Index: want %d, got %d", i, i, row.Index)
		}
		if got := blockNames(row.PhaseParsed.Blocks); !equalStrings(got, w.blocks) {
			t.Errorf("Rows[%d] blocks: want %v, got %v", i, w.blocks, got)
		}
		if row.PhaseParsed.Group != w.group {
			t.Errorf("Rows[%d] group: want %q, got %q", i, w.group, row.PhaseParsed.Group)
		}
	}
}

func TestParseStrict_ValidTable_CarriesTheWorkflowInfo(t *testing.T) {
	table := mustParseStrict(t, strictRegion(strictPhases("EXECUTION.[Stage]"), strictBlocksTable))

	if table.Info.ID != strictID {
		t.Errorf("Info.ID: want %q carried through, got %q", strictID, table.Info.ID)
	}
}

func TestParseStrict_BlockRow_FillInsInInputAndOutputAreVerbatim(t *testing.T) {
	rows := []string{
		strictRowIO("EXECUTION.[Stage].Test", "agent-a",
			"Stage-{Stage}/Plan.md, Stage-*/Notes.md", "Stage-{Stage}/Tests.md"),
	}

	table := mustParseStrict(t, strictRegion(rows))

	row := table.Rows[0]
	wantIn := []string{"Stage-{Stage}/Plan.md", "Stage-*/Notes.md"}
	if !equalStrings(row.InputArtifacts, wantIn) {
		t.Errorf("InputArtifacts: want %v verbatim, got %v", wantIn, row.InputArtifacts)
	}
	wantOut := []string{"Stage-{Stage}/Tests.md"}
	if !equalStrings(row.OutputArtifacts, wantOut) {
		t.Errorf("OutputArtifacts: want %v verbatim, got %v", wantOut, row.OutputArtifacts)
	}
}

func TestParseStrict_GroupedRowsOfOtherBlock_DoNotLoseTheGroup(t *testing.T) {
	// A grouped row must keep both its block and its group; a parser that
	// treats the group segment as part of a bare block row drops the group.
	rows := strictPhases("EXECUTION.[Stage].Test", "EXECUTION.[Stage].Implementation")

	table := mustParseStrict(t, strictRegion(rows, strictBlocksTable, strictGroupsTable))

	if g := table.Rows[0].PhaseParsed.Group; g != "Test" {
		t.Errorf("Rows[0].Group: want Test, got %q", g)
	}
	if g := table.Rows[1].PhaseParsed.Group; g != "Implementation" {
		t.Errorf("Rows[1].Group: want Implementation, got %q", g)
	}
	for i, row := range table.Rows {
		if got := blockNames(row.PhaseParsed.Blocks); !equalStrings(got, []string{"Stage"}) {
			t.Errorf("Rows[%d].Blocks: want [Stage], got %v", i, got)
		}
	}
}
