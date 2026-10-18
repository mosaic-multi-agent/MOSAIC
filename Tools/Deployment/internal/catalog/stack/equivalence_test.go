package stack_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

// newTestMosaicShapedFixture builds a MOSAIC-root-shaped tree with a representative mix of
// content: every agent role, both orchestrators, workflows in two categories, a skill with a
// nested file, a hook with two variants, an issue-producing hook, and legacy-path items.
func newTestMosaicShapedFixture(t *testing.T) (mosaicRoot, catalogRoot string) {
	t.Helper()
	mosaicRoot = newTestMosaicRoot(t)
	catalogRoot = filepath.Join(mosaicRoot, "Catalog")
	writeTestIdentity(t, catalogRoot, "mosaic")

	addTestOrchestrator(t, catalogRoot, "1.0.0")
	addTestOrchestratorScript(t, catalogRoot, "1.0.0")
	addTestSubagentWithTier(t, catalogRoot, "Build", "builder", "1.0.0", "deep")
	addTestSubagentWithTier(t, catalogRoot, "Review", "reviewer", "1.0.0", "fast")
	addTestSubagent(t, catalogRoot, "Build", "committer", "1.0.0", "infrastructure: commit\non_failure: halt\n")
	addTestUtility(t, catalogRoot, "creator", "1.0.0")
	addTestStandalone(t, catalogRoot, "solo", "1.0.0")
	addTestWorkflow(t, catalogRoot, "Build", "feature", "1.0.0", "builder", "reviewer")
	addTestWorkflow(t, catalogRoot, "Ship", "release", "1.0.0", "committer")
	addTestSkill(t, catalogRoot, "lean", "1.0.0", filepath.Join("refs", "more.md"))
	addTestHook(t, catalogRoot, "logger", "claude-code", "opencode")

	// Legacy-path items the single-root loader still reads.
	mustWrite(t, catalogRoot, filepath.Join("Agents", "Generic", "Agents", "Legacy", "old-worker.md"),
		"---\nid: \"old-id\"\nversion: \"1.0.0\"\nname: old-worker\nrole: subagent\n---\nBody.\n")
	mustWrite(t, catalogRoot, filepath.Join("Agents", "Generic", "Skills", "old-skill", "SKILL.md"),
		"---\nname: old-skill\ndescription: d\nversion: \"1.0.0\"\n---\nBody.\n")

	// A hook whose stored hash is wrong, so both loads must report the same issue.
	mustWrite(t, catalogRoot, filepath.Join("Hooks", "tampered", "hook.yaml"),
		"schema_version: \"1\"\nid: tampered\nversion: 1.0.0\ndescription: d\nplaceholder: false\n"+
			"content_hash: \"sha256:0000000000000000000000000000000000000000000000000000000000000000\"\n"+
			"variants:\n  h1:\n    supported: true\n    files:\n      - source: f.sh\n        target: f.sh\n    registration: []\n")
	mustWrite(t, catalogRoot, filepath.Join("Hooks", "tampered", "h1", "f.sh"), "#!/bin/sh\n")
	return mosaicRoot, catalogRoot
}

// withoutAttribution returns a copy of issues with the catalog attribution cleared, so they can
// be compared with the unattributed issues of a plain load.
func withoutAttribution(issues []catalog.Issue) []catalog.Issue {
	out := make([]catalog.Issue, len(issues))
	for i, iss := range issues {
		iss.Catalog = domain.CatalogIdentity{}
		out[i] = iss
	}
	return out
}

func TestStack_OneLayer_ResolvesSameItemsAsCatalogLoad(t *testing.T) {
	mosaic, catRoot := newTestMosaicShapedFixture(t)
	want, err := catalog.Load(mosaic, "")
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	if len(want.Issues()) == 0 {
		t.Fatal("fixture precondition: expected Load to report the tampered-hook issue")
	}

	got, err := stack.New(mosaic, []stack.Layer{{Root: catRoot, Default: true}})
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}

	checks := map[string][2]any{
		"Agents":               {want.Agents(), got.Agents()},
		"UtilityAgents":        {want.UtilityAgents(), got.UtilityAgents()},
		"StandaloneAgents":     {want.StandaloneAgents(), got.StandaloneAgents()},
		"InfrastructureAgents": {want.InfrastructureAgents(), got.InfrastructureAgents()},
		"Orchestrator":         {want.Orchestrator(), got.Orchestrator()},
		"Skills":               {want.Skills(), got.Skills()},
		"Hooks":                {want.Hooks(), got.Hooks()},
		"Workflows":            {want.Workflows(), got.Workflows()},
		"WorkflowCategories":   {want.WorkflowCategories(), got.WorkflowCategories()},
		"Tiers":                {want.Tiers(), got.Tiers()},
		"Issues":               {want.Issues(), withoutAttribution(got.Issues())},
		"Root":                 {want.Root(), got.Root()},
		"CatalogRoot":          {want.CatalogRoot(), got.CatalogRoot()},
	}
	for name, pair := range checks {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s differs:\n  Load:  %+v\n  stack: %+v", name, pair[0], pair[1])
		}
	}
	ws, wok := want.OrchestratorScript()
	gs, gok := got.OrchestratorScript()
	if wok != gok || !reflect.DeepEqual(ws, gs) {
		t.Errorf("OrchestratorScript differs: Load %+v,%v stack %+v,%v", ws, wok, gs, gok)
	}
	for _, key := range []string{"builder", "old-worker", "creator", "solo", "orchestrator", "orchestrator-script"} {
		w, wok := want.Agent(key)
		g, gok := got.Agent(key)
		if wok != gok || !reflect.DeepEqual(w, g) {
			t.Errorf("Agent(%q) differs: Load %+v,%v stack %+v,%v", key, w, wok, g, gok)
		}
	}
	if g, ok := got.AgentByNumericID("old-id"); !ok || g.Key != "old-worker" {
		t.Errorf("AgentByNumericID(old-id) = %+v, %v", g, ok)
	}
}
