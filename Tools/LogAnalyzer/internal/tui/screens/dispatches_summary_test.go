package screens_test

// The text lines around the Dispatches table (run totals and notices), the
// orchestrator row (always first, with its own cells) and the flags cell.

import (
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// lineNumber is the index of the first line of the view that contains needle.
func lineNumber(t *testing.T, view, needle string) int {
	t.Helper()
	for i, l := range strings.Split(stripStyling(view), "\n") {
		if strings.Contains(l, needle) {
			return i
		}
	}
	t.Fatalf("no line contains %q\n%s", needle, stripStyling(view))
	return -1
}

func TestDispatches_RunTotalsLineStaysAsTextAboveTheTable(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.Totals.Tokens = domain.TokenUsage{Input: domain.Tokens(4_242), Output: domain.Tokens(1_717)}

	view := dispatchesView(run, tableW, tallH)

	assertContains(t, view, "Run totals", screens.FormatTotalsLine(run.Totals))
	totals := lineNumber(t, view, screens.FormatTotalsLine(run.Totals))
	if lineNumber(t, view, "Run totals") > totals || totals > lineNumber(t, view, "orchestrator") {
		t.Errorf("the run totals heading and line must come before the table rows\n%s", view)
	}
}

func TestDispatches_TableRowsCarryNoTotalsSummaryLine(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), tableW, tallH)

	assertNotContains(t, view, screens.FormatTotalsLine(dispatchD1().Totals), screens.FormatTotalsLine(dispatchD2().Totals))
	if n := countOf(view, "input:"); n != 1 {
		t.Errorf("only the run totals line shows token categories as text, got %d lines\n%s", n, view)
	}
}

func TestDispatches_NoticeLinesStayAsTextAfterTheRunTotals(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	run.Totals.AssumedTierCacheWrite = 7_777

	view := dispatchesView(run, tableW, tallH)

	assertContains(t, view, "Unpriced models: model-x", screens.AssumedTierNotice(7_777))
	totals := lineNumber(t, view, screens.FormatTotalsLine(run.Totals))
	if lineNumber(t, view, "Unpriced models: model-x") < totals || lineNumber(t, view, screens.AssumedTierNotice(7_777)) < totals {
		t.Errorf("the notices follow the run totals line\n%s", view)
	}
}

func TestDispatches_NoNoticeLinesWhenNothingIsUnpricedOrAssumed(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), tableW, tallH)

	assertNotContains(t, view, "Unpriced models:", "Cache writes priced at")
}

func TestDispatches_OrchestratorRowHasNoTypeNoStatusAndAnAbsentDuration(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), tableW, tallH)
	row := rowFor(t, view, "orchestrator")

	assertNotContains(t, row, "SUCCESS", "BLOCKED", "PARTIALLY_DONE", screens.OpenLabel, screens.NoStatusLabel,
		"builder", "reviewer", longType)
	assertMarkerUnderDurationHeader(t, view, row)
}

func TestDispatches_OrchestratorRowStaysFirstUnderEverySort(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), tableW, tallH, paletteStyles())

	for i := 0; i < 6; i++ {
		view := s.View(tableW, tallH)

		orch := lineNumber(t, view, "orchestrator")
		for _, id := range dispatchIDs() {
			if lineNumber(t, view, id) < orch {
				t.Errorf("sort step %d: the orchestrator row must come before %q\n%s", i, id, view)
			}
		}
		press(s, screens.KeySort)
	}
}

func TestDispatches_OrchestratorRowShowsItsOwnCostTokensAndEffort(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.Orchestrator.Totals.Tokens = domain.TokenUsage{
		Input:         domain.Tokens(3_000_000),
		CacheRead:     domain.Tokens(4_000_000),
		CacheCreation: domain.Tokens(1_000_000),
		Output:        domain.Tokens(642_000),
		Reasoning:     domain.Tokens(300_000), // part of output, never added
	}
	run.Orchestrator.Effort = domain.EffortLevels{{Level: "high", Records: 3}}

	row := rowFor(t, dispatchesView(run, tableW, tallH), "orchestrator")

	assertContains(t, row, "$4.00", "effort: high")
	assertFields(t, row, "8,642,000")
	assertNoFields(t, row, "3,000,000", "8,942,000")
}

func TestDispatches_OrchestratorFlagsNameUnpricedModelsThenTheAssumedTier(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.Orchestrator.UnpricedModels = []domain.PricingKey{{Model: "model-y"}}
	run.Orchestrator.Totals.AssumedTierCacheWrite = 555

	view := dispatchesView(run, tableW, tallH)

	row := rowFor(t, view, "orchestrator")
	assertContains(t, row, "unpriced models", screens.FormatAssumedTier(555))
	if strings.Index(row, "unpriced models") > strings.Index(row, screens.FormatAssumedTier(555)) {
		t.Errorf("unpriced models come before the assumed tier label: %q", row)
	}
	assertNotContains(t, rowFor(t, dispatchesView(dispatchRun(), tableW, tallH), "orchestrator"), "unpriced models", screens.AssumedTierLabel)
}

func TestDispatches_FlagsAreJoinedInTheirFixedOrder(t *testing.T) {
	colourOff(t)
	run := withDispatch(dispatchRun(), "builder#4", func(d *domain.Dispatch) {
		d.MissingUsage = true
		d.Totals.AssumedTierCacheWrite = 4_321
	})

	row := rowFor(t, dispatchesView(run, tableW, tallH), "builder#4")

	want := screens.MissingUsageLabel + "; unpriced models; " + screens.FormatAssumedTier(4_321)
	assertContains(t, row, want)
	assertNotContains(t, rowFor(t, dispatchesView(run, tableW, tallH), "builder#1"), "unpriced models", screens.MissingUsageLabel)
}

func TestDispatches_FlagsAreDrawnInTheWarningRole(t *testing.T) {
	colourOn(t)
	run := withDispatch(dispatchRun(), "builder#4", func(d *domain.Dispatch) { d.MissingUsage = true })

	raw := rawLineWith(t, dispatchesView(run, tableW, tallH), "builder#4")

	assertDrawnInRole(t, raw, screens.MissingUsageLabel+"; unpriced models", tuicommon.RoleSignalWarning)
}
