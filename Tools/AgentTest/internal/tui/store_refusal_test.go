package tui

// Tests that the store-result screen shows a report refused for an unverified
// subject model as refused, with its reason, and counts it in the summary line.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/resultstore"
)

func refusedStoreResult() resultstore.StoreResult {
	return resultstore.StoreResult{
		Stored:                 1,
		SkippedUnverifiedModel: 1,
		Reports: []resultstore.StoredReport{
			{
				SourcePath: "/r/good.json",
				TargetPath: "/T/good.json",
			},
			{
				SourcePath: "/r/bad.json",
				Skipped:    true,
				SkipReason: resultstore.SkipUnverifiedModel,
				Message:    "no observed model",
			},
		},
	}
}

func TestStoreFlow_Result_ShowsRefusedReportWithReason(t *testing.T) {
	o := newProcessOptions()
	o.Store = fakeStoreFunc(refusedStoreResult(), nil)
	m := advanceToStoreResult(t, NewModel(o), "/tmp/reports")

	view := safeView(t, m)

	var line string
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "/r/bad.json") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("store-result view does not list the refused report:\n%s", view)
	}
	if !strings.Contains(strings.ToLower(line), "refused") {
		t.Errorf("refused report line %q is not shown as refused", line)
	}
	if !strings.Contains(line, "no observed model") {
		t.Errorf("refused report line %q does not show the reason", line)
	}
}

func TestStoreFlow_Result_SummaryLineCountsRefusedReports(t *testing.T) {
	o := newProcessOptions()
	o.Store = fakeStoreFunc(refusedStoreResult(), nil)
	m := advanceToStoreResult(t, NewModel(o), "/tmp/reports")

	view := safeView(t, m)

	if !strings.Contains(view, "Stored 1 reports") || !strings.Contains(view, "1 refused") {
		t.Errorf("store-result view lacks stored count and \"1 refused\" in the summary line:\n%s", view)
	}
	if strings.Contains(view, "$") {
		t.Errorf("store-result view shows cost:\n%s", view)
	}
}

func TestStoreFlow_Result_StoredReportIsNotShownAsRefused(t *testing.T) {
	o := newProcessOptions()
	o.Store = fakeStoreFunc(refusedStoreResult(), nil)
	m := advanceToStoreResult(t, NewModel(o), "/tmp/reports")

	view := safeView(t, m)

	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "/r/good.json") && strings.Contains(strings.ToLower(l), "refused") {
			t.Errorf("stored report shown as refused: %q", l)
		}
	}
}
