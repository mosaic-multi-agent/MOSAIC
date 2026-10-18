package stack_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

func TestStack_CompanionInterface_ExposesProvenanceAndShadows(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := newTestCatalogRoot(t, "only")

	c := newTestStack(t, mosaic, root)

	if _, ok := c.(provenanceStack); !ok {
		t.Fatalf("stack.Catalog %T must expose Provenance and Shadows", c)
	}
}

func TestStack_Provenance_ReportsSupplyingLayerPerKindAndPosition(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "hi-worker", "1.0.0")
	addTestSubagent(t, low, "Cat", "lo-worker", "1.0.0")
	addTestUtility(t, low, "lo-util", "1.0.0")
	addTestStandalone(t, high, "hi-solo", "1.0.0")
	addTestWorkflow(t, high, "Cat", "hi-flow", "1.0.0")
	addTestWorkflow(t, low, "Cat", "lo-flow", "1.0.0")
	addTestSkill(t, high, "hi-skill", "1.0.0")
	addTestSkill(t, low, "lo-skill", "1.0.0")
	addTestHook(t, high, "hi-hook", "h1")
	addTestHook(t, low, "lo-hook", "h1")
	c := newTestProvenanceStack(t, mosaic, high, low)

	cases := []struct {
		name string
		item domain.CatalogItem
		root string
		id   string
	}{
		{"worker in top layer", agentItem("hi-worker"), high, "high"},
		{"worker only in lower layer", agentItem("lo-worker"), low, "low"},
		{"utility agent in lower layer", agentItem("lo-util"), low, "low"},
		{"standalone agent in top layer", agentItem("hi-solo"), high, "high"},
		{"workflow in top layer", domain.CatalogItem{Kind: domain.CatalogItemWorkflow, Key: "hi-flow"}, high, "high"},
		{"workflow only in lower layer", domain.CatalogItem{Kind: domain.CatalogItemWorkflow, Key: "lo-flow"}, low, "low"},
		{"skill in top layer", domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: "hi-skill"}, high, "high"},
		{"skill only in lower layer", domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: "lo-skill"}, low, "low"},
		{"hook in top layer", domain.CatalogItem{Kind: domain.CatalogItemHook, Key: "hi-hook"}, high, "high"},
		{"hook only in lower layer", domain.CatalogItem{Kind: domain.CatalogItemHook, Key: "lo-hook"}, low, "low"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := c.Provenance(tc.item)

			if !ok {
				t.Fatalf("Provenance(%+v) not found", tc.item)
			}
			if want := testLayer(tc.root, tc.id); got != want {
				t.Errorf("Provenance(%+v) = %+v, want %+v", tc.item, got, want)
			}
		})
	}
}

func TestStack_Provenance_KeyPresentInSeveralLayersReportsWinner(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	top, mid, low := newTestCatalogRoot(t, "top"), newTestCatalogRoot(t, "mid"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, mid, "Cat", "shared", "2.0.0")
	addTestSubagent(t, low, "Cat", "shared", "1.0.0")
	addTestSkill(t, top, "shared-skill", "3.0.0")
	addTestSkill(t, low, "shared-skill", "1.0.0")
	c := newTestProvenanceStack(t, mosaic, top, mid, low)

	agent, agentOK := c.Provenance(agentItem("shared"))
	skill, skillOK := c.Provenance(domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: "shared-skill"})

	if !agentOK || agent.Identity.ID != "mid" {
		t.Errorf("Provenance(agent shared) = %+v, %v, want the mid layer", agent, agentOK)
	}
	if !skillOK || skill.Identity.ID != "top" {
		t.Errorf("Provenance(skill shared-skill) = %+v, %v, want the top layer", skill, skillOK)
	}
}

func TestStack_Provenance_DefaultLayerCarriesDefaultFlagAndVersion(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	defRoot := filepath.Join(mosaic, "Catalog")
	mustWrite(t, defRoot, "catalog-identity.yaml", "id: mosaic\nname: MOSAIC\nversion: \"4.2.0\"\n")
	addTestSkill(t, defRoot, "default-skill", "1.0.0")
	user := newTestCatalogRoot(t, "user")
	st, err := stack.New(mosaic, []stack.Layer{{Root: user}, {Root: defRoot, Default: true}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	c, ok := st.(provenanceStack)
	if !ok {
		t.Fatalf("stack.Catalog %T does not expose Provenance and Shadows", st)
	}

	got, ok := c.Provenance(domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: "default-skill"})

	if !ok {
		t.Fatal("Provenance(default-skill) not found")
	}
	if !got.Default || got.Identity.ID != "mosaic" || got.Identity.Version != "4.2.0" || got.Root != defRoot {
		t.Errorf("Provenance(default-skill) = %+v, want the default layer mosaic 4.2.0 at %s", got, defRoot)
	}
}

func TestStack_Provenance_OrchestratorKeysAndPairReportThePairsLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestratorPair(t, high, "9.0.0")
	addTestOrchestratorPair(t, low, "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	for _, item := range []domain.CatalogItem{agentItem("orchestrator"), agentItem("orchestrator-script"), pairItem()} {
		got, ok := c.Provenance(item)

		if !ok {
			t.Errorf("Provenance(%+v) not found", item)
			continue
		}
		if want := testLayer(high, "high"); got != want {
			t.Errorf("Provenance(%+v) = %+v, want the highest pair-supplying layer %+v", item, got, want)
		}
	}
}

func TestStack_Provenance_PairComesFromLowerLayerWhenTopSuppliesNone(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "1.0.0")
	addTestOrchestratorPair(t, low, "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	for _, item := range []domain.CatalogItem{agentItem("orchestrator"), agentItem("orchestrator-script"), pairItem()} {
		got, ok := c.Provenance(item)

		if !ok || got.Identity.ID != "low" || got.Root != low {
			t.Errorf("Provenance(%+v) = %+v, %v, want the low layer", item, got, ok)
		}
	}
}

func TestStack_Provenance_UnknownItemsAreNotFound(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "1.0.0")
	addTestSkill(t, low, "a-skill", "1.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	cases := map[string]domain.CatalogItem{
		"unknown agent":               agentItem("nope"),
		"unknown workflow":            {Kind: domain.CatalogItemWorkflow, Key: "nope"},
		"unknown skill":               {Kind: domain.CatalogItemSkill, Key: "nope"},
		"unknown hook":                {Kind: domain.CatalogItemHook, Key: "nope"},
		"skill key asked as agent":    agentItem("a-skill"),
		"agent key asked as skill":    {Kind: domain.CatalogItemSkill, Key: "worker"},
		"orchestrator with no pair":   agentItem("orchestrator"),
		"script orchestrator no pair": agentItem("orchestrator-script"),
		"pair item with no pair":      pairItem(),
	}
	for name, item := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := c.Provenance(item)

			if ok {
				t.Errorf("Provenance(%+v) = %+v, want not found", item, got)
			}
		})
	}
}
