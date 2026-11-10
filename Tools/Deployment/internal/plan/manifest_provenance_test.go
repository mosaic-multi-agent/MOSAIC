package plan_test

// manifest_provenance_test.go verifies that catalog provenance in the manifest does not change
// how the plan classifies a file when the catalog in effect reports no provenance (a catalog that
// is not a stack). A manifest written before provenance existed has no catalog information on its
// entries; that absence must not turn an unchanged file into a conflict or an update, and the
// classification must equal the one for the same manifest with provenance. A recorded catalog
// that differs from the supplier of a stack catalog is covered by catalog_staleness_test.go.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

func classifyHookWithEntryCatalog(t *testing.T, catalog domain.CatalogIdentity) domain.PlanItem {
	t.Helper()
	const hookTarget = "hooks/pre-commit"
	hook := makeHookBundle("pre-commit", "1.0")
	entry := hookManifestEntry(hook, hookTarget)
	entry.Catalog = catalog
	snap := presentSnapshot(domain.Manifest{Entries: []domain.ManifestEntry{entry}})
	deployed := map[string]domain.DeployedArtifactState{hookTarget: hookProbeState()}

	p, err := plan.New().Build(context.Background(), buildInputWithHook(hook, hookTarget, snap, deployed))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	item, ok := findItem(p.Items, "pre-commit")
	if !ok {
		t.Fatal("plan has no item for hook pre-commit")
	}
	return item
}

func classifyAgentWithEntryCatalog(t *testing.T, catalog domain.CatalogIdentity) domain.PlanItem {
	t.Helper()
	const targetPath = "agents/test-agent.md"
	const hash = "sha256:abc123"
	agent := makeAgent("test-agent", "1.0")
	entry := makeManifestEntry(agentRef("test-agent"), targetPath, "1.0", hash)
	entry.HarnessVersion = "1.0"
	entry.InjectionsVersion = "1.0"
	entry.Catalog = catalog
	snap := presentSnapshot(domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		HarnessID:     "test-harness",
		Entries:       []domain.ManifestEntry{entry},
	})
	deployed := map[string]domain.DeployedArtifactState{targetPath: deployedState(hash, "1.0", "1.0", "1.0")}

	p, err := plan.New().Build(context.Background(), buildInputWithSingleAgent(agent, snap, deployed))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	item, ok := findItem(p.Items, "test-agent")
	if !ok {
		t.Fatal("plan has no item for test-agent")
	}
	return item
}

// TestBuild_AgentEntryWithoutCatalog_IsNotAConflict verifies the same guarantee for an agent entry
// recorded by a pre-provenance manifest, and that a recorded catalog changes nothing.
func TestBuild_AgentEntryWithoutCatalog_IsNotAConflict(t *testing.T) {
	without := classifyAgentWithEntryCatalog(t, domain.CatalogIdentity{})
	with := classifyAgentWithEntryCatalog(t, domain.CatalogIdentity{ID: "user-cat", Name: "User", Version: "9.9"})

	if without.Action != domain.ActionUnchanged || without.Conflict != nil {
		t.Errorf("agent entry without catalog: Action %q, Conflict %+v, want unchanged without conflict",
			without.Action, without.Conflict)
	}
	if with.Action != without.Action || (with.Conflict == nil) != (without.Conflict == nil) {
		t.Errorf("agent entry with catalog classifies as %q / %v, differs from without (%q / %v)",
			with.Action, with.Conflict, without.Action, without.Conflict)
	}
}

// TestBuild_ManifestEntryWithoutCatalog_IsNotAConflict verifies an unchanged file recorded by a
// pre-provenance manifest is classified unchanged, not as a conflict.
func TestBuild_ManifestEntryWithoutCatalog_IsNotAConflict(t *testing.T) {
	item := classifyHookWithEntryCatalog(t, domain.CatalogIdentity{})

	if item.Action != domain.ActionUnchanged {
		t.Errorf("Action = %q for an entry without catalog information, want %q", item.Action, domain.ActionUnchanged)
	}
	if item.Conflict != nil {
		t.Errorf("Conflict = %+v, want nil: missing catalog information is not a local modification", item.Conflict)
	}
}

// TestBuild_ManifestEntryCatalog_DoesNotChangeClassification verifies the same file classifies
// the same whether or not its entry records a catalog, whichever catalog that is.
func TestBuild_ManifestEntryCatalog_DoesNotChangeClassification(t *testing.T) {
	without := classifyHookWithEntryCatalog(t, domain.CatalogIdentity{})
	for _, cat := range []domain.CatalogIdentity{
		{ID: "mosaic-default", Version: "0.7.0"},
		{ID: "user-cat", Name: "User", Version: "9.9"},
	} {
		with := classifyHookWithEntryCatalog(t, cat)
		if with.Action != without.Action || (with.Conflict == nil) != (without.Conflict == nil) {
			t.Errorf("catalog %+v: Action %q / conflict %v differs from the entry without catalog (%q / %v)",
				cat, with.Action, with.Conflict, without.Action, without.Conflict)
		}
	}
}
