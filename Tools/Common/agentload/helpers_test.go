package agentload_test

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/agentload"
)

// fixture returns the path of a file under this package's testdata directory.
func fixture(parts ...string) string {
	return filepath.Join(append([]string{"testdata"}, parts...)...)
}

// copyFixture copies a testdata file into a temp directory and returns the copy's
// path, so a test can load it and prove the load left it untouched.
func copyFixture(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(fixture(parts...))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(t.TempDir(), parts[len(parts)-1])
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return dst
}

// mustLoad loads path and fails the test on error.
func mustLoad(t *testing.T, path, formatID string) *agentload.Agent {
	t.Helper()
	a, err := agentload.Load(path, formatID)
	if err != nil {
		t.Fatalf("Load(%q, %q) error: %v", path, formatID, err)
	}
	if a == nil {
		t.Fatalf("Load(%q, %q) returned nil agent with nil error", path, formatID)
	}
	return a
}

// scalar returns the scalar text of a frontmatter key of the agent's document.
func scalar(t *testing.T, a *agentload.Agent, key string) string {
	t.Helper()
	if a.Document == nil {
		t.Fatal("agent has no parsed document")
	}
	v, ok := a.Document.Frontmatter().Get(key)
	if !ok {
		t.Fatalf("frontmatter key %q absent", key)
	}
	return v.Scalar
}

// assertUnchanged fails when the file at path no longer holds want or its
// modification time moved.
func assertUnchanged(t *testing.T, path string, want []byte, wantMod os.FileInfo) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read %q: %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("file %q was modified by Load", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if !info.ModTime().Equal(wantMod.ModTime()) {
		t.Errorf("file %q modification time changed by Load", path)
	}
}
