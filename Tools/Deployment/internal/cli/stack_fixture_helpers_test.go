package cli_test

// stack_fixture_helpers_test.go holds the shared helper that builds a fixture catalog
// root carrying a valid catalog identity file, so the root is acceptable as a catalog
// layer. Nothing in this file is a test itself.

import (
	"os"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

// catalogFolderDir returns a temporary directory path to use as a --catalog-folder
// value in CLI invocations. The directory is created by t.TempDir() so it exists
// on disk and is cleaned up automatically, and it carries a valid identity file.
func catalogFolderDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	content := "id: cli-fixture-catalog\nname: CLI Fixture Catalog\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(catalogpaths.IdentityFile(root), []byte(content), 0o644); err != nil {
		t.Fatalf("write catalog identity file: %v", err)
	}
	return root
}
