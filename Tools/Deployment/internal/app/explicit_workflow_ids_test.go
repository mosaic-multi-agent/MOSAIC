package app

// explicit_workflow_ids_test.go specifies classifyExplicitWorkflowIDs: ids the catalog supplies
// are catalog ids, ids deployed but not supplied are kept ids, and any other id is an error
// that names it.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// classifyFakeCatalog supplies the workflows named in ids and nothing else.
type classifyFakeCatalog struct {
	catalog.Catalog
	ids []string
}

func (c classifyFakeCatalog) CatalogRoot() string { return "fake-catalog" }
func (c classifyFakeCatalog) Workflows() []domain.Workflow {
	out := make([]domain.Workflow, 0, len(c.ids))
	for _, id := range c.ids {
		out = append(out, domain.Workflow{ID: id, Name: id, Version: "1.0"})
	}
	return out
}
func (c classifyFakeCatalog) Workflow(id string) (domain.Workflow, bool) {
	for _, w := range c.Workflows() {
		if w.ID == id {
			return w, true
		}
	}
	return domain.Workflow{}, false
}

func TestClassifyExplicitWorkflowIDs_SplitsCatalogAndKeptIdsInInputOrder(t *testing.T) {
	cat := classifyFakeCatalog{ids: []string{"quick-fix", "other-flow"}}
	deployedCustom := map[string]bool{"custom-x": true, "custom-y": true}

	catalogIDs, keptIDs, err := classifyExplicitWorkflowIDs(cat, []string{"custom-x", "quick-fix", "custom-y", "other-flow"}, deployedCustom)

	if err != nil {
		t.Fatalf("classifyExplicitWorkflowIDs returned error: %v", err)
	}
	if got := strings.Join(catalogIDs, ","); got != "quick-fix,other-flow" {
		t.Errorf("catalog ids = %q, want %q", got, "quick-fix,other-flow")
	}
	if got := strings.Join(keptIDs, ","); got != "custom-x,custom-y" {
		t.Errorf("kept ids = %q, want %q", got, "custom-x,custom-y")
	}
}

func TestClassifyExplicitWorkflowIDs_IdTheCatalogSuppliesIsNeverKept(t *testing.T) {
	cat := classifyFakeCatalog{ids: []string{"quick-fix"}}
	deployedCustom := map[string]bool{"quick-fix": true}

	catalogIDs, keptIDs, err := classifyExplicitWorkflowIDs(cat, []string{"quick-fix"}, deployedCustom)

	if err != nil {
		t.Fatalf("classifyExplicitWorkflowIDs returned error: %v", err)
	}
	if len(catalogIDs) != 1 || catalogIDs[0] != "quick-fix" || len(keptIDs) != 0 {
		t.Errorf("catalog ids = %v, kept ids = %v, want the id as a catalog id only", catalogIDs, keptIDs)
	}
}

func TestClassifyExplicitWorkflowIDs_IdNeitherInCatalogNorDeployed_ErrorNamesIt(t *testing.T) {
	cat := classifyFakeCatalog{ids: []string{"quick-fix"}}
	deployedCustom := map[string]bool{"custom-x": true}

	_, _, err := classifyExplicitWorkflowIDs(cat, []string{"quick-fix", "custom-x", "no-such-flow"}, deployedCustom)

	if err == nil {
		t.Fatal("classifyExplicitWorkflowIDs returned no error for an id that is neither in the catalog nor deployed")
	}
	if !strings.Contains(err.Error(), "no-such-flow") {
		t.Errorf("error %q does not name the unknown id", err.Error())
	}
	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Errorf("error %v is not the unknown-workflow error", err)
	}
}
