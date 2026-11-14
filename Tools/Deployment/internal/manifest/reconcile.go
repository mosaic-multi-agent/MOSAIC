package manifest

import (
	"os"
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// DamagedBundle identifies a hook bundle of one harness that has some, but not all, of its
// recorded files on disk.
type DamagedBundle struct {
	Harness string
	Key     string
}

// Reconciliation is the result of comparing a manifest with the disk.
type Reconciliation struct {
	// Manifest is the input minus dropped entries and minus sections left without entries.
	Manifest domain.Manifest
	// Dropped lists the removed entries with Reason domain.DropReasonMissing, in input order.
	Dropped []domain.DroppedEntry
	// Damaged lists the hook bundles that were kept although some of their files are missing.
	Damaged []DamagedBundle
}

// IsDamaged reports whether the hook bundle key of harness is damaged.
func (r Reconciliation) IsDamaged(harness, key string) bool {
	for _, d := range r.Damaged {
		if d.Harness == harness && d.Key == key {
			return true
		}
	}
	return false
}

// bundleID groups the hook entries of one harness that share one bundle key.
type bundleID struct{ harness, key string }

// bundleState counts the recorded and the present files of one hook bundle.
type bundleState struct{ total, present int }

// Reconcile decides presence only, for the entries of every harness. exists reports whether a
// workspace-relative target path is a regular file. Reconcile performs no I/O itself.
//
// An entry whose file is missing is dropped. A hook bundle with some files missing is kept and
// reported as damaged; a hook bundle with every file missing is dropped. A bundle-level hook
// entry recorded before per-file hook entries (empty-content hash) is always kept and does not
// count towards the presence of its bundle.
func Reconcile(m domain.Manifest, exists func(targetPath string) bool) Reconciliation {
	bundles := map[bundleID]*bundleState{}
	var bundleOrder []bundleID
	for _, e := range m.Entries {
		if !isPerFileHook(e) {
			continue
		}
		id := bundleID{e.Harness, e.Ref.Key}
		st := bundles[id]
		if st == nil {
			st = &bundleState{}
			bundles[id] = st
			bundleOrder = append(bundleOrder, id)
		}
		st.total++
		if exists(e.TargetPath) {
			st.present++
		}
	}

	out := m
	out.Entries = nil
	var dropped []domain.DroppedEntry
	for _, e := range m.Entries {
		if keepEntry(e, bundles, exists) {
			out.Entries = append(out.Entries, e)
			continue
		}
		dropped = append(dropped, domain.DroppedEntry{Entry: e, Reason: domain.DropReasonMissing})
	}

	var damaged []DamagedBundle
	for _, id := range bundleOrder {
		if st := bundles[id]; st.present > 0 && st.present < st.total {
			damaged = append(damaged, DamagedBundle{Harness: id.harness, Key: id.key})
		}
	}

	out.Harnesses = sectionsStillHeld(m, out, dropped)
	return Reconciliation{Manifest: out, Dropped: dropped, Damaged: damaged}
}

// isPerFileHook reports whether e is a hook entry that records one deployed file, as opposed
// to a bundle-level entry recorded before per-file hook entries.
func isPerFileHook(e domain.ManifestEntry) bool {
	return e.Ref.Kind == domain.ArtifactHook && e.ContentHash != Hash(nil)
}

// keepEntry decides whether e survives reconciliation.
func keepEntry(e domain.ManifestEntry, bundles map[bundleID]*bundleState, exists func(string) bool) bool {
	if e.Ref.Kind == domain.ArtifactHook {
		if !isPerFileHook(e) {
			return true
		}
		return bundles[bundleID{e.Harness, e.Ref.Key}].present > 0
	}
	return exists(e.TargetPath)
}

// sectionsStillHeld returns the sections of m, without those of harnesses that lost entries
// and keep none.
func sectionsStillHeld(m, reconciled domain.Manifest, dropped []domain.DroppedEntry) []domain.HarnessSection {
	if len(dropped) == 0 {
		return m.Harnesses
	}
	lost := map[string]bool{}
	for _, d := range dropped {
		lost[d.Entry.Harness] = true
	}
	var sections []domain.HarnessSection
	for _, s := range m.Harnesses {
		if lost[s.HarnessID] && !reconciled.Holds(s.HarnessID) {
			continue
		}
		sections = append(sections, s)
	}
	return sections
}

// FileExists returns the presence check every reconciliation uses: true when
// <workspaceRoot>/<targetPath> is a regular file (symlinks followed). Never errors.
func FileExists(workspaceRoot string) func(targetPath string) bool {
	return func(targetPath string) bool {
		info, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(targetPath)))
		return err == nil && info.Mode().IsRegular()
	}
}
