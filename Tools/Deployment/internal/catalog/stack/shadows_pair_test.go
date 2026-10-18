package stack_test

import (
	"testing"

	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// assertNoOrchestratorAgentRecords fails when a keyed agent record exists for either
// orchestrator key: the pair is reported once, as the pair item.
func assertNoOrchestratorAgentRecords(t *testing.T, records []stackrec.Shadow) {
	t.Helper()
	for _, r := range records {
		if r.Item.Kind == domain.CatalogItemAgent && (r.Item.Key == "orchestrator" || r.Item.Key == "orchestrator-script") {
			t.Errorf("unexpected keyed agent record for an orchestrator key: %+v", r)
		}
	}
}

func TestStack_PairShadow_TwoLayersSupplyingThePairYieldOneRecord(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestratorPair(t, high, "2.0.0")
	addTestOrchestratorPair(t, low, "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	records := c.Shadows()

	if len(records) != 1 {
		t.Fatalf("Shadows() = %+v, want exactly one record for the pair", records)
	}
	assertShadow(t, shadowFor(t, records, pairItem()), testLayer(high, "high"), []domain.CatalogLayer{testLayer(low, "low")}, false)
	assertNoOrchestratorAgentRecords(t, records)
}

func TestStack_PairShadow_ThreeLayersListEveryShadowedLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	top, mid, low := newTestCatalogRoot(t, "top"), newTestCatalogRoot(t, "mid"), newTestCatalogRoot(t, "low")
	addTestOrchestratorPair(t, top, "3.0.0")
	addTestOrchestratorPair(t, mid, "2.0.0")
	addTestOrchestratorPair(t, low, "1.0.0")
	c := newTestProvenanceStack(t, mosaic, top, mid, low)

	records := c.Shadows()

	if len(records) != 1 {
		t.Fatalf("Shadows() = %+v, want exactly one record for the pair", records)
	}
	assertShadow(t, shadowFor(t, records, pairItem()),
		testLayer(top, "top"), []domain.CatalogLayer{testLayer(mid, "mid"), testLayer(low, "low")}, false)
}

func TestStack_PairShadow_DeclaredFlagFollowsThePairDeclaration(t *testing.T) {
	cases := []struct {
		name         string
		highDeclares bool
		lowDeclares  bool
		want         bool
	}{
		{"winner declares the pair", true, false, true},
		{"winner and shadowed layer declare the pair", true, true, true},
		{"only the shadowed layer declares the pair", false, true, false},
		{"nobody declares the pair", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mosaic := newTestMosaicRoot(t)
			high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
			if tc.highDeclares {
				withTestOverrides(t, high, "high", "  orchestrator_pair: true\n")
			}
			if tc.lowDeclares {
				withTestOverrides(t, low, "low", "  orchestrator_pair: true\n")
			}
			addTestOrchestratorPair(t, high, "2.0.0")
			addTestOrchestratorPair(t, low, "1.0.0")
			c := newTestProvenanceStack(t, mosaic, high, low)

			got := shadowFor(t, c.Shadows(), pairItem())

			if got.Declared != tc.want {
				t.Errorf("pair record Declared = %v, want %v", got.Declared, tc.want)
			}
		})
	}
}

func TestStack_PairShadow_DeclaredWinnerKeepsKeyedAgentRecordsSeparate(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	withTestOverrides(t, high, "high", "  orchestrator_pair: true\n")
	addTestOrchestratorPair(t, high, "2.0.0")
	addTestOrchestratorPair(t, low, "1.0.0")
	addTestSubagent(t, high, "Cat", "worker", "2.0.0")
	addTestSubagent(t, low, "Cat", "worker", "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	records := c.Shadows()

	if len(records) != 2 {
		t.Fatalf("Shadows() = %+v, want one pair record and one agent record", records)
	}
	if !shadowFor(t, records, pairItem()).Declared {
		t.Error("pair record must be declared by the winner's orchestrator_pair declaration")
	}
	if shadowFor(t, records, agentItem("worker")).Declared {
		t.Error("the worker record must not inherit the pair declaration")
	}
	assertNoOrchestratorAgentRecords(t, records)
}

func TestStack_PairShadow_OnlyOneLayerSupplyingThePairYieldsNoRecord(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, mid, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "mid"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "1.0.0")
	addTestOrchestratorPair(t, mid, "1.0.0")
	addTestSkill(t, low, "skill", "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, mid, low)

	records := c.Shadows()

	if len(records) != 0 {
		t.Errorf("Shadows() = %+v, want none: only one layer supplies the pair", records)
	}
}

func TestStack_PairShadow_PairSuppliedByNoLayerYieldsNoRecord(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	c := newTestProvenanceStack(t, mosaic, high, low)

	if records := c.Shadows(); len(records) != 0 {
		t.Errorf("Shadows() = %+v, want none", records)
	}
}
