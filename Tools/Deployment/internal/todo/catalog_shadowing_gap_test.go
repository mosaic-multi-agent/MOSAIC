package todo_test

// catalog_shadowing_gap_test.go verifies that a catalog-shadowing gap is recorded under the
// manual-steps category, with its subject and detail carried verbatim.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

// TestCollector_CatalogShadowing_ManualStepsCategory verifies the category and field mapping
// of the catalog-shadowing gap.
func TestCollector_CatalogShadowing_ManualStepsCategory(t *testing.T) {
	c := todo.NewCollector()

	c.AddGap(domain.Gap{
		Kind:    domain.GapCatalogShadowing,
		Subject: "agent:code-reviewer",
		Detail:  "from user-a 1.0.0 shadows mosaic-default 0.7.0; undeclared override",
	})

	items := c.Items()
	if len(items) != 1 {
		t.Fatalf("Items() len = %d, want 1", len(items))
	}
	it := items[0]
	if it.Category != domain.TodoManual {
		t.Errorf("Category = %q, want %q", it.Category, domain.TodoManual)
	}
	if it.Subject != "agent:code-reviewer" || !strings.Contains(it.Detail, "undeclared override") {
		t.Errorf("item = %+v, want the gap's subject and detail carried over", it)
	}
	if it.Owner != "" || it.Fragment != "" {
		t.Errorf("Owner = %q, Fragment = %q, want both empty for a run-level gap", it.Owner, it.Fragment)
	}
}
