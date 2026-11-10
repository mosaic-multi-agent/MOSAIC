package stack_test

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/stackerr"
)

// partialPairCases maps a case name to the layer fixture and the file that stays missing.
var partialPairCases = []struct {
	name    string
	add     func(t *testing.T, root string)
	missing string
}{
	{"only orchestrator.md", func(t *testing.T, r string) { addTestOrchestrator(t, r, "1.0.0") }, "orchestrator-script.md"},
	{"only orchestrator-script.md", func(t *testing.T, r string) { addTestOrchestratorScript(t, r, "1.0.0") }, "orchestrator.md"},
}

func TestNew_LayerWithOnlyOneOrchestratorFile_FailsNamingLayerAndMissingFile(t *testing.T) {
	for _, tc := range partialPairCases {
		t.Run(tc.name, func(t *testing.T) {
			mosaic := newTestMosaicRoot(t)
			root := newTestCatalogRoot(t, "partial")
			tc.add(t, root)

			err := composeRoots(mosaic, root)

			var le *stackerr.LayerError
			if !errors.As(err, &le) {
				t.Fatalf("New error = %v, want *stackerr.LayerError", err)
			}
			if le.Kind != stackerr.ErrOrchestratorPairIncomplete {
				t.Errorf("Kind = %v, want ErrOrchestratorPairIncomplete", le.Kind)
			}
			if le.Root != root || le.CatalogID != "partial" {
				t.Errorf("layer = (%q, %q), want (%q, partial)", le.Root, le.CatalogID, root)
			}
			if le.Err == nil {
				t.Fatal("Err is nil, want a cause naming the missing file")
			}
			assertContains(t, le.Err.Error(), tc.missing)
		})
	}
}

func TestNew_PartialPairInLowerLayer_FailsEvenWhenHigherLayerSuppliesPair(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestrator(t, high, "9.0.0")
	addTestOrchestratorScript(t, high, "9.0.0")
	addTestOrchestrator(t, low, "1.0.0")

	err := composeRoots(mosaic, high, low)

	var le *stackerr.LayerError
	if !errors.As(err, &le) || le.Kind != stackerr.ErrOrchestratorPairIncomplete {
		t.Fatalf("New error = %v, want LayerError of kind ErrOrchestratorPairIncomplete", err)
	}
	if le.Root != low {
		t.Errorf("LayerError.Root = %q, want the lower layer %q", le.Root, low)
	}
}

func TestNew_TwoPartialLayers_ReportsHighestFirst(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestratorScript(t, high, "9.0.0")
	addTestOrchestrator(t, low, "1.0.0")

	err := composeRoots(mosaic, high, low)

	var le *stackerr.LayerError
	if !errors.As(err, &le) || le.Root != high {
		t.Fatalf("New error = %v, want the highest partial layer %q", err, high)
	}
}

func TestNew_LayersWithBothOrNeitherOrchestratorFile_Accepted(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	both, neither := newTestCatalogRoot(t, "both"), newTestCatalogRoot(t, "neither")
	addTestOrchestrator(t, both, "1.0.0")
	addTestOrchestratorScript(t, both, "1.0.0")

	if _, err := stack.New(mosaic, []stack.Layer{{Root: both}, {Root: neither}}); err != nil {
		t.Fatalf("New with a complete pair and a pair-less layer: %v", err)
	}
	if err := composeRoots(mosaic, neither); err != nil {
		t.Fatalf("New with a pair-less stack: %v", err)
	}
}
