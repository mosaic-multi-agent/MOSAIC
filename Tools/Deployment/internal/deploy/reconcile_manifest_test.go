package deploy_test

// reconcile_manifest_test.go specifies how the executor treats the manifest it loads itself:
// it reconciles it with the disk, so an entry whose files are gone is never written back, and
// the manifest it saves is the reconciled prior state plus the run's own changes. A dry run
// builds the same result in memory and leaves the manifest file untouched.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	rcRunHarness   = "test-harness"
	rcOtherHarness = "other-harness"
)

func rcEntry(harness string, kind domain.ArtifactKind, key, path string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:     harness,
		Ref:         domain.ArtifactRef{Kind: kind, Key: key},
		TargetPath:  path,
		Version:     "1.0.0",
		ContentHash: manifest.Hash([]byte(path)),
	}
}

func rcWrite(t *testing.T, ws, rel string) {
	t.Helper()
	p := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("on disk"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// rcSeed saves a manifest for ws made of the given entries (sections derived from them) and
// returns the store that holds it.
func rcSeed(t *testing.T, ws string, entries ...domain.ManifestEntry) manifest.Store {
	t.Helper()
	seen := map[string]bool{}
	var sections []domain.HarnessSection
	for _, e := range entries {
		if !seen[e.Harness] {
			seen[e.Harness] = true
			sections = append(sections, domain.HarnessSection{HarnessID: e.Harness})
		}
	}
	store := manifest.NewStore()
	err := store.Save(ws, domain.Manifest{SchemaVersion: manifest.SchemaVersion, Harnesses: sections, Entries: entries})
	if err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return store
}

func rcRun(t *testing.T, ws string, store manifest.Store, dry bool) deploy.ExecResult {
	t.Helper()
	req := deploy.ExecRequest{
		Plan:       newPlan(ws, newAgentItem("runner", "agents/runner.md", domain.ActionCreate)),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte("# runner")),
		DryRun:     dry,
	}
	res, err := deploy.NewExecutor(store, newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

func rcHasPath(m domain.Manifest, path string) bool {
	for _, e := range m.Entries {
		if e.TargetPath == path {
			return true
		}
	}
	return false
}

func rcLoad(t *testing.T, ws string) domain.Manifest {
	t.Helper()
	snap, err := manifest.NewStore().Load(ws)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	return snap.Manifest
}

// rcSeedMixed seeds a manifest of two harnesses: the run harness records one present and one
// missing agent, the other harness records one missing agent and, optionally, one present.
func rcSeedMixed(t *testing.T, ws string, otherHasFile bool) manifest.Store {
	t.Helper()
	rcWrite(t, ws, "agents/present.md")
	entries := []domain.ManifestEntry{
		rcEntry(rcRunHarness, domain.ArtifactAgent, "present", "agents/present.md"),
		rcEntry(rcRunHarness, domain.ArtifactAgent, "ghost", "agents/ghost.md"),
		rcEntry(rcOtherHarness, domain.ArtifactAgent, "other-ghost", "other/ghost.md"),
	}
	if otherHasFile {
		rcWrite(t, ws, "other/present.md")
		entries = append(entries, rcEntry(rcOtherHarness, domain.ArtifactAgent, "other-present", "other/present.md"))
	}
	return rcSeed(t, ws, entries...)
}

func TestReconcileExecutor_ExecutedRun_SavesTheReconciledManifestPlusItsOwnEntries(t *testing.T) {
	ws := t.TempDir()
	store := rcSeedMixed(t, ws, true)

	res := rcRun(t, ws, store, false)

	for name, m := range map[string]domain.Manifest{"saved manifest": rcLoad(t, ws), "result manifest": res.Manifest} {
		for _, gone := range []string{"agents/ghost.md", "other/ghost.md"} {
			if rcHasPath(m, gone) {
				t.Errorf("%s still records the missing file %s", name, gone)
			}
		}
		for _, kept := range []string{"agents/present.md", "other/present.md", "agents/runner.md"} {
			if !rcHasPath(m, kept) {
				t.Errorf("%s lacks the entry of %s", name, kept)
			}
		}
	}
}

func TestReconcileExecutor_HarnessWithEveryEntryDropped_LeavesNoEntryAndNoSection(t *testing.T) {
	ws := t.TempDir()
	store := rcSeedMixed(t, ws, false)

	rcRun(t, ws, store, false)

	m := rcLoad(t, ws)
	if m.Holds(rcOtherHarness) {
		t.Errorf("harness %s is still held after all its files were found missing", rcOtherHarness)
	}
	if _, ok := m.Section(rcOtherHarness); ok {
		t.Errorf("the section of %s was written back", rcOtherHarness)
	}
	if !m.Holds(rcRunHarness) {
		t.Errorf("the run harness %s is no longer held", rcRunHarness)
	}
}

func TestReconcileExecutor_EntriesOfOtherHarnessesWhoseFilesExist_AreCarriedUnchanged(t *testing.T) {
	ws := t.TempDir()
	store := rcSeedMixed(t, ws, true)
	before, _ := rcLoad(t, ws).Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "other-present"})

	rcRun(t, ws, store, false)

	after, ok := rcLoad(t, ws).Lookup(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "other-present"})
	if !ok || after != before {
		t.Errorf("entry of the other harness changed: before %+v, after %+v (found %v)", before, after, ok)
	}
}

func TestReconcileExecutor_PartlyMissingHookBundle_IsKeptByTheRun(t *testing.T) {
	ws := t.TempDir()
	rcWrite(t, ws, "hooks/logger.json")
	store := rcSeed(t, ws,
		rcEntry(rcOtherHarness, domain.ArtifactHook, "logger", "hooks/logger.json"),
		rcEntry(rcOtherHarness, domain.ArtifactHook, "logger", "hooks/logger.ps1"))

	rcRun(t, ws, store, false)

	m := rcLoad(t, ws)
	for _, p := range []string{"hooks/logger.json", "hooks/logger.ps1"} {
		if !rcHasPath(m, p) {
			t.Errorf("the damaged bundle lost its entry for %s", p)
		}
	}
}

func TestReconcileExecutor_BundleLevelHookEntryWithAbsentPath_IsKeptByTheRun(t *testing.T) {
	ws := t.TempDir()
	legacy := rcEntry(rcOtherHarness, domain.ArtifactHook, "logger", "hooks/logger")
	legacy.ContentHash = manifest.Hash(nil)
	store := rcSeed(t, ws, legacy)

	rcRun(t, ws, store, false)

	if !rcHasPath(rcLoad(t, ws), "hooks/logger") {
		t.Error("the bundle-level hook entry was dropped for its absent path")
	}
}

func TestReconcileExecutor_DryRun_LeavesTheManifestFileByteIdenticalAndReportsTheReconciledState(t *testing.T) {
	ws := t.TempDir()
	store := rcSeedMixed(t, ws, true)
	file := filepath.Join(ws, ".mosaic", "manifest.yaml")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read seeded manifest: %v", err)
	}

	res := rcRun(t, ws, store, true)

	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read manifest after dry run: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("a dry run changed the manifest file")
	}
	for _, gone := range []string{"agents/ghost.md", "other/ghost.md"} {
		if rcHasPath(res.Manifest, gone) {
			t.Errorf("the dry-run result manifest still records the missing file %s", gone)
		}
	}
	if !rcHasPath(res.Manifest, "agents/runner.md") || !rcHasPath(res.Manifest, "agents/present.md") {
		t.Errorf("the dry-run result manifest lacks the run's entry or a present entry: %+v", res.Manifest.Entries)
	}
}
