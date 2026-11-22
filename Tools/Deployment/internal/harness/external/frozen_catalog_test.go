package external_test

import (
	"path/filepath"
	"testing"
)

// frozenCatalogRoot returns the absolute path to the frozen MOSAIC-root-shaped fixture tree
// (Tools/Deployment/testdata/frozen-catalog). Tests that start the reference module use it as
// the MOSAIC root so they do not depend on the live catalog.
func frozenCatalogRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "frozen-catalog"))
	if err != nil {
		t.Fatalf("resolve frozen catalog root: %v", err)
	}
	return abs
}
