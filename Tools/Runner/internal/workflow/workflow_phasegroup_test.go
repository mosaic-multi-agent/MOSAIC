package workflow_test

// Group parsing of the Phase cell on a table that mixes a plain row, grouped
// rows of one block and an ungrouped row of another block. The refusals of
// malformed group notation are asserted by strict_phase_refusals_test.go.

import (
	"testing"
)

func TestParse_GroupedPhase_Test_GroupField(t *testing.T) {
	// EXECUTION.[Stage].Test must populate PhaseParsed.Group with "Test".
	table := mustParseGroupedPhases(t)

	got := table.Rows[1].PhaseParsed.Group
	if got != "Test" {
		t.Errorf("Rows[1].PhaseParsed.Group: want %q, got %q", "Test", got)
	}
}

func TestParse_GroupedPhase_Implementation_GroupField(t *testing.T) {
	// EXECUTION.[Stage].Implementation must populate PhaseParsed.Group with
	// "Implementation".
	table := mustParseGroupedPhases(t)

	got := table.Rows[2].PhaseParsed.Group
	if got != "Implementation" {
		t.Errorf("Rows[2].PhaseParsed.Group: want %q, got %q", "Implementation", got)
	}
}

func TestParse_UngroupedBlockPhase_GroupField_IsEmpty(t *testing.T) {
	// A block row with no group segment must have an empty Group.
	table := mustParseGroupedPhases(t)

	got := table.Rows[3].PhaseParsed.Group
	if got != "" {
		t.Errorf("Rows[3].PhaseParsed.Group: want %q for an ungrouped block row, got %q", "", got)
	}
}

func TestParse_PlainPhase_GroupField_IsEmpty(t *testing.T) {
	// A plain phase (RESEARCH) must have an empty Group.
	table := mustParseGroupedPhases(t)

	got := table.Rows[0].PhaseParsed.Group
	if got != "" {
		t.Errorf("Rows[0].PhaseParsed.Group: want %q for RESEARCH phase, got %q", "", got)
	}
}

func TestParse_GroupedPhase_PhaseName_StaysTheBareName(t *testing.T) {
	// When the phase string has a block reference and a group segment,
	// PhaseParsed.Name must still be the bare phase name "EXECUTION".
	table := mustParseGroupedPhases(t)

	got := table.Rows[1].PhaseParsed.Name
	if got != "EXECUTION" {
		t.Errorf("Rows[1].PhaseParsed.Name: want %q for EXECUTION.[Stage].Test, got %q", "EXECUTION", got)
	}
}

func TestParse_GroupedPhase_IsStaged_True(t *testing.T) {
	// A grouped row is a row of its block and must be flagged as staged.
	table := mustParseGroupedPhases(t)

	if !table.Rows[1].PhaseParsed.IsStaged {
		t.Error("Rows[1] (EXECUTION.[Stage].Test): PhaseParsed.IsStaged must be true")
	}
	if b, ok := table.Rows[1].Block(); !ok || b != "Stage" {
		t.Errorf("Rows[1].Block(): want (Stage, true), got (%q, %v)", b, ok)
	}
}

func TestParse_GroupedPhase_Phase_LiteralPreserved(t *testing.T) {
	// The Phase field must carry the literal cell text, including the block
	// reference and the group segment.
	table := mustParseGroupedPhases(t)

	got := table.Rows[1].Phase
	want := "EXECUTION.[Stage].Test"
	if got != want {
		t.Errorf("Rows[1].Phase: want %q, got %q", want, got)
	}
}
