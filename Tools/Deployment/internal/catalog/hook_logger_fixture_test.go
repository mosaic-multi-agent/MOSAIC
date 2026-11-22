package catalog_test

// hook_logger_fixture_test.go loads the committed hook-bundle fixture used by the hook
// bundle enumeration tests. The fixture is a self-contained MOSAIC tree under
// testdata/catalog/hook-logger-root, so the tests do not depend on the live Catalog/.

import (
	"testing"

	"mosaic-deploy/internal/catalog"
)

// loadHookLoggerCatalog loads the committed hook-logger-root fixture tree. It fails the
// test if the tree is missing or cannot be loaded.
func loadHookLoggerCatalog(t *testing.T) catalog.Catalog {
	t.Helper()
	return loadFixtureTreeCatalog(t, "hook-logger-root")
}
