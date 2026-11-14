package app_test

// transform_source_record_helpers_test.go holds the shared builders of the transform tests that
// look at the source deployment's manifest record: a source deployment of agents, a skill and a
// hook bundle whose manifest entries each carry their own catalog identity, and readers of the
// per-harness view of the manifest after the run. Nothing in this file is a test itself.

import (
	"path/filepath"
	"sort"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// trSourceTime is the time every seeded source entry and section was recorded at.
var trSourceTime = time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)

// trRecordedAgents are the agent fixtures whose source entries carry a catalog identity.
var trRecordedAgents = []string{"subagent", "utility"}

// trCatalogOf returns the catalog identity the source entry named name carries. Every name
// yields a different id, name and version, so a lookup that hits the wrong entry shows.
func trCatalogOf(name string) domain.CatalogIdentity {
	return domain.CatalogIdentity{ID: "cat-" + name, Name: "Catalog " + name, Version: "v-" + name}
}

// trHookFilePaths returns the workspace-relative paths of the files the harness's variant of the
// hook bundle key writes.
func trHookFilePaths(t *testing.T, r *trRun, harnessID, key string) []string {
	t.Helper()
	for _, b := range trHookBundles(t) {
		if b.Key != key {
			continue
		}
		hp, err := r.module(harnessID).HookPlan(domain.HookPlanRequest{Bundle: b, Scope: domain.ScopeProject})
		if err != nil || !hp.Supported {
			t.Fatalf("hook plan of %s for %s: %+v, %v", key, harnessID, hp, err)
		}
		var out []string
		for _, f := range hp.Files {
			out = append(out, hp.TargetDir+"/"+f.TargetName)
		}
		return out
	}
	t.Fatalf("unknown hook bundle %q", key)
	return nil
}

// trSeedRecordedSource installs every agent fixture, the fixture skill and the hook bundle
// trHookKey into the source harness's folders and records them in the workspace manifest under
// the source harness: the agents of trRecordedAgents, every skill file and every hook file, each
// with its own catalog identity (trCatalogOf). The other agents stay unrecorded. It returns the
// recorded entries.
func trSeedRecordedSource(t *testing.T, r *trRun) []domain.ManifestEntry {
	t.Helper()
	r.seed(trAgentFixtures...)
	var entries []domain.ManifestEntry
	add := func(ref domain.ArtifactRef, rel, version string, c domain.CatalogIdentity) {
		entries = append(entries, domain.ManifestEntry{
			Harness: r.src, Ref: ref, TargetPath: rel, Version: version,
			ContentHash: manifest.Hash([]byte(r.read(rel))), DeployedAt: trSourceTime, Catalog: c,
		})
	}
	for _, key := range trRecordedAgents {
		rel := r.agentsDir(r.src) + "/" + key + r.agentExt(r.src)
		add(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key}, rel, trCarriedVersions[key], trCatalogOf("agent-"+key))
	}
	for rel := range r.seedSkill() {
		path := r.skillsDir(r.src) + "/" + trSkillKey + "/" + rel
		add(domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: trSkillKey}, path, trSkillVersion, trCatalogOf("skill-"+filepath.Base(rel)))
	}
	r.seedHookBundle(trHookKey)
	for _, path := range trHookFilePaths(t, r, r.src, trHookKey) {
		add(domain.ArtifactRef{Kind: domain.ArtifactHook, Key: trHookKey}, path, trHookVersion, trCatalogOf("hook"))
	}
	m := domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     []domain.HarnessSection{{HarnessID: r.src, UpdatedAt: trSourceTime}},
		Entries:       entries,
		ToolVersion:   "1.0.0",
	}
	if err := manifest.NewStore().Save(r.ws, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return entries
}

// trSortedIDs returns ids sorted, for comparison with Manifest.HarnessIDs.
func trSortedIDs(ids ...string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

// trEntriesOfHarness returns the manifest entries recorded for one harness.
func trEntriesOfHarness(m domain.Manifest, harnessID string) []domain.ManifestEntry {
	return m.ForHarness(harnessID).Entries
}
