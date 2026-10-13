package compat_test

// Tests for execution group resolution: group names and row ranges of the
// brownfield-tdd-build-verified, brownfield-tdd, greenfield-tdd, quick-fix and
// implementation-only shapes, asserted against the per-block admitted model.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

func TestAdmit_CatalogShapes_GroupNamesAndRowRanges(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []domain.ExecutionGroup
	}{
		{"brownfield-tdd-build-verified", brownfieldBuildVerifiedContent, []domain.ExecutionGroup{
			{Name: "Test", StartRow: 7, EndRow: 10},
			{Name: "Implementation", StartRow: 10, EndRow: 13},
		}},
		{"brownfield-tdd", brownfieldTDDContent, []domain.ExecutionGroup{
			{Name: "Test", StartRow: 7, EndRow: 9},
			{Name: "Implementation", StartRow: 9, EndRow: 11},
		}},
		{"greenfield-tdd", greenFieldTDDContent, []domain.ExecutionGroup{
			{Name: "Test", StartRow: 8, EndRow: 10},
			{Name: "Implementation", StartRow: 10, EndRow: 12},
		}},
		{"quick-fix", quickFixContent, []domain.ExecutionGroup{{Name: "", StartRow: 2, EndRow: 3}}},
		{"implementation-only", implOnlyContent, []domain.ExecutionGroup{{Name: "", StartRow: 0, EndRow: 2}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := mustAdmit(t, mustParseShape(t, tc.content, tc.name, "4.0"))

			if got := stageBlock(t, aw).Groups; !reflect.DeepEqual(got, tc.want) {
				t.Errorf("block Groups: want %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestAdmit_TwoGroupShape_GroupsAreContiguousAndCoverTheBlock(t *testing.T) {
	aw := mustAdmit(t, mustParseShape(t, brownfieldBuildVerifiedContent, "brownfield-tdd-build-verified", "4.0"))

	b := stageBlock(t, aw)

	if len(b.Groups) != 2 {
		t.Fatalf("Groups: want 2, got %d", len(b.Groups))
	}
	if b.Groups[0].EndRow != b.Groups[1].StartRow {
		t.Errorf("groups not contiguous: first ends at %d, second starts at %d", b.Groups[0].EndRow, b.Groups[1].StartRow)
	}
	if b.Groups[0].StartRow != b.StartRow || b.Groups[1].EndRow != b.EndRow {
		t.Errorf("groups must cover the block [%d,%d), got [%d,%d)",
			b.StartRow, b.EndRow, b.Groups[0].StartRow, b.Groups[1].EndRow)
	}
}

func TestAdmit_UngroupedBlock_HoldsOneImplicitGroupWithEmptyName(t *testing.T) {
	for _, fx := range shapeFixtures() {
		if fx.grouped {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			aw := mustAdmit(t, mustParseShape(t, fx.content, fx.id, "4.0"))

			b := stageBlock(t, aw)

			if len(b.Groups) != 1 || b.Groups[0].Name != "" {
				t.Errorf("Groups: want one implicit group with an empty name, got %+v", b.Groups)
			}
		})
	}
}

func TestAdmit_RepeatedAgentThroughModeQualifiers_IsAdmittedAsDistinctRows(t *testing.T) {
	// build-review appears in two rows, told apart by the qualifiers (tests) and
	// (impl); both rows keep the bare id build-review.
	table := mustParseShape(t, brownfieldBuildVerifiedContent, "brownfield-tdd-build-verified", "4.0")

	aw := admitIn(t, table, domain.ExecutionModeAuto)

	first, second := aw.Table.Rows[8], aw.Table.Rows[11]
	if first.BareAgentID() != "build-review" || second.BareAgentID() != "build-review" {
		t.Fatalf("rows 9 and 12 must both be build-review, got %q and %q", first.BareAgentID(), second.BareAgentID())
	}
	if first.Agent == second.Agent {
		t.Errorf("the two build-review rows must keep distinct whole cells, both are %q", first.Agent)
	}
}
