package app_test

// promote_replace_refusal_test.go covers promote without Overwrite when the target layer
// already holds an agent with the key: it is refused whichever category is chosen, nothing is
// written or removed, and the error names the existing file. It also covers the one failure
// of a replacement: the old file cannot be removed.

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

// TestPromote_ExistingKeyWithoutOverwrite_RefusedInEveryCategory verifies an existing agent
// with the key is never replaced or duplicated without Overwrite, in the same category or any
// other, and that nothing changes.
func TestPromote_ExistingKeyWithoutOverwrite_RefusedInEveryCategory(t *testing.T) {
	for _, tc := range replaceCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ls := newPlLayerSet(t, "top", "low")
			ls.addAgentIn(t, "low", tc.existing, plAgentKey, "301")
			svc := app.New(ls.deps(t, newPlInteraction("")))
			src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
			srcBefore := readBytes(t, src)
			before := ls.snapshotLayers(t)

			// Act
			result, err := svc.Promote(context.Background(), replaceRequest(src, tc.chosen, false))

			// Assert
			if !errors.Is(err, app.ErrPromoteDestinationExists) {
				t.Fatalf("error = %v, want one wrapping ErrPromoteDestinationExists", err)
			}
			if existing := ls.agentPath("low", tc.existing, plAgentKey); !strings.Contains(err.Error(), existing) {
				t.Errorf("error %q does not name the existing file %q", err, existing)
			}
			if !reflect.DeepEqual(result, app.PromoteResult{}) {
				t.Errorf("result is non-zero on refusal: %+v", result)
			}
			if got := ls.changedLayers(t, before); len(got) != 0 {
				t.Errorf("layers changed despite the refusal: %v", got)
			}
			if got := keyFiles(t, ls.roots["low"], plAgentKey); len(got) != 1 {
				t.Errorf("layer holds %v for the key, want exactly one file", got)
			}
			if !bytes.Equal(srcBefore, readBytes(t, src)) {
				t.Error("the source file was modified")
			}
		})
	}
}

// holdFileUndeletable makes removing path fail for the rest of the test: an open handle on
// Windows (no delete sharing), a read-only parent directory elsewhere.
func holdFileUndeletable(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %q: %v", path, err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return
	}
	if os.Geteuid() == 0 {
		t.Skip("root can remove files from a read-only directory")
	}
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod %q: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// TestPromote_Overwrite_RemovalOfOldFileFails_ReportsBothFiles verifies that when the old file
// of a cross-category replacement cannot be removed, the new file has been written, the old one
// remains, and the error wraps the removal error and names both files.
func TestPromote_Overwrite_RemovalOfOldFileFails_ReportsBothFiles(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "low")
	ls.addAgent(t, "low", "Audit", plAgentKey, "301")
	oldPath := ls.agentPath("low", "Audit", plAgentKey)
	newPath := ls.agentPath("low", "Build", plAgentKey)
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	holdFileUndeletable(t, oldPath)

	// Act
	_, err := svc.Promote(context.Background(), replaceRequest(src, "Build", true))

	// Assert
	if err == nil {
		t.Fatal("Promote succeeded although the old file could not be removed")
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("error %v does not wrap the removal error", err)
	}
	for _, p := range []string{oldPath, newPath} {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("error %q does not name %q", err, p)
		}
		if _, statErr := os.Stat(p); statErr != nil {
			t.Errorf("expected %q to exist after the failed removal: %v", p, statErr)
		}
	}
	if id, _ := parseFrontmatterFieldSvc(t, newPath, "id"); id != "301" {
		t.Errorf("new file id = %q, want the replaced agent's 301", id)
	}
}
