package catalog_test

// fixture_root_test.go holds the helpers that resolve and load the committed MOSAIC-root
// fixture under testdata/catalog/mosaic-root. Tests that need a populated catalog use these
// helpers instead of the live repository, so they are unaffected by changes to the real
// Catalog/ tree. A missing or unloadable fixture is a repository defect: the helpers fail
// the test and never skip.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/catalogpaths"
)

// fixtureTreeDir returns the absolute path of testdata/catalog/<subdir> (relative to this
// package directory) and fails the test if the path cannot be made absolute or is not a
// directory.
func fixtureTreeDir(t *testing.T, subdir string) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "catalog", subdir))
	if err != nil {
		t.Fatalf("resolve fixture directory %q: %v", subdir, err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("fixture directory %q is missing or not a directory: %v", dir, err)
	}
	return dir
}

// fixtureMosaicRoot returns the absolute path of the committed MOSAIC-root fixture
// testdata/catalog/mosaic-root. It fails the test if the directory or its ResolveRoot
// marker is missing.
func fixtureMosaicRoot(t *testing.T) string {
	t.Helper()
	dir := fixtureTreeDir(t, "mosaic-root")
	marker := filepath.Join(dir, catalogpaths.MosaicRelSourceFilesFormatFile)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("fixture root marker %q is missing: %v", marker, err)
	}
	return dir
}

// loadFixtureRootCatalog resolves the fixture MOSAIC root through catalog.ResolveRoot and
// loads it with catalog.Load. It fails the test if either returns an error.
func loadFixtureRootCatalog(t *testing.T) catalog.Catalog {
	t.Helper()
	root, err := catalog.ResolveRoot(fixtureMosaicRoot(t))
	if err != nil {
		t.Fatalf("ResolveRoot(fixture root): %v", err)
	}
	cat, err := catalog.Load(root, "")
	if err != nil {
		t.Fatalf("catalog.Load(%q): %v", root, err)
	}
	return cat
}

// loadFixtureTreeCatalog loads the committed fixture tree testdata/catalog/<fixtureSubdir>
// with catalog.Load. It fails the test if the directory is missing or Load returns an error.
func loadFixtureTreeCatalog(t *testing.T, fixtureSubdir string) catalog.Catalog {
	t.Helper()
	dir := fixtureTreeDir(t, fixtureSubdir)
	cat, err := catalog.Load(dir, "")
	if err != nil {
		t.Fatalf("catalog.Load(%q): %v", dir, err)
	}
	return cat
}
