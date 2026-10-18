package stack_test

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// provenanceStack is what the app layer reaches through the stack's companion interface:
// provenance lookup and shadow records.
type provenanceStack interface {
	stack.Catalog
	catalog.ProvenanceReporter
	Shadows() []stackrec.Shadow
}

// newTestProvenanceStack composes roots (highest first) and fails the test when the resolved
// catalog does not expose provenance and shadow records through its companion interface.
func newTestProvenanceStack(t *testing.T, mosaicRoot string, roots ...string) provenanceStack {
	t.Helper()
	c := newTestStack(t, mosaicRoot, roots...)
	p, ok := c.(provenanceStack)
	if !ok {
		t.Fatalf("stack.Catalog %T does not expose Provenance and Shadows", c)
	}
	return p
}

// withTestOverrides rewrites root's identity file so it declares the given overrides block
// (YAML text under the "overrides:" key, already indented by two spaces).
func withTestOverrides(t *testing.T, root, id, overridesBody string) {
	t.Helper()
	mustWrite(t, root, "catalog-identity.yaml",
		"id: "+id+"\nname: Catalog "+id+"\nversion: \"1.0.0\"\noverrides:\n"+overridesBody)
}

// testLayer is the layer carrier expected for a catalog root created by newTestCatalogRoot.
func testLayer(root, id string) domain.CatalogLayer {
	return domain.CatalogLayer{
		Identity: domain.CatalogIdentity{ID: id, Name: "Catalog " + id, Version: "1.0.0"},
		Root:     root,
	}
}

func agentItem(key string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: key}
}

func pairItem() domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair}
}

// addTestOrchestratorPair writes both orchestrator files into root.
func addTestOrchestratorPair(t *testing.T, root, version string) {
	t.Helper()
	addTestOrchestrator(t, root, version)
	addTestOrchestratorScript(t, root, version)
}

// shadowFor returns the record for item, failing the test unless there is exactly one.
func shadowFor(t *testing.T, records []stackrec.Shadow, item domain.CatalogItem) stackrec.Shadow {
	t.Helper()
	var found []stackrec.Shadow
	for _, r := range records {
		if r.Item == item {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one shadow record for %+v, got %d in %+v", item, len(found), records)
	}
	return found[0]
}

// assertShadow checks winner, shadowed layers (in order) and the declared flag of a record.
func assertShadow(t *testing.T, got stackrec.Shadow, winner domain.CatalogLayer, shadowed []domain.CatalogLayer, declared bool) {
	t.Helper()
	if !reflect.DeepEqual(got.Winner, winner) {
		t.Errorf("%+v: Winner = %+v, want %+v", got.Item, got.Winner, winner)
	}
	if !reflect.DeepEqual(got.Shadowed, shadowed) {
		t.Errorf("%+v: Shadowed = %+v, want %+v", got.Item, got.Shadowed, shadowed)
	}
	if got.Declared != declared {
		t.Errorf("%+v: Declared = %v, want %v", got.Item, got.Declared, declared)
	}
}
