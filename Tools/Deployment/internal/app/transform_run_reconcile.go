package app

// transform_run_reconcile.go carries the manifest entries dropped before a transform was planned
// (old-format migration and reconciliation with the disk) into the reviewed plan and the result.

import (
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// reconcileManifest records the entries the manifest loses before planning and keeps the
// reconciled manifest of the target harness. It reads the manifest only; nothing is written.
func (r *transformRun) reconcileManifest() {
	rm := loadReconciledManifest(r.s.deps.Manifest, r.workspace)
	r.dropped = rm.Dropped
	r.targetManifest = rm.forHarness(r.tgtID)
	r.sourceManifest = rm.forHarness(r.srcID)
}

// droppedEntryResults is the result form of the dropped entries; nil when there are none.
func (r *transformRun) droppedEntryResults() []TransformDroppedEntry {
	var out []TransformDroppedEntry
	for _, d := range r.dropped {
		out = append(out, TransformDroppedEntry{
			Harness: d.Entry.Harness, Kind: d.Entry.Ref.Kind, Key: d.Entry.Ref.Key,
			Path: filepath.ToSlash(d.Entry.TargetPath), Reason: d.Reason,
		})
	}
	return out
}

// droppedEntries is the plan form of the dropped entries; nil when there are none.
func (r *transformRun) droppedEntries() []domain.DroppedEntry {
	if len(r.dropped) == 0 {
		return nil
	}
	return r.dropped
}
