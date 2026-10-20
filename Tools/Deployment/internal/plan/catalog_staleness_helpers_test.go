package plan_test

// catalog_staleness_helpers_test.go holds the fixtures of the catalog-change staleness tests: a
// catalog that answers provenance per item, and one plan input holding an agent, both
// orchestrator files, a two-file skill and a hook, all deployed and recorded in the manifest
// under a chosen catalog. Nothing in this file is a test itself.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

// Catalog ids used by the catalog-change tests.
const (
	ccOldCatalogID = "orchonly"
	ccNewCatalogID = "mosaic-default"

	// ccCatalogDeltaField is the delta field the catalog change contributes.
	ccCatalogDeltaField = "source_catalog"
)

// Target paths of the planned items, as the fixture module computes them.
const (
	ccWorkerTarget       = "agents/worker.md"
	ccOrchTarget         = "agents/orchestrator.md"
	ccOrchScriptTarget   = "agents/orchestrator-script.md"
	ccSkillEntryTarget   = "skills/lean-tdd/SKILL.md"
	ccSkillCompanionPath = "skills/lean-tdd/companion.md"
	ccHookTarget         = "hooks/pre-commit"

	ccDeployedHash = "sha256:cc-deployed"
)

// provenanceCatalog wraps the plan test catalog with a Provenance method that answers from a
// fixed table. An item missing from the table answers ok == false.
type provenanceCatalog struct {
	*fakeCatalog
	layers map[domain.CatalogItem]domain.CatalogLayer
}

func (c *provenanceCatalog) Provenance(item domain.CatalogItem) (domain.CatalogLayer, bool) {
	l, ok := c.layers[item]
	return l, ok
}

var _ catalog.ProvenanceReporter = (*provenanceCatalog)(nil)

// ccItems are the catalog items the fixture plans, in provenance-table form.
func ccItems() []domain.CatalogItem {
	return []domain.CatalogItem{
		{Kind: domain.CatalogItemAgent, Key: "worker"},
		{Kind: domain.CatalogItemAgent, Key: "orchestrator"},
		{Kind: domain.CatalogItemAgent, Key: "orchestrator-script"},
		{Kind: domain.CatalogItemSkill, Key: "lean-tdd"},
		{Kind: domain.CatalogItemHook, Key: "pre-commit"},
	}
}

// ccSupplier returns a provenance table answering id for every planned item.
func ccSupplier(id string) map[domain.CatalogItem]domain.CatalogLayer {
	layer := domain.CatalogLayer{Identity: domain.CatalogIdentity{ID: id, Name: id + " name", Version: "1.0.0"}}
	table := map[domain.CatalogItem]domain.CatalogLayer{}
	for _, it := range ccItems() {
		table[it] = layer
	}
	return table
}

// ccCatalogOptions varies the fixture.
type ccCatalogOptions struct {
	// recorded is the catalog stored in every manifest entry.
	recorded domain.CatalogIdentity
	// hookDeployedHash overrides the deployed hook content hash; "" keeps it equal to the
	// recorded hash (an unmodified hook).
	hookDeployedHash string
	// workerSourceVersion overrides the worker's catalog version; "" keeps it equal to the
	// recorded version.
	workerSourceVersion string
	// skillSourceVersion overrides the skill's catalog version; "" keeps it equal to the
	// recorded version.
	skillSourceVersion string
	// hookSourceVersion overrides the hook's catalog version; "" keeps it equal to the recorded
	// version.
	hookSourceVersion string
}

// ccFakeCatalog returns the plain catalog holding the planned items.
func ccFakeCatalog(o ccCatalogOptions) *fakeCatalog {
	worker := makeAgent("worker", "1.0")
	if o.workerSourceVersion != "" {
		worker.Version = o.workerSourceVersion
	}
	worker.RequiredSkills = []string{"lean-tdd"}
	skill := makeMultiFileSkill("lean-tdd", []string{"SKILL.md", "companion.md"})
	if o.skillSourceVersion != "" {
		skill.Version = o.skillSourceVersion
	}
	hookVersion := "1.0"
	if o.hookSourceVersion != "" {
		hookVersion = o.hookSourceVersion
	}
	return &fakeCatalog{
		orchestrator:         makeOrchestrator(),
		orchestratorScript:   makeOrchestratorScriptAgent(),
		orchestratorScriptOK: true,
		workers:              []domain.Agent{worker},
		skills:               []domain.Skill{skill},
		hooks:                []domain.HookBundle{makeHookBundle("pre-commit", hookVersion)},
		workflows:            []domain.Workflow{makeWorkflow("test-wf", "worker")},
	}
}

// ccModule maps agents, skill files and hooks to the fixture target paths.
func ccModule() *fakeModule {
	mod := newFakeModule()
	mod.TargetPathFn = func(req domain.TargetPathRequest) (string, error) {
		switch req.Kind {
		case domain.ArtifactAgent:
			return "agents/" + req.Key + ".md", nil
		case domain.ArtifactSkill:
			return "skills/" + req.Key + "/" + req.FileName, nil
		case domain.ArtifactHook:
			return "hooks/" + req.Key, nil
		}
		return "", domain.ErrArtifactUnsupported
	}
	return mod
}

// ccManifestEntries returns one entry per planned file, all naming recorded as their catalog and
// all at the version and hash the deployed files carry.
func ccManifestEntries(recorded domain.CatalogIdentity) []domain.ManifestEntry {
	agentEntry := func(key, target string) domain.ManifestEntry {
		e := makeManifestEntry(agentRef(key), target, "1.0", ccDeployedHash)
		e.Catalog = recorded
		return e
	}
	skillEntry := func(target string) domain.ManifestEntry {
		e := makeManifestEntry(skillRef("lean-tdd"), target, "1.0", ccDeployedHash)
		e.Catalog = recorded
		return e
	}
	hookEntry := hookManifestEntry(makeHookBundle("pre-commit", "1.0"), ccHookTarget)
	hookEntry.Catalog = recorded
	return []domain.ManifestEntry{
		agentEntry("worker", ccWorkerTarget),
		agentEntry("orchestrator", ccOrchTarget),
		agentEntry("orchestrator-script", ccOrchScriptTarget),
		skillEntry(ccSkillEntryTarget),
		skillEntry(ccSkillCompanionPath),
		hookEntry,
	}
}

// ccDeployedStates returns the deployed state of every planned file: current by every rule other
// than catalog provenance.
func ccDeployedStates(o ccCatalogOptions) map[string]domain.DeployedArtifactState {
	orch := deployedState(ccDeployedHash, "1.0", "1.0", "1.0")
	orch.Workflows = makeDeployedWorkflows("test-wf", "1.0")
	hook := hookProbeState()
	if o.hookDeployedHash != "" {
		hook.ContentHash = o.hookDeployedHash
	}
	return map[string]domain.DeployedArtifactState{
		ccWorkerTarget:       deployedState(ccDeployedHash, "1.0", "1.0", "1.0"),
		ccOrchTarget:         orch,
		ccOrchScriptTarget:   orch,
		ccSkillEntryTarget:   deployedState(ccDeployedHash, "1.0", "1.0", "1.0"),
		ccSkillCompanionPath: deployedState(ccDeployedHash, "", "", ""),
		ccHookTarget:         hook,
	}
}

// ccBuild plans the fixture over cat. Every planned file is deployed and recorded under
// o.recorded.
func ccBuild(t *testing.T, cat catalog.Catalog, o ccCatalogOptions) domain.Plan {
	t.Helper()
	return ccBuildWith(t, cat, ccDeployedStates(o), ccManifestEntries(o.recorded))
}

// ccBuildWith plans the fixture over cat with explicit deployed states and manifest entries.
// tweaks adjust the plan input before planning.
func ccBuildWith(t *testing.T, cat catalog.Catalog, states map[string]domain.DeployedArtifactState, entries []domain.ManifestEntry, tweaks ...func(*plan.Input)) domain.Plan {
	t.Helper()
	snap := presentSnapshot(domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		HarnessID:     "test-harness",
		Entries:       entries,
	})
	in := plan.Input{
		Catalog:       cat,
		Module:        ccModule(),
		Mode:          domain.ModeDeployWorkspace,
		WorkspacePath: "/fake/workspace",
		Scope:         domain.ScopeProject,
		GOOS:          "linux",
		Manifest:      snap,
		WorkflowIDs:   []string{"test-wf"},
		HookIDs:       []string{"pre-commit"},
		Models: map[string]domain.ModelSelection{
			"worker":              {ModelID: "test-model", Origin: domain.OriginHarnessList},
			"orchestrator":        {ModelID: "test-model", Origin: domain.OriginHarnessList},
			"orchestrator-script": {ModelID: "test-model", Origin: domain.OriginHarnessList},
		},
		DeployedState: states,
	}
	for _, tweak := range tweaks {
		tweak(&in)
	}
	p, err := plan.New().Build(context.Background(), in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return p
}

// ccStackBuild plans the fixture over a provenance-reporting catalog whose every item is
// supplied by supplier.
func ccStackBuild(t *testing.T, supplier string, o ccCatalogOptions) domain.Plan {
	t.Helper()
	cat := &provenanceCatalog{fakeCatalog: ccFakeCatalog(o), layers: ccSupplier(supplier)}
	return ccBuild(t, cat, o)
}

// ccItemAt returns the plan item for target, failing the test when the plan has none.
func ccItemAt(t *testing.T, p domain.Plan, target string) domain.PlanItem {
	t.Helper()
	for _, it := range p.Items {
		if it.TargetPath == target {
			return it
		}
	}
	t.Fatalf("plan has no item for target %q", target)
	return domain.PlanItem{}
}

// ccPlannedTargets lists every target the fixture plans.
func ccPlannedTargets() []string {
	return []string{
		ccWorkerTarget, ccOrchTarget, ccOrchScriptTarget,
		ccSkillEntryTarget, ccSkillCompanionPath, ccHookTarget,
	}
}

// ccCatalogDelta returns the catalog delta of item, if it carries one.
func ccCatalogDelta(item domain.PlanItem) (domain.VersionDelta, bool) {
	for _, d := range item.Stale {
		if d.Field == ccCatalogDeltaField {
			return d, true
		}
	}
	return domain.VersionDelta{}, false
}
