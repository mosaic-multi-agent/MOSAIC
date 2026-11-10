package screens_test

// review_catalog_delta_test.go verifies the plan review shows the catalog-change delta of an
// updated item with a user-facing label and both catalog ids, not the raw field name. The TUI
// does not import the plan package, so the field value is used literally.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func TestReviewScreen_CatalogDelta_ShowsLabelAndBothCatalogIds(t *testing.T) {
	p := planWithItems(domain.ModeUpdateWorkspace, []domain.PlanItem{
		{
			Ref:    agentRef("some-agent"),
			Action: domain.ActionUpdate,
			Reason: "stale: supplied by a different catalog",
			Stale: []domain.VersionDelta{
				{Field: "source_catalog", Deployed: "orchonly", Source: "mosaic-default"},
			},
		},
	})
	s := screens.NewReviewScreen(p, 120, 80, plainStyles())

	view := s.View()

	if !strings.Contains(view, "source catalog") {
		t.Errorf("plan review does not show the user-facing label \"source catalog\":\n%s", view)
	}
	if strings.Contains(view, "source_catalog") {
		t.Errorf("plan review exposes the raw field name \"source_catalog\":\n%s", view)
	}
	for _, id := range []string{"orchonly", "mosaic-default"} {
		if !strings.Contains(view, id) {
			t.Errorf("plan review does not name catalog %q:\n%s", id, view)
		}
	}
}
