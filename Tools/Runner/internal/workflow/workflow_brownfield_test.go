package workflow_test

// Row parsing of the grouped brownfield shape (13 rows) and the
// implementation-only shape (starts with block rows) by the strict parser.

import (
	"testing"
)

func TestParse_Brownfield_RowCount(t *testing.T) {
	table := mustParseBrownfield(t)

	if len(table.Rows) != 13 {
		t.Errorf("want 13 rows, got %d", len(table.Rows))
	}
}

func TestParse_Brownfield_RowIndices_ZeroBasedConsecutive(t *testing.T) {
	table := mustParseBrownfield(t)

	for i, row := range table.Rows {
		if row.Index != i {
			t.Errorf("Rows[%d].Index: want %d, got %d", i, i, row.Index)
		}
	}
}

func TestParse_Brownfield_ExecutionRows_AllInTheStageBlock(t *testing.T) {
	// Rows 7-12 are EXECUTION.[Stage].{Group} rows and must all carry block Stage.
	table := mustParseBrownfield(t)

	for i := 7; i <= 12; i++ {
		if !table.Rows[i].PhaseParsed.IsStaged {
			t.Errorf("Rows[%d] (%s): PhaseParsed.IsStaged must be true", i, table.Rows[i].Phase)
		}
		if b, ok := table.Rows[i].Block(); !ok || b != "Stage" {
			t.Errorf("Rows[%d].Block(): want (Stage, true), got (%q, %v)", i, b, ok)
		}
	}
}

func TestParse_Brownfield_NonExecutionRows_NotInABlock(t *testing.T) {
	// Rows 0-6 (RESEARCH, PLANNING, DESIGN) are outside every block.
	table := mustParseBrownfield(t)

	for i := 0; i <= 6; i++ {
		if table.Rows[i].PhaseParsed.IsStaged {
			t.Errorf("Rows[%d] (%s): PhaseParsed.IsStaged must be false", i, table.Rows[i].Phase)
		}
	}
}

func TestParse_Brownfield_ArtifactFillIn_PreservedVerbatim(t *testing.T) {
	// Row 7 (test-writer-tdd) input includes "Stage-{Stage}/Plan.md".
	// The fill-in notation must not be altered during parsing.
	table := mustParseBrownfield(t)

	artifacts := table.Rows[7].InputArtifacts
	if !containsArtifact(artifacts, "Stage-{Stage}/Plan.md") {
		t.Errorf("Rows[7].InputArtifacts: want %q verbatim, got %v", "Stage-{Stage}/Plan.md", artifacts)
	}
}

func TestParse_Brownfield_MultiInputArtifacts_SplitCorrectly(t *testing.T) {
	// Row 9 (tests-review-tdd) has 4 comma-separated input paths.
	table := mustParseBrownfield(t)

	got := len(table.Rows[9].InputArtifacts)
	if got != 4 {
		t.Errorf("Rows[9].InputArtifacts: want 4 elements, got %d: %v", got, table.Rows[9].InputArtifacts)
	}
}

func TestParse_Brownfield_GlobPattern_OutputArtifact_Preserved(t *testing.T) {
	// Row 3 (planner-tdd-soft) output includes "Stage-*/Plan.md".
	table := mustParseBrownfield(t)

	artifacts := table.Rows[3].OutputArtifacts
	if !containsArtifact(artifacts, "Stage-*/Plan.md") {
		t.Errorf("Rows[3].OutputArtifacts: want %q verbatim, got %v", "Stage-*/Plan.md", artifacts)
	}
}

func TestParse_Brownfield_RepeatedAgent_KeepsWholeCellsAndBareId(t *testing.T) {
	// build-review appears in rows 8 and 11, told apart by mode qualifiers. Each
	// row keeps its whole cell; both carry the bare id.
	table := mustParseBrownfield(t)

	cases := map[int]string{8: "build-review(tests)", 11: "build-review(impl)"}
	for i, wantCell := range cases {
		row := table.Rows[i]
		if row.Agent != wantCell {
			t.Errorf("Rows[%d].Agent: want the whole cell %q, got %q", i, wantCell, row.Agent)
		}
		if row.BareAgentID() != "build-review" {
			t.Errorf("Rows[%d].BareAgentID(): want build-review, got %q", i, row.BareAgentID())
		}
	}
}

// ---- implementation-only: starts with block rows ----

func TestParse_ImplementationOnly_RowCount(t *testing.T) {
	table := mustParseImplOnly(t)

	if len(table.Rows) != 3 {
		t.Errorf("want 3 rows, got %d", len(table.Rows))
	}
}

func TestParse_ImplementationOnly_FirstRow_IsInTheStageBlock(t *testing.T) {
	// This workflow has no row before the block; row 0 must be a block row.
	table := mustParseImplOnly(t)

	if !table.Rows[0].PhaseParsed.IsStaged {
		t.Error("Rows[0] (EXECUTION.[Stage]): PhaseParsed.IsStaged must be true")
	}
}

func TestParse_ImplementationOnly_LastRow_NotInABlock(t *testing.T) {
	// Row 2 (REVIEW) is outside every block.
	table := mustParseImplOnly(t)

	if table.Rows[2].PhaseParsed.IsStaged {
		t.Errorf("Rows[2] (REVIEW): PhaseParsed.IsStaged must be false, got true")
	}
}
