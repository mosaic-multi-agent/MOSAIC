package app_test

// stack_catalog_of_test.go verifies the provenance method of the stack service interface: with
// a stack catalog it returns the layer that supplies an agent, workflow or hook (the highest
// layer that has it), answers both orchestrator agent keys with the layer that supplies the
// orchestrator pair, and reports not found for an item no layer has and for a catalog that is
// not a stack.

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

const (
	cofUserID    = "cof-user"
	cofDefaultID = "cof-default"
)

// cofWriteWorkflow writes a workflow source into a catalog root.
func cofWriteWorkflow(t *testing.T, root, id string) {
	t.Helper()
	writeSysFile(t, filepath.Join(root, "Workflows", "Build", id+".md"),
		"---\nid: "+id+"\nversion: \"1.0\"\nname: "+id+"\ndescription: A workflow.\nhint: Hint.\n---\nBody.\n")
}

// cofWriteHook writes a hook bundle with one supported variant into a catalog root.
func cofWriteHook(t *testing.T, root, id string) {
	t.Helper()
	writeSysFile(t, filepath.Join(root, "Hooks", id, "hook.yaml"),
		"schema_version: \"1\"\nid: "+id+"\nversion: 1.0.0\ndescription: Hook "+id+".\nplaceholder: false\nvariants:\n"+
			"  h1:\n    supported: true\n    files:\n      - source: h1-hook.sh\n        target: h1-hook.sh\n    registration: []\n")
	writeSysFile(t, filepath.Join(root, "Hooks", id, "h1", "h1-hook.sh"), "#!/bin/sh\n")
}

// cofFixture is a two-layer stack: the user layer over the default layer. Each layer supplies
// items of its own plus a shared agent, workflow and hook, and both supply the orchestrator pair.
type cofFixture struct {
	svc        app.Service
	user, dflt domain.CatalogLayer
}

func newCofFixture(t *testing.T) cofFixture {
	t.Helper()
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	mosaic := newSysMosaicRoot(t, sysLayer{id: cofDefaultID})
	user := newSysUserLayer(t, sysLayer{id: cofUserID})
	dfltRoot := filepath.Join(mosaic, catalogpaths.CatalogDirName)
	for _, root := range []string{user, dfltRoot} {
		crWriteOrchestratorPair(t, root)
		crWriteAgent(t, root, "cof-shared-agent", "1.0")
		cofWriteWorkflow(t, root, "cof-shared-flow")
		cofWriteHook(t, root, "cof-shared-hook")
	}
	crWriteAgent(t, user, "cof-user-agent", "1.0")
	cofWriteWorkflow(t, user, "cof-user-flow")
	cofWriteHook(t, user, "cof-user-hook")
	crWriteAgent(t, dfltRoot, "cof-default-agent", "1.0")
	cofWriteWorkflow(t, dfltRoot, "cof-default-flow")
	cofWriteHook(t, dfltRoot, "cof-default-hook")
	useResolvedStack(t, &deps, mosaic, stack.Layer{Root: user}, defaultStackLayer(mosaic))
	svc := app.New(deps)
	layers := svc.ActiveStack().Layers
	if len(layers) != 2 {
		t.Fatalf("fixture stack has %d layers, want 2", len(layers))
	}
	return cofFixture{svc: svc, user: layers[0], dflt: layers[1]}
}

// TestCatalogOf_StackCatalog_ReturnsTheSupplyingLayerOfEachKind verifies that agents,
// workflows and hooks are attributed to the layer that supplies them.
func TestCatalogOf_StackCatalog_ReturnsTheSupplyingLayerOfEachKind(t *testing.T) {
	f := newCofFixture(t)
	cases := []struct {
		name string
		item domain.CatalogItem
		want func() domain.CatalogLayer
	}{
		{"agent only in user layer", domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "cof-user-agent"}, func() domain.CatalogLayer { return f.user }},
		{"agent only in default layer", domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "cof-default-agent"}, func() domain.CatalogLayer { return f.dflt }},
		{"workflow only in user layer", domain.CatalogItem{Kind: domain.CatalogItemWorkflow, Key: "cof-user-flow"}, func() domain.CatalogLayer { return f.user }},
		{"workflow only in default layer", domain.CatalogItem{Kind: domain.CatalogItemWorkflow, Key: "cof-default-flow"}, func() domain.CatalogLayer { return f.dflt }},
		{"hook only in user layer", domain.CatalogItem{Kind: domain.CatalogItemHook, Key: "cof-user-hook"}, func() domain.CatalogLayer { return f.user }},
		{"hook only in default layer", domain.CatalogItem{Kind: domain.CatalogItemHook, Key: "cof-default-hook"}, func() domain.CatalogLayer { return f.dflt }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := f.svc.CatalogOf(c.item)

			if !ok {
				t.Fatalf("CatalogOf(%+v) not found, want the supplying layer", c.item)
			}
			if got != c.want() {
				t.Errorf("CatalogOf(%+v) = %+v, want %+v", c.item, got, c.want())
			}
		})
	}
}

// TestCatalogOf_ItemInSeveralLayers_ReturnsTheWinningLayer verifies that an item both layers
// supply is attributed to the higher one.
func TestCatalogOf_ItemInSeveralLayers_ReturnsTheWinningLayer(t *testing.T) {
	f := newCofFixture(t)
	for _, item := range []domain.CatalogItem{
		{Kind: domain.CatalogItemAgent, Key: "cof-shared-agent"},
		{Kind: domain.CatalogItemWorkflow, Key: "cof-shared-flow"},
		{Kind: domain.CatalogItemHook, Key: "cof-shared-hook"},
	} {
		got, ok := f.svc.CatalogOf(item)

		if !ok || got != f.user {
			t.Errorf("CatalogOf(%+v) = %+v, %v; want the user layer %+v", item, got, ok, f.user)
		}
	}
}

// TestCatalogOf_OrchestratorAgentKeys_ReturnTheLayerOfThePair verifies that both orchestrator
// agent keys answer with the layer that supplies the orchestrator pair.
func TestCatalogOf_OrchestratorAgentKeys_ReturnTheLayerOfThePair(t *testing.T) {
	f := newCofFixture(t)
	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		item := domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: key}

		got, ok := f.svc.CatalogOf(item)

		if !ok || got != f.user {
			t.Errorf("CatalogOf(%+v) = %+v, %v; want the user layer %+v that supplies the pair", item, got, ok, f.user)
		}
	}
}

// TestCatalogOf_UnknownItem_ReportsNotFound verifies that an item no layer contains, and an
// existing key under the wrong kind, are not found.
func TestCatalogOf_UnknownItem_ReportsNotFound(t *testing.T) {
	f := newCofFixture(t)
	for _, item := range []domain.CatalogItem{
		{Kind: domain.CatalogItemAgent, Key: "no-such-agent"},
		{Kind: domain.CatalogItemWorkflow, Key: "no-such-flow"},
		{Kind: domain.CatalogItemHook, Key: "no-such-hook"},
		{Kind: domain.CatalogItemWorkflow, Key: "cof-user-agent"},
	} {
		if got, ok := f.svc.CatalogOf(item); ok {
			t.Errorf("CatalogOf(%+v) = %+v, true; want not found", item, got)
		}
	}
}

// TestCatalogOf_PlainCatalog_ReportsNotFound verifies that a catalog that is not a stack
// attributes nothing, even to an item it does contain.
func TestCatalogOf_PlainCatalog_ReportsNotFound(t *testing.T) {
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	svc := app.New(deps)

	for _, item := range []domain.CatalogItem{
		{Kind: domain.CatalogItemAgent, Key: crAgentKey},
		{Kind: domain.CatalogItemWorkflow, Key: "quick-fix"},
	} {
		if got, ok := svc.CatalogOf(item); ok {
			t.Errorf("CatalogOf(%+v) = %+v, true; want not found for a plain catalog", item, got)
		}
	}
}
