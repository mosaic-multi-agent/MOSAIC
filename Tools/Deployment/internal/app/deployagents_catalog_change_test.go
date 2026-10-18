package app_test

// deployagents_catalog_change_test.go verifies how the deploy-agents flow reports and writes an
// orchestrator-role file whose recorded catalog is no longer the catalog supplying it. The file
// joins the run only for infrastructure declaration drift; when it does, its plan item keeps the
// catalog change next to the declaration drift, the rewrite takes the body from the catalog in
// effect (keeping the deployed workflows and model), and the manifest stamp names that catalog.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	dacOldCatalogID = "orchonly"
	dacNewCatalogID = "mosaic-default"

	// dacDeployedBodyMarker is prose of the deployed file that only the old supplier's body had.
	dacDeployedBodyMarker = "BODY-FROM-OLD-SUPPLIER"
	// dacCurrentBodyMarker is prose of the source the catalog in effect supplies.
	dacCurrentBodyMarker = "BODY-FROM-CURRENT-SUPPLIER"
)

// supplierCatalog is the injection fixture catalog that also reports which catalog supplies the
// orchestrator-role agents.
type supplierCatalog struct {
	*stubCatalog
	layer domain.CatalogLayer
}

func newSupplierCatalog(cat *stubCatalog, supplier domain.CatalogIdentity) *supplierCatalog {
	return &supplierCatalog{stubCatalog: cat, layer: domain.CatalogLayer{Identity: supplier}}
}

func (c *supplierCatalog) Provenance(item domain.CatalogItem) (domain.CatalogLayer, bool) {
	if item.Kind == domain.CatalogItemAgent && strings.HasPrefix(item.Key, "orchestrator") {
		return c.layer, true
	}
	return domain.CatalogLayer{}, false
}

var (
	dacRecorded = domain.CatalogIdentity{ID: dacOldCatalogID, Name: "Orchestrator Only", Version: "1.0.0"}
	dacSupplier = domain.CatalogIdentity{ID: dacNewCatalogID, Name: "Default", Version: "2.0.0"}
)

// dacDeployedOrchestrator is the deployed file: a body only the old supplier had, the given
// declarations, and the locally preserved workflows.
func dacDeployedOrchestrator(infraInner string) string {
	return strings.Replace(injDeployedOrchestrator(infraInner, injWorkflowInner), "# Orchestrator", dacDeployedBodyMarker, 1)
}

// dacCurrentSource is the orchestrator source the catalog in effect supplies.
func dacCurrentSource() []byte {
	return []byte(strings.Replace(orchestratorMainSourceDoc, "<AvailableWorkflows", dacCurrentBodyMarker+"\n\n<AvailableWorkflows", 1))
}

// dacConfig seeds one manifested orchestrator that lacks the security-scanner declaration (so
// declaration drift admits it), recorded under recorded while supplier supplies it now.
func dacConfig(recorded domain.CatalogIdentity) injectionConfig {
	return injectionConfig{
		files:      map[string]string{injOrchestratorPath: dacDeployedOrchestrator("")},
		manifested: true,
		infraIDs:   []string{injSecurityKey},
		recorded:   recorded,
		supplier:   dacSupplier,
		customizeCatalog: func(c *stubCatalog) {
			c.sources[c.orchestrator.SourcePath] = dacCurrentSource()
		},
	}
}

// dacCatalogDelta returns the catalog delta of item, if any.
func dacCatalogDelta(item domain.PlanItem) (domain.VersionDelta, bool) {
	for _, d := range item.Stale {
		if d.Field == "source_catalog" {
			return d, true
		}
	}
	return domain.VersionDelta{}, false
}

func TestDeployAgents_OrchestratorWithDeclarationDriftAndCatalogChange_ItemNamesBothCauses(t *testing.T) {
	run := runInfraInjection(t, dacConfig(dacRecorded))

	requireRunOK(t, run)
	item, ok := run.planItem(injOrchestratorPath)
	if !ok {
		t.Fatalf("the executor plan has no item for %s", injOrchestratorPath)
	}
	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q (reason %q), want %q", item.Action, item.Reason, domain.ActionUpdate)
	}
	d, has := dacCatalogDelta(item)
	if !has || d.Deployed != dacOldCatalogID || d.Source != dacNewCatalogID {
		t.Errorf("Stale = %+v, want a catalog delta from %q to %q", item.Stale, dacOldCatalogID, dacNewCatalogID)
	}
	if last := item.Stale[len(item.Stale)-1]; last.Field != "source_catalog" {
		t.Errorf("last delta = %+v, want the catalog delta after the declaration deltas", last)
	}
	if len(item.Stale) < 2 {
		t.Errorf("Stale = %+v, want the declaration drift deltas kept before the catalog delta", item.Stale)
	}
	for _, want := range []string{injSecurityKey, dacOldCatalogID, dacNewCatalogID} {
		if !strings.Contains(item.Reason, want) {
			t.Errorf("Reason = %q, want it to name %q", item.Reason, want)
		}
	}
	if !strings.HasPrefix(item.Reason, "stale: ") {
		t.Errorf("Reason = %q, want it to start with \"stale: \"", item.Reason)
	}
}

func TestDeployAgents_OrchestratorWithDeclarationDriftAndSameCatalog_ItemHasNoCatalogDelta(t *testing.T) {
	run := runInfraInjection(t, dacConfig(dacSupplier))

	requireRunOK(t, run)
	item, ok := run.planItem(injOrchestratorPath)
	if !ok {
		t.Fatalf("the executor plan has no item for %s", injOrchestratorPath)
	}
	if _, has := dacCatalogDelta(item); has {
		t.Errorf("Stale = %+v, want no catalog delta when the recorded catalog supplies the file", item.Stale)
	}
	if !strings.Contains(item.Reason, injSecurityKey) {
		t.Errorf("Reason = %q, want the declaration drift", item.Reason)
	}
}

func TestDeployAgents_OrchestratorWithCatalogChangeOnly_IsNotPlanned(t *testing.T) {
	cfg := dacConfig(dacRecorded)
	cfg.files = map[string]string{injOrchestratorPath: dacDeployedOrchestrator(injSection(injSecurityBlock("1.1")))}

	run := runInfraInjection(t, cfg)

	requireRunOK(t, run)
	if item, ok := run.planItem(injOrchestratorPath); ok {
		t.Errorf("plan holds %+v for a file with current declarations, want it left out of deploy agents", item)
	}
	if got, _ := run.readFile(t, injOrchestratorPath); got != cfg.files[injOrchestratorPath] {
		t.Error("the file was rewritten although only its catalog changed")
	}
}

func TestDeployAgents_OrchestratorConflict_ReasonStillCarriesDeclarationDrift(t *testing.T) {
	cfg := dacConfig(dacRecorded)
	cfg.manifested = false
	cfg.conflictDefault = domain.DecisionOverwrite

	run := runInfraInjection(t, cfg)

	requireRunOK(t, run)
	item, ok := run.planItem(injOrchestratorPath)
	if !ok {
		t.Fatalf("the executor plan has no item for %s", injOrchestratorPath)
	}
	if item.Action != domain.ActionConflict {
		t.Fatalf("Action = %q, want %q for a file with no manifest record", item.Action, domain.ActionConflict)
	}
	if !strings.Contains(item.Reason, injSecurityKey) {
		t.Errorf("Reason = %q, want the declaration drift appended", item.Reason)
	}
}

func TestDeployAgents_OrchestratorWithCatalogChange_WritesCurrentSupplierBodyKeepsWorkflowsAndModel(t *testing.T) {
	run := runInfraInjection(t, dacConfig(dacRecorded))

	requireRunOK(t, run)
	got, _ := run.readFile(t, injOrchestratorPath)
	if !strings.Contains(got, dacCurrentBodyMarker) {
		t.Errorf("the rewritten file does not carry the body of the catalog in effect:\n%s", got)
	}
	if strings.Contains(got, dacDeployedBodyMarker) {
		t.Errorf("the rewritten file still carries the body of the old supplier:\n%s", got)
	}
	if w := injRegion(t, got, "AvailableWorkflows"); w != injWorkflowInner {
		t.Errorf("workflows changed on rewrite:\n%s", w)
	}
	if !strings.Contains(got, "model: model-b") {
		t.Errorf("the deployed model was not kept:\n%s", got)
	}
	if !strings.Contains(injRegion(t, got, "InfrastructureAgents"), injSection(injSecurityBlock("1.1"))) {
		t.Error("the declaration was not written")
	}
}

func TestDeployAgents_OrchestratorWithCatalogChange_StampNamesCurrentSupplier(t *testing.T) {
	run := runInfraInjection(t, dacConfig(dacRecorded))

	requireRunOK(t, run)
	stamp, ok := run.rec.req.VersionStamps[injOrchestratorPath]
	if !ok {
		t.Fatalf("no version stamp for %s", injOrchestratorPath)
	}
	if stamp.Catalog != dacSupplier {
		t.Errorf("stamp Catalog = %+v, want the supplier %+v", stamp.Catalog, dacSupplier)
	}
}
