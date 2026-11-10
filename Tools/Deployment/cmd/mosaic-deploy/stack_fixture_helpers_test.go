package main

// stack_fixture_helpers_test.go holds the shared helper that gives a fixture catalog
// root a valid catalog identity file, so the root is acceptable as a catalog layer.
// Nothing in this file is a test itself.

import (
	"os"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

// writeCatalogIdentityFile writes a valid identity file with the given id into the
// catalog root, creating the root if needed. Fixture roots that coexist in one test
// must use distinct ids.
func writeCatalogIdentityFile(t *testing.T, catalogRoot, id string) {
	t.Helper()
	if err := os.MkdirAll(catalogRoot, 0o755); err != nil {
		t.Fatalf("create catalog root %q: %v", catalogRoot, err)
	}
	content := "id: " + id + "\nname: Fixture " + id + "\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(catalogpaths.IdentityFile(catalogRoot), []byte(content), 0o644); err != nil {
		t.Fatalf("write catalog identity file: %v", err)
	}
}

// newIdentifiedCatalogRoot returns a fresh temporary catalog root that already carries
// a valid identity file with the given id.
func newIdentifiedCatalogRoot(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	writeCatalogIdentityFile(t, root, id)
	return root
}
