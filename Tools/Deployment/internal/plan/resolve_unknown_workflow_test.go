package plan_test

// resolve_unknown_workflow_test.go verifies that an unknown workflow id is reported together
// with the catalogs that were searched, while still matching plan.ErrUnknownWorkflow.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

func layeredCatalogWithOrchestrator(layers ...domain.CatalogLayer) layeredFakeCatalog {
	return layeredFakeCatalog{
		fakeCatalog: &fakeCatalog{root: "/fake/top-layer-root", orchestrator: makeOrchestrator()},
		layers:      layers,
	}
}

func TestResolveArtifactsFrom_UnknownWorkflowInStack_NamesIdAndEverySearchedCatalog(t *testing.T) {
	cat := layeredCatalogWithOrchestrator(
		testLayer("team-extras", "/fake/team", false),
		testLayer("mosaic-default", "/fake/mosaic/Catalog", true),
	)

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{WorkflowIDs: []string{"team-review"}})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), "team-review") {
		t.Errorf("error %q does not name the workflow id", err.Error())
	}
	for _, l := range cat.layers {
		if !strings.Contains(err.Error(), l.Identity.ID) {
			t.Errorf("error %q does not name the searched catalog %q", err.Error(), l.Identity.ID)
		}
	}
}

func TestResolveArtifactsFrom_UnknownWorkflowInStack_ListsCatalogsHighestFirst(t *testing.T) {
	cat := layeredCatalogWithOrchestrator(
		testLayer("first-layer", "/fake/one", false),
		testLayer("second-layer", "/fake/two", false),
	)

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{WorkflowIDs: []string{"missing-wf"}})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	first, second := strings.Index(err.Error(), "first-layer"), strings.Index(err.Error(), "second-layer")
	if first < 0 || second < 0 || first > second {
		t.Errorf("error %q must list first-layer before second-layer", err.Error())
	}
}

func TestResolveArtifactsFrom_UnknownWorkflowInPlainCatalog_NamesCatalogRoot(t *testing.T) {
	cat := &fakeCatalog{root: "/fake/plain-catalogue", orchestrator: makeOrchestrator()}

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{WorkflowIDs: []string{"missing-wf"}})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), "missing-wf") || !strings.Contains(err.Error(), "/fake/plain-catalogue") {
		t.Errorf("error %q must name the workflow id and the catalog root", err.Error())
	}
}

func TestResolveArtifactsFrom_UnknownWorkflow_StartsWithWorkflowNotFoundAndQuotedId(t *testing.T) {
	cat := layeredCatalogWithOrchestrator(testLayer("team-extras", "/fake/team", false))

	_, err := plan.ResolveArtifactsFrom(cat, plan.Selection{WorkflowIDs: []string{"team-review"}})

	if err == nil || !strings.HasPrefix(err.Error(), `workflow not found: "team-review"`) {
		t.Errorf("error = %v, want text starting with `workflow not found: \"team-review\"`", err)
	}
}
