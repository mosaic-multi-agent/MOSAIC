package deploy_test

// seed_entry_files_helpers_test.go holds helpers that seed a prior manifest together with the
// files it records. The executor reconciles its loaded manifest with the disk, so an entry whose
// file is absent is dropped before the run; tests that expect prior entries to survive must
// have the files on disk.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// writeMissingEntryFiles creates every file recorded by entries that is not yet on disk under
// workspace. Existing files are left untouched.
func writeMissingEntryFiles(t *testing.T, workspace string, entries []domain.ManifestEntry) {
	t.Helper()
	for _, e := range entries {
		full := filepath.Join(workspace, filepath.FromSlash(e.TargetPath))
		if _, err := os.Lstat(full); err == nil {
			continue
		}
		writeWorkspaceFile(t, workspace, e.TargetPath, "deployed content")
	}
}

// saveManifestWithFiles saves prior for workspace and creates the files its entries record.
func saveManifestWithFiles(t *testing.T, store manifest.Store, workspace string, prior domain.Manifest) {
	t.Helper()
	writeMissingEntryFiles(t, workspace, prior.Entries)
	if err := store.Save(workspace, prior); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
}
