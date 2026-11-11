package app

// manifest_reconcile.go is the one entry point every flow uses to read the workspace manifest:
// it loads the manifest, applies the old-format migration result and reconciles the whole
// manifest (all harnesses) with the disk, in memory only. Entries dropped on the way are carried
// to the plan so they are listed before the user confirms the run.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// reconciledManifest is the manifest as the flows see it before planning.
type reconciledManifest struct {
	// Snapshot holds the whole reconciled manifest; Snapshot.DamagedHooks lists the damaged
	// hook bundles.
	Snapshot manifest.Snapshot
	// Dropped lists the entries removed by the old-format migration, then those whose files are
	// missing.
	Dropped []domain.DroppedEntry
}

// loadReconciledManifest loads the manifest of workspace and reconciles it with the disk. A
// manifest that is not present is returned as loaded; load errors are treated like an absent
// manifest, as the flows always did.
func loadReconciledManifest(store manifest.Store, workspace string) reconciledManifest {
	snap, _ := store.Load(workspace)
	return reconcileSnapshot(snap, manifest.FileExists(workspace))
}

// reconcileSnapshot reconciles a loaded snapshot with the disk reported by exists.
func reconcileSnapshot(snap manifest.Snapshot, exists func(string) bool) reconciledManifest {
	out := reconciledManifest{Snapshot: snap, Dropped: append([]domain.DroppedEntry(nil), snap.MigrationDropped...)}
	if snap.State != manifest.StatePresent {
		return out
	}
	r := manifest.Reconcile(snap.Manifest, exists)
	out.Snapshot.Manifest = r.Manifest
	out.Snapshot.DamagedHooks = r.Damaged
	out.Dropped = append(out.Dropped, r.Dropped...)
	return out
}

// forHarness returns the reconciled snapshot narrowed to the entries of harnessID: a run sees
// only its own harness's entries.
func (r reconciledManifest) forHarness(harnessID string) manifest.Snapshot {
	return r.Snapshot.ForHarness(harnessID)
}

// attachTo records the dropped entries on the plan shown for review.
func (r reconciledManifest) attachTo(p *domain.Plan) {
	if len(r.Dropped) > 0 {
		p.DroppedEntries = r.Dropped
	}
}
