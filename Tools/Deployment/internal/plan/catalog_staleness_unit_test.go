package plan_test

// catalog_staleness_unit_test.go covers the pure catalog-staleness functions directly
// (CatalogStaleness, CatalogDrift, CurrentCatalog) and the position of the catalog delta and
// reason after infrastructure drift in a planned orchestrator.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

func TestCatalogStaleness_IdsKnownAndDifferent_IsStaleWithDeltaAndReason(t *testing.T) {
	recorded := domain.CatalogIdentity{ID: "orchonly", Name: "A", Version: "1"}
	current := domain.CatalogIdentity{ID: "mosaic-default", Name: "B", Version: "2"}

	d := plan.CatalogStaleness(recorded, current)

	if !d.Stale() {
		t.Fatal("Stale() = false, want true for two known, different ids")
	}
	delta, ok := d.Delta()
	want := domain.VersionDelta{Field: plan.CatalogDeltaField, Deployed: "orchonly", Source: "mosaic-default"}
	if !ok || delta != want {
		t.Errorf("Delta() = %+v, %v, want %+v", delta, ok, want)
	}
	for _, id := range []string{"orchonly", "mosaic-default"} {
		if !strings.Contains(d.Reason(), id) {
			t.Errorf("Reason() = %q, want it to name %q", d.Reason(), id)
		}
	}
}

func TestCatalogStaleness_NotStaleCases_HaveNoDeltaAndNoReason(t *testing.T) {
	cases := []struct {
		name              string
		recorded, current domain.CatalogIdentity
	}{
		{"both unknown", domain.CatalogIdentity{}, domain.CatalogIdentity{}},
		{"recorded unknown", domain.CatalogIdentity{}, domain.CatalogIdentity{ID: "x"}},
		{"current unknown", domain.CatalogIdentity{ID: "x"}, domain.CatalogIdentity{}},
		{"recorded has name and version but no id", domain.CatalogIdentity{Name: "N", Version: "1"}, domain.CatalogIdentity{ID: "x"}},
		{"current has name and version but no id", domain.CatalogIdentity{ID: "x"}, domain.CatalogIdentity{Name: "N", Version: "1"}},
		{"same id, different name and version", domain.CatalogIdentity{ID: "x", Name: "A", Version: "1"}, domain.CatalogIdentity{ID: "x", Name: "B", Version: "2"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := plan.CatalogStaleness(tc.recorded, tc.current)

			if d.Stale() {
				t.Error("Stale() = true, want false")
			}
			if delta, ok := d.Delta(); ok {
				t.Errorf("Delta() = %+v, true, want no delta", delta)
			}
			if r := d.Reason(); r != "" {
				t.Errorf("Reason() = %q, want empty", r)
			}
		})
	}
}

func TestCatalogDrift_ReasonIsReproducibleFromTheDelta(t *testing.T) {
	d := plan.CatalogStaleness(domain.CatalogIdentity{ID: "orchonly", Name: "A"}, domain.CatalogIdentity{ID: "mosaic-default", Version: "2"})
	delta, _ := d.Delta()

	again := plan.CatalogStaleness(domain.CatalogIdentity{ID: delta.Deployed}, domain.CatalogIdentity{ID: delta.Source})

	if again.Reason() != d.Reason() || d.Reason() == "" {
		t.Errorf("Reason from delta = %q, original = %q, want equal and non-empty", again.Reason(), d.Reason())
	}
}

func TestCurrentCatalog_ReportsTheSupplierOnlyWhereItIsKnown(t *testing.T) {
	o := ccCatalogOptions{}
	stack := &provenanceCatalog{fakeCatalog: ccFakeCatalog(o), layers: ccSupplier(ccNewCatalogID)}
	cases := []struct {
		name string
		cat  catalog.Catalog
		ref  domain.ArtifactRef
		want string
	}{
		{"agent", stack, domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "worker"}, ccNewCatalogID},
		{"orchestrator script agent", stack, domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "orchestrator-script"}, ccNewCatalogID},
		{"skill", stack, domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "lean-tdd"}, ccNewCatalogID},
		{"hook", stack, domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "pre-commit"}, ccNewCatalogID},
		{"item the catalog does not know", stack, domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "ghost"}, ""},
		{"kind other than agent, skill or hook", stack, domain.ArtifactRef{Kind: domain.ArtifactKind("workflow"), Key: "worker"}, ""},
		{"catalog without provenance", ccFakeCatalog(o), domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "worker"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := plan.CurrentCatalog(tc.cat, tc.ref)

			if ok != (tc.want != "") || id.ID != tc.want {
				t.Errorf("CurrentCatalog = %+v, %v, want id %q (ok %v)", id, ok, tc.want, tc.want != "")
			}
		})
	}
}

// TestBuild_CatalogChanged_OrchestratorWithInfrastructureDrift_CatalogDeltaAndReasonComeLast
// verifies the catalog delta follows the infrastructure deltas and the catalog reason follows the
// infrastructure reason part, both joined into one update item.
func TestBuild_CatalogChanged_OrchestratorWithInfrastructureDrift_CatalogDeltaAndReasonComeLast(t *testing.T) {
	o := ccCatalogOptions{recorded: domain.CatalogIdentity{ID: ccOldCatalogID}}
	fake := ccFakeCatalog(o)
	fake.infraAgents = []domain.Agent{infraCatalogAgent("code-review", "1.0")}
	cat := &provenanceCatalog{fakeCatalog: fake, layers: ccSupplier(ccNewCatalogID)}

	p := ccBuildWith(t, cat, ccDeployedStates(o), ccManifestEntries(o.recorded), func(in *plan.Input) {
		in.Mode = domain.ModeUpdateWorkspace
		in.ScannedAgentKeys = []string{"worker", "orchestrator", "orchestrator-script"}
		in.InfrastructureDeclarations = ensureIntent("code-review")
	})

	item := ccItemAt(t, p, ccOrchTarget)
	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q (reason %q), want %q", item.Action, item.Reason, domain.ActionUpdate)
	}
	if !hasInfraDelta(item, "code-review") {
		t.Fatalf("Stale = %+v, want an infrastructure delta for code-review", item.Stale)
	}
	if last := item.Stale[len(item.Stale)-1]; last.Field != ccCatalogDeltaField {
		t.Errorf("last delta = %+v, want the catalog delta after the infrastructure delta", last)
	}
	parts := strings.Split(strings.TrimPrefix(item.Reason, "stale: "), "; ")
	last := parts[len(parts)-1]
	if !strings.Contains(last, ccOldCatalogID) || !strings.Contains(last, ccNewCatalogID) {
		t.Errorf("Reason = %q, want the catalog reason as its last part", item.Reason)
	}
	if !strings.Contains(item.Reason, "code-review") {
		t.Errorf("Reason = %q, want the infrastructure reason kept", item.Reason)
	}
}
