package systemfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// testLayerDir creates a temp catalog root and the layer input for it. files lists the
// system-file names to create in the root.
func testLayerDir(t *testing.T, id string, overrides identity.Overrides, files ...string) stackrec.LayerInput {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f), []byte("# "+f+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
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

func systemFileItem(name string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemSystemFile, Key: name}
}

var (
	protocolName = catalogpaths.RelProtocolFile
	bundleName   = catalogpaths.RelBundleFile
)
