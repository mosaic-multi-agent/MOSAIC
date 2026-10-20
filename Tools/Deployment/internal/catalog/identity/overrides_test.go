package identity_test

import (
	"testing"

	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/domain"
)

func TestNewOverrides_DeclaresExactlyGivenItems(t *testing.T) {
	a := item(domain.CatalogItemAgent, "a")
	s := item(domain.CatalogItemSkill, "a")

	o := identity.NewOverrides(a)

	if !o.Declares(a) {
		t.Error("expected agent a declared")
	}
	if o.Declares(s) {
		t.Error("skill a must not be declared")
	}
}

func TestNewOverrides_DuplicatesCollapse(t *testing.T) {
	w := item(domain.CatalogItemWorkflow, "w")

	o := identity.NewOverrides(w, w, w)

	assertItems(t, o.Items(), []domain.CatalogItem{w})
}

func TestNewOverrides_NoItemsDeclaresNothing(t *testing.T) {
	o := identity.NewOverrides()

	if n := len(o.Items()); n != 0 {
		t.Fatalf("Items() len = %d, want 0", n)
	}
	if o.Declares(item(domain.CatalogItemOrchestratorPair, "")) {
		t.Error("empty overrides must declare nothing")
	}
}

func TestZeroOverrides_DeclaresNothing(t *testing.T) {
	var o identity.Overrides

	if len(o.Items()) != 0 || o.Declares(item(domain.CatalogItemAgent, "x")) {
		t.Error("zero Overrides must declare nothing")
	}
}

func TestNewOverrides_ItemsSortedByKindThenKey(t *testing.T) {
	o := identity.NewOverrides(
		item(domain.CatalogItemWorkflow, "b"),
		item(domain.CatalogItemAgent, "z"),
		item(domain.CatalogItemWorkflow, "a"),
		item(domain.CatalogItemAgent, "m"),
	)

	assertItems(t, o.Items(), []domain.CatalogItem{
		item(domain.CatalogItemAgent, "m"),
		item(domain.CatalogItemAgent, "z"),
		item(domain.CatalogItemWorkflow, "a"),
		item(domain.CatalogItemWorkflow, "b"),
	})
}

// Orchestrator agent items built by hand match only themselves, never the pair.
func TestNewOverrides_OrchestratorAgentsNeverMatchPair(t *testing.T) {
	orch := item(domain.CatalogItemAgent, "orchestrator")
	pair := item(domain.CatalogItemOrchestratorPair, "")

	o := identity.NewOverrides(orch)

	if !o.Declares(orch) {
		t.Error("expected the orchestrator agent item to match itself")
	}
	if o.Declares(pair) {
		t.Error("orchestrator agent item must not declare the pair")
	}
	if identity.NewOverrides(pair).Declares(orch) {
		t.Error("pair item must not declare the orchestrator agent")
	}
}
