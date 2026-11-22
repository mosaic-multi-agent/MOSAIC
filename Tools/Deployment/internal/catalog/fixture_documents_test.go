package catalog_test

// fixture_documents_test.go resolves the committed fixture directories that hold the bundle
// document, the protocol document and the Codex harness-content files. A missing directory
// is a repository defect: the helpers fail the test and never skip.

import (
	"os"
	"path/filepath"
	"testing"
)

// documentFixtureCatalogDir returns the absolute Catalog/ directory of the committed
// document-root fixture, which holds a DeployedSections.md bundle document.
func documentFixtureCatalogDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(fixtureTreeDir(t, "document-root"), "Catalog")
}

// frozenCatalogDir returns the absolute Catalog/ directory of the read-only
// testdata/frozen-catalog fixture, which holds a CommunicationProtocol.md document and
// the harness injection content directories.
func frozenCatalogDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "frozen-catalog", "Catalog"))
	if err != nil {
		t.Fatalf("resolve frozen catalog directory: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("frozen catalog directory %q is missing or not a directory: %v", dir, err)
	}
	return dir
}
