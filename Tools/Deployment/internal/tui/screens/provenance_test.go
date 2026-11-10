package screens_test

// provenance_test.go verifies that every selection screen shows, next to each of its items, the
// catalog label it received through SetProvenance; that items from different catalogs show
// different labels; and that unknown provenance (a nil label map, an id missing from the map)
// renders as "unknown" without breaking the screen.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/tui/screens"
)

const unknownLabel = "unknown"

// TestSelectionScreens_ShowTheLabelOfEachItemNextToIt verifies that each item's row shows the
// label given for it, and not the label of an item from another catalog.
func TestSelectionScreens_ShowTheLabelOfEachItemNextToIt(t *testing.T) {
	for _, ps := range provScreens() {
		t.Run(ps.name, func(t *testing.T) {
			labels := screens.ProvenanceLabels{
				ps.keys[0]: "layer-alpha",
				ps.keys[1]: "layer-beta",
				ps.keys[2]: "layer-gamma",
			}

			view := ps.view(provWidth, provHeight, labels)

			for i, want := range []string{"layer-alpha", "layer-beta", "layer-gamma"} {
				row := provRow(t, view, ps.marks[i])
				if !strings.Contains(row, want) {
					t.Errorf("row of %q does not show its catalog %q:\n%s", ps.marks[i], want, row)
				}
				for j, other := range []string{"layer-alpha", "layer-beta", "layer-gamma"} {
					if j != i && strings.Contains(row, other) {
						t.Errorf("row of %q shows %q, the catalog of another item:\n%s", ps.marks[i], other, row)
					}
				}
			}
		})
	}
}

// TestSelectionScreens_NilLabels_RenderEveryItemAsUnknown verifies that a nil label map marks
// every item as unknown.
func TestSelectionScreens_NilLabels_RenderEveryItemAsUnknown(t *testing.T) {
	for _, ps := range provScreens() {
		t.Run(ps.name, func(t *testing.T) {
			view := ps.view(provWidth, provHeight, nil)

			for _, mark := range ps.marks {
				if row := provRow(t, view, mark); !strings.Contains(row, unknownLabel) {
					t.Errorf("row of %q does not show %q for a nil label map:\n%s", mark, unknownLabel, row)
				}
			}
		})
	}
}

// TestSelectionScreens_IdMissingFromLabels_RendersOnlyThatItemAsUnknown verifies that an item
// whose id the label map lacks shows "unknown", while the labelled items keep their label.
func TestSelectionScreens_IdMissingFromLabels_RendersOnlyThatItemAsUnknown(t *testing.T) {
	for _, ps := range provScreens() {
		t.Run(ps.name, func(t *testing.T) {
			labels := screens.ProvenanceLabels{ps.keys[0]: "layer-alpha", ps.keys[2]: "layer-gamma"}

			view := ps.view(provWidth, provHeight, labels)

			missing := provRow(t, view, ps.marks[1])
			if !strings.Contains(missing, unknownLabel) {
				t.Errorf("row of %q (id not in the map) does not show %q:\n%s", ps.marks[1], unknownLabel, missing)
			}
			for _, i := range []int{0, 2} {
				row := provRow(t, view, ps.marks[i])
				if strings.Contains(row, unknownLabel) {
					t.Errorf("row of %q has a label but shows %q:\n%s", ps.marks[i], unknownLabel, row)
				}
			}
		})
	}
}

// TestSelectionScreens_EmptyLabelMap_RendersEveryItemAsUnknown verifies that a non-nil map
// without entries behaves like a nil map.
func TestSelectionScreens_EmptyLabelMap_RendersEveryItemAsUnknown(t *testing.T) {
	for _, ps := range provScreens() {
		t.Run(ps.name, func(t *testing.T) {
			view := ps.view(provWidth, provHeight, screens.ProvenanceLabels{})

			for _, mark := range ps.marks {
				if row := provRow(t, view, mark); !strings.Contains(row, unknownLabel) {
					t.Errorf("row of %q does not show %q for an empty label map:\n%s", mark, unknownLabel, row)
				}
			}
		})
	}
}

// TestInfrastructureScreen_BothOrchestratorAgentsShowTheLayerOfThePair verifies that the
// orchestrator and the script orchestrator, which the pair's layer supplies together, both show
// that layer while another infrastructure agent shows its own.
func TestInfrastructureScreen_BothOrchestratorAgentsShowTheLayerOfThePair(t *testing.T) {
	ps := provInfrastructureScreen()
	labels := screens.ProvenanceLabels{
		"orchestrator":        "layer-pair",
		"orchestrator-script": "layer-pair",
		"prov-infra-x":        "layer-other",
	}

	view := ps.view(provWidth, provHeight, labels)

	for _, mark := range []string{"Orchestrator Primary", "Orchestrator Variant"} {
		if row := provRow(t, view, mark); !strings.Contains(row, "layer-pair") {
			t.Errorf("row of %q does not show the pair's layer:\n%s", mark, row)
		}
	}
	if row := provRow(t, view, "Infra Extra"); !strings.Contains(row, "layer-other") || strings.Contains(row, "layer-pair") {
		t.Errorf("row of the non-orchestrator agent must show only its own layer:\n%s", row)
	}
}

// TestSelectionScreens_NarrowTerminal_RendersWithLabelsWithoutBreaking verifies that a small
// terminal still renders every screen, with and without labels, and the items stay listed.
func TestSelectionScreens_NarrowTerminal_RendersWithLabelsWithoutBreaking(t *testing.T) {
	for _, ps := range provScreens() {
		t.Run(ps.name, func(t *testing.T) {
			labels := screens.ProvenanceLabels{ps.keys[0]: "layer-alpha", ps.keys[1]: "layer-beta", ps.keys[2]: "layer-gamma"}

			for _, l := range []screens.ProvenanceLabels{labels, nil} {
				view := ps.view(40, 12, l)

				if strings.TrimSpace(view) == "" {
					t.Error("narrow terminal rendered an empty view")
				}
			}
		})
	}
}
