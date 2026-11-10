package stackconfig_test

// findworkspace_test.go covers FindWorkspace.
//
//   - A file or a directory inside a deployed workspace finds that workspace, also when the path
//     itself does not exist yet.
//   - A directory that is itself a workspace is its own workspace.
//   - The nearest workspace wins when workspaces are nested.
//   - A directory holding .mosaic but no manifest is not a workspace.
//   - A path with no workspace among its ancestors, and an empty path, report none.
//   - A relative path is resolved against the working directory and the result is absolute.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/manifest"
)

// newDeployedWorkspace creates a directory holding .mosaic/manifest.yaml and returns it.
func newDeployedWorkspace(t *testing.T, parent string) string {
	t.Helper()
	ws := filepath.Join(parent, "ws")
	writeManifestAt(t, ws)
	return ws
}

func writeManifestAt(t *testing.T, ws string) {
	t.Helper()
	dir := filepath.Join(ws, manifest.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create .mosaic: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), []byte("schema_version: \"1\"\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func requireSameDir(t *testing.T, got, want string) {
	t.Helper()
	gi, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat result %q: %v", got, err)
	}
	wi, err := os.Stat(want)
	if err != nil {
		t.Fatalf("stat %q: %v", want, err)
	}
	if !os.SameFile(gi, wi) {
		t.Errorf("workspace = %q, want %q", got, want)
	}
}

func requireWorkspace(t *testing.T, path, want string) {
	t.Helper()
	got, ok := stackconfig.FindWorkspace(path)
	if !ok {
		t.Fatalf("FindWorkspace(%q) found no workspace, want %q", path, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("workspace %q is not absolute", got)
	}
	requireSameDir(t, got, want)
}

func TestFindWorkspace_FileInsideWorkspace(t *testing.T) {
	ws := newDeployedWorkspace(t, t.TempDir())
	file := filepath.Join(ws, "agents", "a.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	requireWorkspace(t, file, ws)
}

func TestFindWorkspace_DirectoryInsideWorkspace(t *testing.T) {
	ws := newDeployedWorkspace(t, t.TempDir())
	dir := filepath.Join(ws, "agents", "deep")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	requireWorkspace(t, dir, ws)
}

func TestFindWorkspace_DirectoryThatIsItselfAWorkspace(t *testing.T) {
	ws := newDeployedWorkspace(t, t.TempDir())

	requireWorkspace(t, ws, ws)
}

func TestFindWorkspace_PathThatDoesNotExistYet_UsesItsAncestors(t *testing.T) {
	ws := newDeployedWorkspace(t, t.TempDir())

	requireWorkspace(t, filepath.Join(ws, "not", "created", "x.md"), ws)
}

func TestFindWorkspace_NestedWorkspaces_NearestWins(t *testing.T) {
	outer := newDeployedWorkspace(t, t.TempDir())
	inner := newDeployedWorkspace(t, outer)

	requireWorkspace(t, filepath.Join(inner, "sub"), inner)
}

func TestFindWorkspace_NoAncestorHasManifest_ReportsNone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plain", "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if got, ok := stackconfig.FindWorkspace(dir); ok {
		t.Errorf("FindWorkspace(%q) = %q, want none", dir, got)
	}
}

func TestFindWorkspace_MosaicDirectoryWithoutManifest_IsNotAWorkspace(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(filepath.Join(dir, manifest.Dir, manifest.BackupDir), 0o755); err != nil {
		t.Fatal(err)
	}

	if got, ok := stackconfig.FindWorkspace(dir); ok {
		t.Errorf("FindWorkspace(%q) = %q, want none: .mosaic without a manifest is not a workspace", dir, got)
	}
}

func TestFindWorkspace_EmptyPath_ReportsNone(t *testing.T) {
	if got, ok := stackconfig.FindWorkspace(""); ok {
		t.Errorf("FindWorkspace(\"\") = %q, want none", got)
	}
}

func TestFindWorkspace_RelativePath_ResolvedAgainstWorkingDirectory(t *testing.T) {
	ws := newDeployedWorkspace(t, t.TempDir())
	if err := os.MkdirAll(filepath.Join(ws, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(ws)

	requireWorkspace(t, "sub", ws)
	requireWorkspace(t, ".", ws)
}
