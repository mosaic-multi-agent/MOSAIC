package app_test

// transform_hooks_per_file_test.go verifies the manifest a GHCP-pair transform leaves for a
// carried hook bundle: one entry per file the target variant wrote, with the real path and the
// hash of the written bytes, no entry for the registration file the user owns, and no entry for
// a source-only file that was removed.

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func TestTransformGhcpPair_CarriedHookBundle_ManifestHoldsPerFileEntriesOfExistingFiles(t *testing.T) {
	for _, pair := range [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			r, _ := trPairHookRun(t, pair[0], pair[1])
			written := r.hooksDir(pair[1]) + "/logger.sh"

			r.mustRun(r.request())

			entries := trEntryByRef(trLoadManifestIfAny(t, r.ws), domain.ArtifactHook, trPairHookKey)
			if len(entries) != 1 {
				t.Fatalf("hook entries = %+v, want exactly one (the written logger.sh)", entries)
			}
			e := entries[0]
			if filepath.ToSlash(e.TargetPath) != written {
				t.Errorf("entry path = %q, want the written file %q (not a bundle path, not the registration or removed files)", e.TargetPath, written)
			}
			if e.ContentHash != manifest.Hash([]byte(r.read(written))) {
				t.Errorf("entry hash = %q, want the hash of the bytes on disk", e.ContentHash)
			}
			if e.Version != trHookVersion {
				t.Errorf("entry version = %q, want the catalog bundle version %q", e.Version, trHookVersion)
			}
		})
	}
}
