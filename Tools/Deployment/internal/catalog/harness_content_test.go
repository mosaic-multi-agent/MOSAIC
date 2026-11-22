package catalog_test

// harness_content_test.go provides the structural load check for a Codex harness-content
// directory (I6.4).
//
// The test loads the Codex content directory of the read-only testdata/frozen-catalog
// fixture through the production injection-content loader and asserts structure only:
// both files load without error, and the orchestrator file declares a non-empty version.
// No assertion is made against the wording, length or region content of the files.

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/harness/injectionfile"
)

// TestCodexHarnessContent_FixtureDirectoryLoads verifies that the fixture
// Catalog/HarnessInjections/Codex/ directory parses through the production
// injection-content loader without structural error, and that the orchestrator
// content file declares a non-empty version. Assert structure only: no assertion is
// made against the wording, length, or region content of the files in this directory.
func TestCodexHarnessContent_FixtureDirectoryLoads(t *testing.T) {
	contentDir := filepath.Join(frozenCatalogDir(t), "HarnessInjections", "Codex")


	content, err := injectionfile.LoadDir(contentDir)
	if err != nil {
		t.Fatalf("injectionfile.LoadDir(%q): %v; "+
			"the fixture Codex injections directory must parse through "+
			"the production injection-content loader without error; "+
			"check that HarnessInjections.md and HarnessInjectionsOrchestrator.md "+
			"exist with well-formed frontmatter and properly closed managed regions",
			contentDir, err)
	}

	if content.OrchestratorVersion == "" {
		t.Errorf("HarnessInjectionsOrchestrator.md: version is empty; "+
			"the file must declare a non-empty version in its YAML frontmatter; "+
			"add 'version: \"1.0.0\"' (or later) to the frontmatter block")
	}
}
