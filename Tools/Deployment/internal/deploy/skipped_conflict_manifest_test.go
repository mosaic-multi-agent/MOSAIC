package deploy_test

// skipped_conflict_manifest_test.go covers the manifest state after a conflict item is
// decided DecisionSkip when the same artifact ref is recorded under several target paths
// (for example one entry per harness for the same agent key).
//
// Rule: the saved manifest holds, at the skipped item's (ref, target path), exactly the
// entry the prior manifest held at that same pair, and no entry otherwise. An entry
// recorded for the same ref under another path is never copied to the skipped path.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	skipPathA = ".claude/agents/runner.md"
	skipPathB = ".github/agents/runner.agent.md"
)

var skipRunnerRef = domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "runner"}

// seedSkipWorkspace writes the existing conflicting file at skipPathB, saves a prior
// manifest holding the given entries, and returns the workspace and store.
func seedSkipWorkspace(t *testing.T, entries ...domain.ManifestEntry) (string, manifest.Store) {
	t.Helper()
	workspace := t.TempDir()
	writeExisting(t, workspace, skipPathB, []byte("locally written content"))
	store := manifest.NewStore()
	prior := domain.Manifest{SchemaVersion: "1", HarnessID: "test-harness", Entries: entries}
	if err := store.Save(workspace, prior); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return workspace, store
}

// executeSkippedConflict runs a single conflict item at skipPathB decided DecisionSkip.
func executeSkippedConflict(t *testing.T, workspace string, store manifest.Store) domain.Manifest {
	t.Helper()
	item := newConflictItem("runner", skipPathB)
	req := deploy.ExecRequest{
		Plan:       newPlan(workspace, item),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte("new content")),
		Conflicts:  map[string]domain.ConflictDecision{skipPathB: domain.DecisionSkip},
	}
	ex := deploy.NewExecutor(store, newSpyLogger(), newSpyCollector())
	result, err := ex.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return result.Manifest
}

func skipEntry(path, hashChar string) domain.ManifestEntry {
	return domain.ManifestEntry{
		Ref:         skipRunnerRef,
		TargetPath:  path,
		ContentHash: "sha256:" + strings.Repeat(hashChar, 64),
	}
}

// countRefAt returns how many entries carry ref at path.
func countRefAt(m domain.Manifest, ref domain.ArtifactRef, path string) int {
	n := 0
	for _, e := range m.Entries {
		if e.Ref == ref && e.TargetPath == path {
			n++
		}
	}
	return n
}

// TestSkippedConflict_OtherPathEntryNotCopiedToSkippedPath verifies that a skipped item
// with no entry of its own gets no entry, even when its ref has an entry at another path.
func TestSkippedConflict_OtherPathEntryNotCopiedToSkippedPath(t *testing.T) {
	workspace, store := seedSkipWorkspace(t, skipEntry(skipPathA, "a"))

	got := executeSkippedConflict(t, workspace, store)

	if _, found := got.LookupAt(skipRunnerRef, skipPathB); found {
		t.Errorf("manifest has an entry at skipped path %q carrying another path's data", skipPathB)
	}
	if len(got.Entries) != 1 {
		t.Errorf("manifest has %d entries; want exactly the one for %q", len(got.Entries), skipPathA)
	}
}

// TestSkippedConflict_OtherPathEntryUnchanged verifies the entry recorded for the same ref
// at another path survives a skip at a different path exactly as it was.
func TestSkippedConflict_OtherPathEntryUnchanged(t *testing.T) {
	other := skipEntry(skipPathA, "a")
	workspace, store := seedSkipWorkspace(t, other)

	got := executeSkippedConflict(t, workspace, store)

	entry, found := got.LookupAt(skipRunnerRef, skipPathA)
	if !found {
		t.Fatalf("entry at %q was lost", skipPathA)
	}
	if entry.ContentHash != other.ContentHash {
		t.Errorf("entry at %q ContentHash = %q; want unchanged %q", skipPathA, entry.ContentHash, other.ContentHash)
	}
	if n := countRefAt(got, skipRunnerRef, skipPathA); n != 1 {
		t.Errorf("entries at %q = %d; want 1", skipPathA, n)
	}
}

// TestSkippedConflict_SavedManifestHasNoDuplicatePair verifies the manifest as saved on
// disk holds each (ref, path) pair at most once and nothing at the skipped path.
func TestSkippedConflict_SavedManifestHasNoDuplicatePair(t *testing.T) {
	workspace, store := seedSkipWorkspace(t, skipEntry(skipPathA, "a"))

	executeSkippedConflict(t, workspace, store)

	snap, err := store.Load(workspace)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap.State != manifest.StatePresent {
		t.Fatalf("manifest state = %v; want present", snap.State)
	}
	seen := map[string]int{}
	for _, e := range snap.Manifest.Entries {
		seen[e.Ref.Key+"|"+e.TargetPath]++
	}
	for k, n := range seen {
		if n > 1 {
			t.Errorf("(ref, path) %q appears %d times; want at most 1", k, n)
		}
	}
	if n := countRefAt(snap.Manifest, skipRunnerRef, skipPathB); n != 0 {
		t.Errorf("saved manifest has %d entries at skipped path %q; want 0", n, skipPathB)
	}
	if n := countRefAt(snap.Manifest, skipRunnerRef, skipPathA); n != 1 {
		t.Errorf("saved manifest has %d entries at %q; want 1", n, skipPathA)
	}
}

// TestSkippedConflict_OwnEntryKeptWhenOtherPathEntryPrecedesIt verifies that when both
// paths have entries and the other path's entry comes first in the manifest, the skipped
// path keeps its own entry and not the first one for the ref.
func TestSkippedConflict_OwnEntryKeptWhenOtherPathEntryPrecedesIt(t *testing.T) {
	other := skipEntry(skipPathA, "a")
	own := skipEntry(skipPathB, "b")
	workspace, store := seedSkipWorkspace(t, other, own)

	got := executeSkippedConflict(t, workspace, store)

	entry, found := got.LookupAt(skipRunnerRef, skipPathB)
	if !found {
		t.Fatalf("own entry at %q was lost", skipPathB)
	}
	if entry.ContentHash != own.ContentHash {
		t.Errorf("own entry ContentHash = %q; want unchanged %q", entry.ContentHash, own.ContentHash)
	}
	if n := countRefAt(got, skipRunnerRef, skipPathB); n != 1 {
		t.Errorf("entries at %q = %d; want 1", skipPathB, n)
	}
	otherGot, found := got.LookupAt(skipRunnerRef, skipPathA)
	if !found || otherGot.ContentHash != other.ContentHash {
		t.Errorf("other path entry changed: found=%v hash=%q; want %q", found, otherGot.ContentHash, other.ContentHash)
	}
}

// TestSkippedConflict_OwnEntryKeptWhenOtherPathEntryFollowsIt verifies the own entry is
// kept unchanged when it is the first entry for the ref.
func TestSkippedConflict_OwnEntryKeptWhenOtherPathEntryFollowsIt(t *testing.T) {
	own := skipEntry(skipPathB, "b")
	other := skipEntry(skipPathA, "a")
	workspace, store := seedSkipWorkspace(t, own, other)

	got := executeSkippedConflict(t, workspace, store)

	entry, found := got.LookupAt(skipRunnerRef, skipPathB)
	if !found || entry.ContentHash != own.ContentHash {
		t.Errorf("own entry: found=%v hash=%q; want unchanged %q", found, entry.ContentHash, own.ContentHash)
	}
	otherGot, found := got.LookupAt(skipRunnerRef, skipPathA)
	if !found || otherGot.ContentHash != other.ContentHash {
		t.Errorf("other path entry changed: found=%v hash=%q; want %q", found, otherGot.ContentHash, other.ContentHash)
	}
}
