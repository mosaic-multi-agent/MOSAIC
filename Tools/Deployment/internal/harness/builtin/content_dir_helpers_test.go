package builtin_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

// frozenContentDirs resolves a harness's injection content directory inside the frozen
// fixture catalog (testdata/frozen-catalog/Catalog), without a stack or identity file.
func frozenContentDirs(t *testing.T) func(string) (string, error) {
	t.Helper()
	catalogRoot := filepath.Join(deploymentTestdata(t, "frozen-catalog"), "Catalog")
	return func(harnessDirName string) (string, error) {
		return catalogpaths.HarnessInjectionsDir(catalogRoot, harnessDirName), nil
	}
}
