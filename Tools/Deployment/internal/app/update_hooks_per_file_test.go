package app_test

// update_hooks_per_file_test.go verifies that Service.Update handles a deployed hook bundle file
// by file: each file of the bundle is its own plan item, an edited file is updated with the edit
// reason and backed up first while an untouched file is left alone, and the manifest written
// afterwards holds one entry per file with the real path and the hash of the bytes on disk.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

const uhEditReason = "Deploy-owned content was edited locally"

// uhFixtureFiles returns the catalog content of every file of layout l, keyed by its
// workspace-relative deployed path: a deployment that nobody edited.
func uhFixtureFiles(t *testing.T, l uhLayout) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, name := range l.files {
		files[uhHooksDir+"/"+name] = uhFixture(t, l, name)
	}
	return files
}

// uhFileEntries returns the per-file hook entries a deploy of layout l at version records.
func uhFileEntries(t *testing.T, l uhLayout, version string) []domain.ManifestEntry {
	t.Helper()
	var entries []domain.ManifestEntry
	for _, name := range l.files {
		data := []byte(uhFixture(t, l, name))
		owned, err := transform.OwnedContentOf(domain.ArtifactHook, data)
		if err != nil {
			t.Fatalf("OwnedContentOf: %v", err)
		}
		entries = append(entries, domain.ManifestEntry{
			Harness:     uhHarnessID,
			Ref:         domain.ArtifactRef{Kind: domain.ArtifactHook, Key: uhHookKey},
			TargetPath:  uhHooksDir + "/" + name,
			Version:     version,
			ContentHash: manifest.Hash(data),
			Owned:       &owned,
		})
	}
	return entries
}

// hookItems returns the hook plan items of the plan the executor received, by target path.
func (r *uhRun) hookItems() map[string]domain.PlanItem {
	items := map[string]domain.PlanItem{}
	for _, item := range r.exec.req.Plan.Items {
		if item.Ref.Kind == domain.ArtifactHook && item.Ref.Key == uhHookKey {
			items[item.TargetPath] = item
		}
	}
	return items
}

// hookEntries returns the hook manifest entries the run wrote, by target path.
func (r *uhRun) hookEntries() map[string]domain.ManifestEntry {
	entries := map[string]domain.ManifestEntry{}
	for _, e := range r.exec.res.Manifest.Entries {
		if e.Ref.Kind == domain.ArtifactHook && e.Ref.Key == uhHookKey {
			entries[e.TargetPath] = e
		}
	}
	return entries
}

func TestUpdate_EditedHookFile_OnlyThatFileUpdatedWithEditReasonAndBackup(t *testing.T) {
	files := uhFixtureFiles(t, uhFlat)
	editedPath := uhHooksDir + "/logger.sh"
	untouchedPath := uhHooksDir + "/util.sh"
	files[editedPath] = "LOCALLY EDITED logger\n"

	run := runUpdateHooks(t, uhConfig{
		entries: uhFileEntries(t, uhFlat, uhCatalogVersion), files: files,
	})

	requireUpdateHooksOK(t, run)
	items := run.hookItems()
	if len(items) != 2 {
		t.Fatalf("hook items = %v, want one per bundle file", items)
	}
	edited := items[editedPath]
	if edited.Action != domain.ActionUpdate || !edited.BackupBeforeWrite ||
		!strings.Contains(reasonText(edited), uhEditReason) {
		t.Errorf("edited file: action %v, backup %v, reasons %q; want update with backup and the edit reason",
			edited.Action, edited.BackupBeforeWrite, reasonText(edited))
	}
	if other := items[untouchedPath]; other.Action != domain.ActionUnchanged || other.BackupBeforeWrite {
		t.Errorf("untouched file: action %v, backup %v; want unchanged without backup", other.Action, other.BackupBeforeWrite)
	}
}

func TestUpdate_EditedHookFile_BackedUpUnderMosaicBackupsAndRewritten(t *testing.T) {
	files := uhFixtureFiles(t, uhFlat)
	editedPath := uhHooksDir + "/logger.sh"
	const editedText = "LOCALLY EDITED logger\n"
	files[editedPath] = editedText

	run := runUpdateHooks(t, uhConfig{
		entries: uhFileEntries(t, uhFlat, uhCatalogVersion), files: files,
	})

	requireUpdateHooksOK(t, run)
	if got, _ := run.read(editedPath); got != uhFixture(t, uhFlat, "logger.sh") {
		t.Errorf("edited file = %q, want the catalog content", got)
	}
	if !backupHolds(t, filepath.Join(run.workspace, ".mosaic", "backups"), editedText) {
		t.Error("no backup under .mosaic/backups holds the edited content")
	}
}

func TestUpdate_StaleHookWithUneditedFiles_EachFileUpdatedWithoutBackup(t *testing.T) {
	run := runUpdateHooks(t, uhConfig{
		entries: uhFileEntries(t, uhFlat, uhStaleVersion), files: uhFixtureFiles(t, uhFlat),
	})

	requireUpdateHooksOK(t, run)
	items := run.hookItems()
	if len(items) != len(uhFlat.files) {
		t.Fatalf("hook items = %v, want one per bundle file", items)
	}
	for path, item := range items {
		if item.Action != domain.ActionUpdate || item.BackupBeforeWrite || item.Conflict != nil {
			t.Errorf("%s: action %v, backup %v, conflict %+v; want a plain update", path, item.Action, item.BackupBeforeWrite, item.Conflict)
		}
	}
}

func TestUpdate_HookBundle_ManifestHoldsOneEntryPerFileWithRealPathAndHash(t *testing.T) {
	files := uhFixtureFiles(t, uhFlat)

	run := runUpdateHooks(t, uhConfig{
		entries: uhFileEntries(t, uhFlat, uhStaleVersion), files: files,
	})

	requireUpdateHooksOK(t, run)
	entries := run.hookEntries()
	if len(entries) != len(uhFlat.files) {
		t.Fatalf("hook manifest entries = %v, want one per file", entries)
	}
	for path, want := range files {
		e, ok := entries[path]
		if !ok {
			t.Errorf("no manifest entry for %s", path)
			continue
		}
		if e.ContentHash != manifest.Hash([]byte(want)) || e.Version != uhCatalogVersion {
			t.Errorf("%s: hash %q version %q, want the hash of the written bytes at version %s", path, e.ContentHash, e.Version, uhCatalogVersion)
		}
		if _, err := os.Stat(filepath.Join(run.workspace, filepath.FromSlash(path))); err != nil {
			t.Errorf("entry path %s does not exist on disk: %v", path, err)
		}
	}
	if _, bundle := entries[uhHookTarget]; bundle {
		t.Errorf("manifest still holds a bundle-level entry at %s", uhHookTarget)
	}
}

// backupHolds reports whether any file under dir has exactly the given content.
func backupHolds(t *testing.T, dir, content string) bool {
	t.Helper()
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if data, rerr := os.ReadFile(path); rerr == nil && string(data) == content {
			found = true
		}
		return nil
	})
	return found
}
