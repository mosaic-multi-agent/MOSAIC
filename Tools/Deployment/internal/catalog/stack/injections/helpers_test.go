package injections_test

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

var (
	claudeDir = catalogpaths.HarnessDirNameClaudeCode
	ghcpDir   = catalogpaths.HarnessDirNameGhcpCLI
	openDir   = catalogpaths.HarnessDirNameOpenCode
)

const (
	sharedFile = "HarnessInjections.md"
	orchFile   = "HarnessInjectionsOrchestrator.md"
)

// testLayerDir creates a temp catalog root and the layer input for it. For every harness
// directory name in harnesses it creates HarnessInjections/<name>/ holding the given files.
func testLayerDir(t *testing.T, id string, overrides identity.Overrides, harnesses map[string][]string) stackrec.LayerInput {
	t.Helper()
	root := t.TempDir()
	for name, files := range harnesses {
		dir := injDir(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("# "+id+" "+f+"\n"), 0o644); err != nil {
				t.Fatalf("write %s: %v", f, err)
			}
		}
	}
	return stackrec.LayerInput{
		Layer: domain.CatalogLayer{
			Identity: domain.CatalogIdentity{ID: id, Name: "Catalog " + id, Version: "1.0.0"},
			Root:     root,
		},
		Overrides: overrides,
	}
}

func injectionsItem(harnessDir string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemHarnessInjections, Key: harnessDir}
}

func bothFiles() []string { return []string{sharedFile, orchFile} }

// injDir is the expected injection directory of a harness in a catalog root, spelled out
// independently of the production path builder.
func injDir(root, harnessDir string) string {
	return filepath.Join(root, "HarnessInjections", harnessDir)
}
