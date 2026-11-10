package claudecode_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

// frozenContentDirs resolves a harness's injection content directory inside the frozen
// fixture catalog (testdata/frozen-catalog/Catalog), without a stack or identity file.
func frozenContentDirs(t *testing.T) func(string) (string, error) {
	t.Helper()
	catalogRoot := filepath.Join(frozenCatalogRoot(t), "Catalog")
	return func(harnessDirName string) (string, error) {
		return catalogpaths.HarnessInjectionsDir(catalogRoot, harnessDirName), nil
	}
}

// contentDirsUnder resolves a harness's injection content directory below a temp
// MOSAIC-style root (<root>/Catalog/HarnessInjections/<harness dir>).
func contentDirsUnder(root string) func(string) (string, error) {
	return func(harnessDirName string) (string, error) {
		return filepath.Join(root, catalogpaths.MosaicRelHarnessInjectionsDir, harnessDirName), nil
	}
}
