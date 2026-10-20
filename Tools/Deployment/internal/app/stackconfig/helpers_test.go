package stackconfig_test

// helpers_test.go holds the fixtures shared by the stackconfig tests: catalog roots with an
// identity, workspaces with a manifest carrying a stack, and small assertions.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// newCatalogRoot creates a catalog root whose identity has the given id and returns the root.
func newCatalogRoot(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	writeIdentity(t, root, id)
	return root
}

// writeIdentity writes a valid catalog-identity.yaml with the given id into root.
func writeIdentity(t *testing.T, root, id string) {
	t.Helper()
	content := "id: " + id + "\nname: Fixture " + id + "\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(root, "catalog-identity.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write identity: %v", err)
	}
}

// userLayer is the persisted form of a user layer.
func userLayer(root, id string) domain.PersistedCatalogLayer {
	return domain.PersistedCatalogLayer{Path: root, CatalogID: id}
}

var defaultLayer = domain.PersistedCatalogLayer{Default: true}

// saveStack saves a manifest carrying layers into a new workspace and returns the workspace.
func saveStack(t *testing.T, layers ...domain.PersistedCatalogLayer) string {
	t.Helper()
	ws := t.TempDir()
	m := domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		HarnessID:     "claude-code",
		UpdatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		CatalogStack:  layers,
	}
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return ws
}

// writeRawManifest places raw manifest text in a new workspace and returns the workspace.
func writeRawManifest(t *testing.T, text string) string {
	t.Helper()
	ws := t.TempDir()
	dir := filepath.Join(ws, manifest.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create .mosaic: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), []byte(text), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return ws
}

// requireNoStack asserts that Read reports "none" without an error.
func requireNoStack(t *testing.T, workspace string) {
	t.Helper()
	s, ok, err := stackconfig.Read(workspace)
	if err != nil {
		t.Fatalf("Read: unexpected error: %v", err)
	}
	if ok || len(s.Layers) != 0 {
		t.Errorf("Read = (%+v, %v), want none", s, ok)
	}
}

// mustEncode encodes d or fails the test.
func mustEncode(t *testing.T, d stackconfig.Description) stackconfig.Stored {
	t.Helper()
	s, err := stackconfig.Encode(d)
	if err != nil {
		t.Fatalf("Encode(%+v): %v", d, err)
	}
	return s
}
