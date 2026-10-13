package compat_test

// Tests for admission success of the five catalog workflow shapes and the
// per-block groups and approach rows, on WorkflowFormat 2.16 fixtures.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// shapeFixture names one workflow shape fixture.
type shapeFixture struct {
	name    string
	content string
	id      string
	grouped bool
}

func shapeFixtures() []shapeFixture {
	return []shapeFixture{
		{"brownfield-tdd-build-verified", brownfieldBuildVerifiedContent, "brownfield-tdd-build-verified", true},
		{"greenfield-tdd", greenFieldTDDContent, "greenfield-tdd", true},
		{"brownfield-tdd", brownfieldTDDContent, "brownfield-tdd", true},
		{"quick-fix", quickFixContent, "quick-fix", false},
		{"implementation-only", implOnlyContent, "implementation-only", false},
	}
}

// ---- Admission success ----

func TestAdmit_CatalogShapes_AreAdmittedInEveryMode(t *testing.T) {
	for _, fx := range shapeFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			table := mustParseShape(t, fx.content, fx.id, "4.0")

			forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
				admitIn(t, table, mode)
			})
		})
	}
}

// ---- One Stage block; groups are declared only for grouped shapes ----

func TestAdmit_CatalogShapes_HaveOneStageBlock(t *testing.T) {
	for _, fx := range shapeFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			aw := mustAdmit(t, mustParseShape(t, fx.content, fx.id, "4.0"))

			if len(aw.Blocks) != 1 || aw.Blocks[0].Name != "Stage" {
				t.Errorf("want the one block Stage, got %+v", aw.Blocks)
			}
		})
	}
}

func TestAdmit_CatalogShapes_GroupsDeclaredOnlyForGroupedRows(t *testing.T) {
	for _, fx := range shapeFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			aw := mustAdmit(t, mustParseShape(t, fx.content, fx.id, "4.0"))

			if got := stageBlock(t, aw).Grouped; got != fx.grouped {
				t.Errorf("block Grouped: want %v, got %v", fx.grouped, got)
			}
		})
	}
}

// ---- The approach table is carried onto the admitted workflow ----

func TestAdmit_GroupedShapes_ApproachTableCarriedThrough(t *testing.T) {
	for _, fx := range shapeFixtures() {
		if !fx.grouped {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			aw := mustAdmit(t, mustParseShape(t, fx.content, fx.id, "4.0"))

			if !aw.Table.ApproachTable.Present() {
				t.Fatal("ApproachTable: want present (the workflow declares a Groups table)")
			}
			want := []domain.Approach{"TDD", "Implementation-First", "Implementation-Only", "Tests-Only"}
			got := stageBlock(t, aw).Approaches.Approaches()
			if len(got) != len(want) {
				t.Fatalf("block approaches: want %v, got %v", want, got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("block approach %d: want %q, got %q", i, want[i], got[i])
				}
			}
		})
	}
}

func TestAdmit_UngroupedShapes_HaveNoApproachTable(t *testing.T) {
	for _, fx := range shapeFixtures() {
		if fx.grouped {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			aw := mustAdmit(t, mustParseShape(t, fx.content, fx.id, "4.0"))

			if aw.Table.ApproachTable.Present() {
				t.Error("ApproachTable: want not present for an ungrouped workflow")
			}
			if stageBlock(t, aw).Approaches.Present() {
				t.Error("block approaches: want none for an ungrouped block")
			}
		})
	}
}

// ---- Group lookup by name ----

// groupNamed finds the group of block b with exactly the given name.
func groupNamed(b domain.AdmittedBlock, name domain.GroupName) (domain.ExecutionGroup, bool) {
	for _, g := range b.Groups {
		if g.Name == name {
			return g, true
		}
	}
	return domain.ExecutionGroup{}, false
}

func TestAdmit_BlockGroups_FindGroupByName(t *testing.T) {
	aw := mustAdmit(t, mustParseShape(t, brownfieldBuildVerifiedContent, "brownfield-tdd-build-verified", "4.0"))

	g, ok := groupNamed(stageBlock(t, aw), "Test")
	if !ok {
		t.Fatal(`group "Test": want found=true, got false`)
	}
	if g.StartRow != 7 || g.EndRow != 10 {
		t.Errorf(`group "Test": want rows [7,10), got [%d,%d)`, g.StartRow, g.EndRow)
	}
}

func TestAdmit_BlockGroups_MissingGroupIsNotFound(t *testing.T) {
	aw := mustAdmit(t, mustParseShape(t, brownfieldBuildVerifiedContent, "brownfield-tdd-build-verified", "4.0"))

	_, ok := groupNamed(stageBlock(t, aw), "NoSuchGroup")

	if ok {
		t.Error(`group "NoSuchGroup": want found=false, got true`)
	}
}

func TestAdmit_BlockGroups_NamesAreCaseSensitive(t *testing.T) {
	aw := mustAdmit(t, mustParseShape(t, brownfieldBuildVerifiedContent, "brownfield-tdd-build-verified", "4.0"))

	_, ok := groupNamed(stageBlock(t, aw), "test")

	if ok {
		t.Error(`group "test": group names are case-sensitive, want found=false`)
	}
}
