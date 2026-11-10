package stack_test

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestStack_Shadows_TwoLayersSharingKeyYieldOneRecordPerKind(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "2.0.0")
	addTestSubagent(t, low, "Cat", "worker", "1.0.0")
	addTestWorkflow(t, high, "Cat", "flow", "2.0.0")
	addTestWorkflow(t, low, "Cat", "flow", "1.0.0")
	addTestSkill(t, high, "skill", "2.0.0")
	addTestSkill(t, low, "skill", "1.0.0")
	addTestHook(t, high, "hook", "h1")
	addTestHook(t, low, "hook", "h1")
	c := newTestProvenanceStack(t, mosaic, high, low)

	records := c.Shadows()

	if len(records) != 4 {
		t.Fatalf("Shadows() has %d records, want 4 (one per shadowed key): %+v", len(records), records)
	}
	items := []domain.CatalogItem{
		agentItem("worker"),
		{Kind: domain.CatalogItemWorkflow, Key: "flow"},
		{Kind: domain.CatalogItemSkill, Key: "skill"},
		{Kind: domain.CatalogItemHook, Key: "hook"},
	}
	for _, item := range items {
		assertShadow(t, shadowFor(t, records, item), testLayer(high, "high"), []domain.CatalogLayer{testLayer(low, "low")}, false)
	}
}

func TestStack_Shadows_KeyAcrossRolesIsOneAgentRecord(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "shared", "2.0.0")
	addTestUtility(t, low, "shared", "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	records := c.Shadows()

	if len(records) != 1 {
		t.Fatalf("Shadows() = %+v, want exactly one record", records)
	}
	assertShadow(t, shadowFor(t, records, agentItem("shared")), testLayer(high, "high"), []domain.CatalogLayer{testLayer(low, "low")}, false)
}

func TestStack_Shadows_ThreeLayersListEveryShadowedLayerHighestFirst(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	top, mid, low := newTestCatalogRoot(t, "top"), newTestCatalogRoot(t, "mid"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, top, "Cat", "everywhere", "3.0.0")
	addTestSubagent(t, mid, "Cat", "everywhere", "2.0.0")
	addTestSubagent(t, low, "Cat", "everywhere", "1.0.0")
	addTestSkill(t, top, "skip-mid", "3.0.0")
	addTestSkill(t, low, "skip-mid", "1.0.0")
	addTestWorkflow(t, mid, "Cat", "lower-two", "2.0.0")
	addTestWorkflow(t, low, "Cat", "lower-two", "1.0.0")
	c := newTestProvenanceStack(t, mosaic, top, mid, low)

	records := c.Shadows()

	if len(records) != 3 {
		t.Fatalf("Shadows() has %d records, want 3: %+v", len(records), records)
	}
	assertShadow(t, shadowFor(t, records, agentItem("everywhere")),
		testLayer(top, "top"), []domain.CatalogLayer{testLayer(mid, "mid"), testLayer(low, "low")}, false)
	assertShadow(t, shadowFor(t, records, domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: "skip-mid"}),
		testLayer(top, "top"), []domain.CatalogLayer{testLayer(low, "low")}, false)
	assertShadow(t, shadowFor(t, records, domain.CatalogItem{Kind: domain.CatalogItemWorkflow, Key: "lower-two"}),
		testLayer(mid, "mid"), []domain.CatalogLayer{testLayer(low, "low")}, false)
}

func TestStack_Shadows_UnshadowedItemsHaveNoRecord(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "hi-worker", "1.0.0")
	addTestSubagent(t, low, "Cat", "lo-worker", "1.0.0")
	addTestSkill(t, high, "hi-skill", "1.0.0")
	addTestWorkflow(t, low, "Cat", "lo-flow", "1.0.0")
	addTestHook(t, high, "hi-hook", "h1")
	addTestOrchestratorPair(t, low, "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	if records := c.Shadows(); len(records) != 0 {
		t.Errorf("Shadows() = %+v, want none: no item is in more than one layer", records)
	}
}

func TestStack_Shadows_SingleLayerHasNoRecords(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	only := newTestCatalogRoot(t, "only")
	addTestSubagent(t, only, "Cat", "worker", "1.0.0")
	c := newTestProvenanceStack(t, mosaic, only)

	if records := c.Shadows(); len(records) != 0 {
		t.Errorf("Shadows() = %+v, want none for a single-layer stack", records)
	}
}

func TestStack_Shadows_DeclaredFlagFollowsWinnersIdentity(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, mid, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "mid"), newTestCatalogRoot(t, "low")
	withTestOverrides(t, high, "high", "  agents: [declared-agent]\n  skills: [declared-skill]\n")
	withTestOverrides(t, mid, "mid", "  workflows: [mid-flow]\n")
	withTestOverrides(t, low, "low", "  agents: [undeclared-agent, mid-undeclared-agent]\n  hooks: [low-hook]\n")
	addTestSubagent(t, high, "Cat", "declared-agent", "3.0.0")
	addTestSubagent(t, mid, "Cat", "declared-agent", "2.0.0")
	addTestSubagent(t, high, "Cat", "undeclared-agent", "3.0.0")
	addTestSubagent(t, low, "Cat", "undeclared-agent", "1.0.0")
	addTestSkill(t, high, "declared-skill", "3.0.0")
	addTestSkill(t, low, "declared-skill", "1.0.0")
	addTestSubagent(t, mid, "Cat", "mid-undeclared-agent", "2.0.0")
	addTestSubagent(t, low, "Cat", "mid-undeclared-agent", "1.0.0")
	addTestWorkflow(t, mid, "Cat", "mid-flow", "2.0.0")
	addTestWorkflow(t, low, "Cat", "mid-flow", "1.0.0")
	addTestHook(t, mid, "low-hook", "h1")
	addTestHook(t, low, "low-hook", "h1")
	c := newTestProvenanceStack(t, mosaic, high, mid, low)

	records := c.Shadows()

	want := map[domain.CatalogItem]bool{
		agentItem("declared-agent"):                            true,
		agentItem("undeclared-agent"):                          false, // only the shadowed layer declares it
		{Kind: domain.CatalogItemSkill, Key: "declared-skill"}: true,
		agentItem("mid-undeclared-agent"):                      false, // winner (mid) does not declare it, only low does
		{Kind: domain.CatalogItemWorkflow, Key: "mid-flow"}:    true,
		{Kind: domain.CatalogItemHook, Key: "low-hook"}:        false, // only the shadowed layer declares it
	}
	if len(records) != len(want) {
		t.Fatalf("Shadows() has %d records, want %d: %+v", len(records), len(want), records)
	}
	for item, declared := range want {
		if got := shadowFor(t, records, item).Declared; got != declared {
			t.Errorf("%+v: Declared = %v, want %v", item, got, declared)
		}
	}
}

func TestStack_Shadows_AreSortedByKindThenKey(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	for _, root := range []string{high, low} {
		addTestSubagent(t, root, "Cat", "b-agent", "1.0.0")
		addTestSubagent(t, root, "Cat", "a-agent", "1.0.0")
		addTestWorkflow(t, root, "Cat", "flow", "1.0.0")
		addTestSkill(t, root, "skill", "1.0.0")
		addTestHook(t, root, "hook", "h1")
	}
	c := newTestProvenanceStack(t, mosaic, high, low)

	records := c.Shadows()

	want := []domain.CatalogItem{
		agentItem("a-agent"),
		agentItem("b-agent"),
		{Kind: domain.CatalogItemHook, Key: "hook"},
		{Kind: domain.CatalogItemSkill, Key: "skill"},
		{Kind: domain.CatalogItemWorkflow, Key: "flow"},
	}
	if len(records) != len(want) {
		t.Fatalf("Shadows() has %d records, want %d: %+v", len(records), len(want), records)
	}
	for i, item := range want {
		if records[i].Item != item {
			t.Errorf("Shadows()[%d].Item = %+v, want %+v", i, records[i].Item, item)
		}
	}
}
