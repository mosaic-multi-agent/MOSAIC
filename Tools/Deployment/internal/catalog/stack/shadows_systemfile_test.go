package stack_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/systemfile"
	"mosaic-deploy/internal/domain"
)

// systemFileStack is what the app layer reaches through the companion interface for system files.
type systemFileStack interface {
	stack.Catalog
	SystemFiles() systemfile.Result
}

func newTestSystemFileStack(t *testing.T, mosaicRoot string, roots ...string) systemFileStack {
	t.Helper()
	c, ok := newTestStack(t, mosaicRoot, roots...).(systemFileStack)
	if !ok {
		t.Fatalf("stack.Catalog does not expose SystemFiles()")
	}
	return c
}

func systemFileItem(name string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemSystemFile, Key: name}
}

func TestStack_SystemFiles_ResolveFromHighestLayerThatHasEachFile(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	mustWrite(t, high, catalogpaths.RelBundleFile, "# bundle high\n")
	mustWrite(t, low, catalogpaths.RelBundleFile, "# bundle low\n")
	mustWrite(t, low, catalogpaths.RelProtocolFile, "# protocol low\n")
	c := newTestSystemFileStack(t, mosaic, high, low)

	res := c.SystemFiles()

	if res.Bundle.Path != filepath.Join(high, catalogpaths.RelBundleFile) {
		t.Errorf("Bundle.Path = %q, want the high layer's file", res.Bundle.Path)
	}
	if res.Protocol.Path != filepath.Join(low, catalogpaths.RelProtocolFile) {
		t.Errorf("Protocol.Path = %q, want the low layer's file", res.Protocol.Path)
	}
	if res.Protocol.Supplier != testLayer(low, "low") {
		t.Errorf("Protocol.Supplier = %+v, want the low layer", res.Protocol.Supplier)
	}
}

func TestStack_SystemFiles_UnsuppliedFileIsNotSupplied(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	only := newTestCatalogRoot(t, "only")
	c := newTestSystemFileStack(t, mosaic, only)

	res := c.SystemFiles()

	if res.Protocol.Err() == nil || res.Bundle.Err() == nil {
		t.Errorf("both files must be reported as not supplied: %+v", res)
	}
}

func TestStack_SystemFiles_ProvenanceAndShadowsCoverSystemFiles(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	mustWrite(t, high, catalogpaths.RelProtocolFile, "# p\n")
	mustWrite(t, low, catalogpaths.RelProtocolFile, "# p\n")
	mustWrite(t, low, catalogpaths.RelBundleFile, "# b\n")
	c := newTestProvenanceStack(t, mosaic, high, low)

	layer, ok := c.Provenance(systemFileItem(catalogpaths.RelProtocolFile))
	if !ok || layer != testLayer(high, "high") {
		t.Errorf("Provenance(protocol) = %+v, %v, want the high layer", layer, ok)
	}
	layer, ok = c.Provenance(systemFileItem(catalogpaths.RelBundleFile))
	if !ok || layer != testLayer(low, "low") {
		t.Errorf("Provenance(bundle) = %+v, %v, want the low layer", layer, ok)
	}
	rec := shadowFor(t, c.Shadows(), systemFileItem(catalogpaths.RelProtocolFile))
	assertShadow(t, rec, testLayer(high, "high"), []domain.CatalogLayer{testLayer(low, "low")}, false)
	for _, r := range c.Shadows() {
		if r.Item == systemFileItem(catalogpaths.RelBundleFile) {
			t.Errorf("unexpected shadow record for a file in one layer only: %+v", r)
		}
	}
}

func TestStack_SystemFiles_DeclaredFlagFollowsWinnerSystemFilesDeclaration(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	withTestOverrides(t, high, "high", "  system_files: [CommunicationProtocol.md]\n")
	mustWrite(t, high, catalogpaths.RelProtocolFile, "# p high\n")
	mustWrite(t, low, catalogpaths.RelProtocolFile, "# p low\n")
	mustWrite(t, high, catalogpaths.RelBundleFile, "# b high\n")
	mustWrite(t, low, catalogpaths.RelBundleFile, "# b low\n")
	c := newTestProvenanceStack(t, mosaic, high, low)

	records := c.Shadows()

	both := []domain.CatalogLayer{testLayer(low, "low")}
	assertShadow(t, shadowFor(t, records, systemFileItem(catalogpaths.RelProtocolFile)), testLayer(high, "high"), both, true)
	assertShadow(t, shadowFor(t, records, systemFileItem(catalogpaths.RelBundleFile)), testLayer(high, "high"), both, false)
}

func TestStack_SystemFiles_NoSupplierMeansNoProvenanceForThatFileOnly(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	only := newTestCatalogRoot(t, "only")
	mustWrite(t, only, catalogpaths.RelProtocolFile, "# p\n")
	c := newTestProvenanceStack(t, mosaic, only)

	if _, ok := c.Provenance(systemFileItem(catalogpaths.RelBundleFile)); ok {
		t.Error("Provenance for an unsupplied system file must report ok=false")
	}
	if layer, ok := c.Provenance(systemFileItem(catalogpaths.RelProtocolFile)); !ok || layer != testLayer(only, "only") {
		t.Errorf("Provenance(protocol) = %+v, %v, want the only layer", layer, ok)
	}
}

func TestStack_SystemFiles_LoaderFacingFileNames(t *testing.T) {
	if catalogpaths.RelProtocolFile != "CommunicationProtocol.md" {
		t.Errorf("RelProtocolFile = %q", catalogpaths.RelProtocolFile)
	}
	if catalogpaths.RelBundleFile != "DeployedSections.md" {
		t.Errorf("RelBundleFile = %q", catalogpaths.RelBundleFile)
	}
}
