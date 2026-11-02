package todo_test

// collector_kind_test.go verifies that items built from gaps remember the gap kind (so the
// console can pick out individual warnings) and that items added directly carry none.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

func TestCollector_AddGap_ItemCarriesGapKind(t *testing.T) {
	kinds := []domain.GapKind{
		domain.GapUndeliveredOrchestratorInjection,
		domain.GapRoleForbiddenRegion,
		domain.GapNoModel,
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			c := todo.NewCollector()

			c.AddGap(domain.Gap{Kind: kind, Subject: "subject", Owner: "owner", Detail: "detail"})

			items := c.Items()
			if len(items) != 1 || items[0].Kind != kind {
				t.Errorf("items = %+v, want one item with Kind %q", items, kind)
			}
		})
	}
}

func TestCollector_Add_DirectItemHasNoKind(t *testing.T) {
	c := todo.NewCollector()

	c.Add(domain.TodoItem{Category: domain.TodoManual, Subject: "direct"})

	if items := c.Items(); len(items) != 1 || items[0].Kind != "" {
		t.Errorf("items = %+v, want one item without a kind", items)
	}
}
