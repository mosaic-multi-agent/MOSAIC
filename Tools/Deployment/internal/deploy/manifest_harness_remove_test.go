package deploy_test

// manifest_harness_remove_test.go covers harness scoping of removals: a run for one harness
// that removes a file drops only its own manifest entry, never another harness's entry at the
// same path.

import (
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func TestManifestHarness_RemovalDropsOnlyTheRunHarnessEntryAtASharedPath(t *testing.T) {
	ws := t.TempDir()
	store := manifest.NewStore()
	m := twoHarnessManifest()
	m.Entries = append(m.Entries, priorEntry(runHarness, sharedPath, reviewerRef, "sha256:open-shared"))
	seedManifest(t, store, ws, m)
	writeWorkspaceFile(t, ws, sharedPath, "# shared file")
	removal := domain.Removal{Ref: reviewerRef, Path: sharedPath, DropEntry: true, Reason: "no longer deployed"}

	executeFor(t, store, ws, runHarness, func(r *deploy.ExecRequest) {
		r.Plan.Removals = []domain.Removal{removal}
	})

	at := entriesAt(loadManifest(t, ws), sharedPath)
	if _, ok := at[runHarness]; ok {
		t.Errorf("entries at %s = %+v, want the %s entry dropped by its own removal", sharedPath, at, runHarness)
	}
	if got := at[otherHarness]; got.ContentHash != "sha256:ghcp" {
		t.Errorf("entries at %s = %+v, want the %s entry kept intact", sharedPath, at, otherHarness)
	}
}
