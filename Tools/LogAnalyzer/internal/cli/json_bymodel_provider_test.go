package cli_test

// The report JSON names the provider of every by_model row, so one model
// priced under two providers stays distinguishable. A row whose requests
// logged no provider carries no provider key.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestEncodeReport_Overview_ByModelCarriesProvider(t *testing.T) {
	run := vwRun()
	run.Overview.ByModel = []domain.ModelFigures{
		{Model: "shared", Provider: "p1", Priced: true, Requests: 2, Totals: vwTotals(100, true)},
		{Model: "shared", Provider: "p2", Priced: false, Requests: 1, Totals: vwTotals(0, false)},
		{Model: "shared", Priced: true, Requests: 1, Totals: vwTotals(50, true)},
	}

	ov := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(run))), "overview")

	models := vwList(t, ov, "by_model")
	if len(models) != 3 {
		t.Fatalf("by_model has %d entries, want 3", len(models))
	}
	if p := vwObj(t, models[0])["provider"]; p != "p1" {
		t.Errorf("first row provider = %v, want p1", p)
	}
	if p := vwObj(t, models[1])["provider"]; p != "p2" {
		t.Errorf("second row provider = %v, want p2", p)
	}
	if p, present := vwObj(t, models[2])["provider"]; present {
		t.Errorf("row without provider has provider key %v, want it omitted", p)
	}
}
