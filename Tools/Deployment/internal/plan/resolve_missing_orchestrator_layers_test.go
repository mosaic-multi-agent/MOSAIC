package plan_test

// resolve_missing_orchestrator_layers_test.go verifies that the missing-orchestrator refusal
// names the catalog layers that were searched when the catalog is composed from layers, and
// keeps naming the catalog root when it is not.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// layeredFakeCatalog is an orchestrator-less fakeCatalog that also lists its layers.
type layeredFakeCatalog struct {
	*fakeCatalog
	layers []domain.CatalogLayer
}

func (c layeredFakeCatalog) Layers() []domain.CatalogLayer { return c.layers }

func testLayer(id, root string, isDefault bool) domain.CatalogLayer {
	return domain.CatalogLayer{Identity: domain.CatalogIdentity{ID: id, Name: id, Version: "1.0.0"}, Root: root, Default: isDefault}
}

func TestResolveArtifactsFrom_LayeredCatalogWithoutOrchestrator_NamesEveryLayerSearched(t *testing.T) {
	cat := layeredFakeCatalog{
		fakeCatalog: orchestratorlessCatalog("/fake/top-layer-root"),
		layers: []domain.CatalogLayer{
			testLayer("team-extras", "/fake/team", false),
			testLayer("company-base", "/fake/company", false),
			testLayer("mosaic-default", "/fake/mosaic/Catalog", true),
		},
	}

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{})

	if !errors.Is(err, plan.ErrMissingOrchestrator) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrMissingOrchestrator)", err)
	}
	for _, l := range cat.layers {
		if !strings.Contains(err.Error(), l.Identity.ID) || !strings.Contains(err.Error(), l.Root) {
			t.Errorf("error %q does not name the searched layer %s (%s)", err.Error(), l.Identity.ID, l.Root)
		}
	}
}

func TestResolveArtifactsFrom_LayeredCatalogWithoutOrchestrator_ListsLayersHighestFirst(t *testing.T) {
	cat := layeredFakeCatalog{
		fakeCatalog: orchestratorlessCatalog("/fake/top-layer-root"),
		layers: []domain.CatalogLayer{
			testLayer("first-layer", "/fake/one", false),
			testLayer("second-layer", "/fake/two", false),
		},
	}

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{})

	if err == nil {
		t.Fatal("expected ErrMissingOrchestrator, got nil")
	}
	first, second := strings.Index(err.Error(), "first-layer"), strings.Index(err.Error(), "second-layer")
	if first < 0 || second < 0 || first > second {
		t.Errorf("error %q must list first-layer before second-layer (highest layer first)", err.Error())
	}
}

func TestResolveArtifactsFrom_CatalogWithoutLayers_StillNamesTheCatalogRoot(t *testing.T) {
	cat := orchestratorlessCatalog("/fake/plain-catalogue")

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{})

	if !errors.Is(err, plan.ErrMissingOrchestrator) || !strings.Contains(err.Error(), "/fake/plain-catalogue") {
		t.Errorf("error = %v, want ErrMissingOrchestrator naming /fake/plain-catalogue", err)
	}
}
