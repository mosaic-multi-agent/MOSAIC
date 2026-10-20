package stack_test

import (
	"errors"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
)

// injectionsStack is what the app layer reaches through the companion interface for harness
// injections.
type injectionsStack interface {
	stack.Catalog
	HarnessInjections() injections.Result
}

func newTestInjectionsStack(t *testing.T, mosaicRoot string, roots ...string) injectionsStack {
	t.Helper()
	c, ok := newTestProvenanceStack(t, mosaicRoot, roots...).(injectionsStack)
	if !ok {
		t.Fatalf("stack.Catalog does not expose HarnessInjections()")
	}
	return c
}

func injectionsItem(harnessDir string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemHarnessInjections, Key: harnessDir}
}

func writeInjectionsFile(t *testing.T, root, harnessDir, name string) {
	t.Helper()
	rel := filepath.Join("HarnessInjections", harnessDir, name)
	mustWrite(t, root, rel, "# "+name+"\n")
}

func TestStack_HarnessInjections_ResolveFromHighestLayerPerHarness(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	writeInjectionsFile(t, high, catalogpaths.HarnessDirNameClaudeCode, "HarnessInjections.md")
	writeInjectionsFile(t, low, catalogpaths.HarnessDirNameClaudeCode, "HarnessInjections.md")
	writeInjectionsFile(t, low, catalogpaths.HarnessDirNameOpenCode, "HarnessInjections.md")
	c := newTestInjectionsStack(t, mosaic, high, low)

	res := c.HarnessInjections()

	claude := res.Lookup(catalogpaths.HarnessDirNameClaudeCode)
	if claude.Dir != injDir(high, catalogpaths.HarnessDirNameClaudeCode) {
		t.Errorf("claude Dir = %q, want the high layer's", claude.Dir)
	}
	if claude.Supplier != testLayer(high, "high") {
		t.Errorf("claude Supplier = %+v, want the high layer", claude.Supplier)
	}
	open := res.Lookup(catalogpaths.HarnessDirNameOpenCode)
	if open.Dir != injDir(low, catalogpaths.HarnessDirNameOpenCode) {
		t.Errorf("opencode Dir = %q, want the low layer's", open.Dir)
	}
}

func TestStack_HarnessInjections_UnsuppliedHarnessIsNotSupplied(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	only := newTestCatalogRoot(t, "only")
	writeInjectionsFile(t, only, catalogpaths.HarnessDirNameClaudeCode, "HarnessInjections.md")
	c := newTestInjectionsStack(t, mosaic, only)

	err := c.HarnessInjections().Lookup(catalogpaths.HarnessDirNameGhcpCLI).Err()

	var nse *injections.NotSuppliedError
	if !errors.As(err, &nse) || nse.HarnessDir != catalogpaths.HarnessDirNameGhcpCLI {
		t.Errorf("Err() = %v, want *NotSuppliedError naming %q", err, catalogpaths.HarnessDirNameGhcpCLI)
	}
}

func TestStack_HarnessInjections_ProvenanceAndShadowsCoverHarnessInjections(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	claude, ghcp := catalogpaths.HarnessDirNameClaudeCode, catalogpaths.HarnessDirNameGhcpCLI
	writeInjectionsFile(t, high, claude, "HarnessInjections.md")
	writeInjectionsFile(t, low, claude, "HarnessInjections.md")
	writeInjectionsFile(t, low, ghcp, "HarnessInjections.md")
	c := newTestInjectionsStack(t, mosaic, high, low)

	layer, ok := c.Provenance(injectionsItem(claude))
	if !ok || layer != testLayer(high, "high") {
		t.Errorf("Provenance(claude) = %+v, %v, want the high layer", layer, ok)
	}
	layer, ok = c.Provenance(injectionsItem(ghcp))
	if !ok || layer != testLayer(low, "low") {
		t.Errorf("Provenance(ghcp) = %+v, %v, want the low layer", layer, ok)
	}
	rec := shadowFor(t, c.Shadows(), injectionsItem(claude))
	assertShadow(t, rec, testLayer(high, "high"), []domain.CatalogLayer{testLayer(low, "low")}, false)
	for _, r := range c.Shadows() {
		if r.Item == injectionsItem(ghcp) {
			t.Errorf("unexpected shadow record for a harness in one layer only: %+v", r)
		}
	}
}

func TestStack_HarnessInjections_DeclaredFlagFollowsWinnersDeclaration(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	claude, ghcp := catalogpaths.HarnessDirNameClaudeCode, catalogpaths.HarnessDirNameGhcpCLI
	withTestOverrides(t, high, "high", "  harness_injections: [\""+claude+"\"]\n")
	for _, root := range []string{high, low} {
		writeInjectionsFile(t, root, claude, "HarnessInjections.md")
		writeInjectionsFile(t, root, ghcp, "HarnessInjections.md")
	}
	c := newTestInjectionsStack(t, mosaic, high, low)

	records := c.Shadows()

	lower := []domain.CatalogLayer{testLayer(low, "low")}
	assertShadow(t, shadowFor(t, records, injectionsItem(claude)), testLayer(high, "high"), lower, true)
	assertShadow(t, shadowFor(t, records, injectionsItem(ghcp)), testLayer(high, "high"), lower, false)
}

func TestStack_HarnessInjections_NoSupplierMeansNoProvenanceForThatHarnessOnly(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	only := newTestCatalogRoot(t, "only")
	writeInjectionsFile(t, only, catalogpaths.HarnessDirNameClaudeCode, "HarnessInjections.md")
	c := newTestInjectionsStack(t, mosaic, only)

	if _, ok := c.Provenance(injectionsItem(catalogpaths.HarnessDirNameOpenCode)); ok {
		t.Error("Provenance for an unsupplied harness must report ok=false")
	}
	if layer, ok := c.Provenance(injectionsItem(catalogpaths.HarnessDirNameClaudeCode)); !ok || layer != testLayer(only, "only") {
		t.Errorf("Provenance(claude) = %+v, %v, want the only layer", layer, ok)
	}
}

// injDir is the expected injection directory of a harness in a catalog root, spelled out
// independently of the production path builder.
func injDir(root, harnessDir string) string {
	return filepath.Join(root, "HarnessInjections", harnessDir)
}
