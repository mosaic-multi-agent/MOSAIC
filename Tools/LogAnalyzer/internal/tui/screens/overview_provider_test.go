package screens_test

// By-model rows of the overview name the provider, so one model under two
// providers shows as two distinguishable rows.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestOverview_ByModelRowsNameTheProvider(t *testing.T) {
	run := overviewRun()
	run.Overview.ByModel = []domain.ModelFigures{
		{Model: "shared-model", Provider: "prov-one", Priced: true, Requests: 2, Totals: run.Overview.Totals},
		{Model: "shared-model", Provider: "prov-two", Priced: true, Requests: 1, Totals: run.Overview.Totals},
		{Model: "shared-model", Priced: true, Requests: 1, Totals: run.Overview.Totals},
	}

	view := overviewView(run, tallH)

	assertContains(t, view, "shared-model [prov-one]", "shared-model [prov-two]")
	if n := strings.Count(view, "shared-model"); n != 3 {
		t.Errorf("view names shared-model %d times, want 3 rows", n)
	}
}
