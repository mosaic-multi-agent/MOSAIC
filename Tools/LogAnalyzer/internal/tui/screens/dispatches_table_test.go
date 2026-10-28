package screens_test

// The Dispatches view as a table: every dispatch and the orchestrator on one
// row, long names that never break the columns, the share-of-run-cost bar, the
// status badge and the peak context in their roles. Cache columns are in
// dispatches_cache_test.go and sorting in dispatches_sort_test.go.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func dispatchesView(run domain.RunReport, width, height int) string {
	return screens.NewRunDetailScreen(run, width, height, paletteStyles()).View(width, height)
}

func TestDispatches_EveryDispatchAndTheOrchestratorHaveARow(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), 140, tallH)

	for _, id := range append(dispatchIDs(), "orchestrator") {
		rowFor(t, view, id)
	}
	assertContains(t, rowFor(t, view, "orchestrator"), "$4.00")
	assertContains(t, rowFor(t, view, "builder#1"), "$3.17")
	assertContains(t, rowFor(t, view, "reviewer#1"), "$1.39")
}

func TestDispatches_LongInstanceIdsAndTypesStayIntactWhenThereIsRoom(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), tableW, tallH)

	assertContains(t, view, longInstance, longType)
	row := rowFor(t, view, longInstance)
	assertContains(t, row, longType, "$2.28")
}

func TestDispatches_LongNamesNeverBreakTheColumnsAtNarrowWidths(t *testing.T) {
	colourOff(t)
	for _, width := range []int{120, 100, 80, 60, 40} {
		view := dispatchesView(dispatchRun(), width, tallH)

		assertWithinBounds(t, view, width, tallH)
		if width >= 60 {
			assertNameShown(t, view, longInstance, 4)
			assertNameShown(t, view, "builder#1", 4)
		}
		if width >= 80 {
			// Cost and status are the columns that are never dropped, and every dispatch is
			// one line: its name, status and cost never wrap onto separate lines.
			assertContains(t, rowFor(t, view, "$2.28"), "senior", "BLOCKED")
			assertContains(t, rowFor(t, view, "$3.17"), "builde", "SUCCESS")
		}
	}
}

func TestDispatches_RowsStayOneLineEachAtEveryWidth(t *testing.T) {
	colourOff(t)
	for _, width := range []int{200, 120, 80, 60} {
		view := dispatchesView(dispatchRun(), width, tallH)

		for _, id := range dispatchIDs() {
			if n := countOf(view, id[:6]); n == 0 {
				t.Errorf("width %d: dispatch %q has no row", width, id)
			}
		}
		// seven rows (orchestrator and six dispatches) plus the frame and header must fit
		// one line each: a wrapped row would push the help line off the terminal.
		lines := viewLines(view)
		assertContains(t, tailOf(lines), "? help")
	}
}

func TestDispatches_CostsAreAlignedWhateverTheNameLength(t *testing.T) {
	colourOff(t)
	for _, width := range []int{140, 100, 80} {
		view := dispatchesView(dispatchRun(), width, tallH)

		var cols []int
		for _, cost := range []string{"$3.17", "$2.28", "$1.39"} {
			cols = append(cols, displayColumn(rowFor(t, view, cost), cost))
		}
		for _, c := range cols {
			if c < 0 || c != cols[0] {
				t.Errorf("width %d: cost cells are not aligned (columns %v)\n%s", width, cols, view)
				break
			}
		}
	}
}

func TestDispatches_ShareBarGrowsWithTheDispatchsShareOfRunCost(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), 140, tallH)

	orch := barWeight(rowFor(t, view, "orchestrator"))
	d1, d2 := barWeight(rowFor(t, view, "builder#1")), barWeight(rowFor(t, view, longInstance))
	d3, d4 := barWeight(rowFor(t, view, "reviewer#1")), barWeight(rowFor(t, view, "builder#2"))
	if !(orch > d1 && d1 > d2 && d2 > d3 && d3 > d4 && d4 > 0) {
		t.Errorf("share bars must follow the cost shares ($4.00, $3.17, $2.28, $1.39, $0.54 of $10.80): got %d %d %d %d %d\n%s",
			orch, d1, d2, d3, d4, view)
	}
}

func TestDispatches_UnpricedDispatchHasNoShareBarAndNeverShowsZeroDollars(t *testing.T) {
	colourOff(t)

	row := rowFor(t, dispatchesView(dispatchRun(), 140, tallH), "builder#4")

	if w := barWeight(row); w != 0 {
		t.Errorf("an unpriced dispatch has no cost share, bar weight = %d: %q", w, row)
	}
	assertContains(t, row, screens.UnpricedMarker)
	assertNotContains(t, row, "$0.00")
}

func TestDispatches_StatusBadgeNamesTheOutcomeOfEveryDispatch(t *testing.T) {
	colourOff(t)
	view := dispatchesView(dispatchRun(), 140, tallH)

	for _, c := range []struct{ id, status string }{
		{"builder#1", "SUCCESS"}, {longInstance, "BLOCKED"}, {"reviewer#1", "PARTIALLY_DONE"},
		{"builder#2", "SUCCESS"}, {"builder#3", screens.NoStatusLabel}, {"builder#4", screens.OpenLabel},
	} {
		assertContains(t, rowFor(t, view, c.id), c.status)
	}
	assertNotContains(t, rowFor(t, view, "builder#1"), screens.OpenLabel, screens.NoStatusLabel)
}

func TestDispatches_StatusBadgeIsDrawnInTheStatusMappingRole(t *testing.T) {
	colourOn(t)
	run := dispatchRun()
	view := dispatchesView(run, tableW, tallH)

	// d1 and the orchestrator can hold the cursor, so only the other rows are checked.
	for _, c := range []struct {
		d      domain.Dispatch
		status string
	}{
		{dispatchD2(), "BLOCKED"}, {dispatchD3(), "PARTIALLY_DONE"}, {dispatchD4(), "SUCCESS"},
		{dispatchD5(), screens.NoStatusLabel}, {dispatchD6(), screens.OpenLabel},
	} {
		raw := rawLineWith(t, view, string(c.d.Invocation.Actor.Instance))
		assertDrawnInRole(t, raw, c.status, screens.StatusRole(c.d.Invocation))
	}
}

func TestDispatches_PeakContextShowsTheFigureAndIsDrawnInItsBandRole(t *testing.T) {
	colourOn(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)

	for _, c := range []struct {
		id     string
		tokens string
		band   domain.ContextBand
	}{
		{longInstance, "380,000", domain.BandDegraded},
		{"reviewer#1", "260,000", domain.BandAboveOptimal},
		{"builder#2", "150,000", domain.BandOptimal},
		{"builder#3", "40,000", domain.BandBelowOptimal},
	} {
		raw := rawLineWith(t, view, c.id)
		assertDrawnInRole(t, raw, c.tokens, screens.BandRole(c.band))
	}
}

func TestDispatches_AbsentPeakContextShowsTheMarkerNotZero(t *testing.T) {
	colourOff(t)
	absent := dispatchesView(dispatchRun(), 140, tallH)
	zero := dispatchesView(withDispatch(dispatchRun(), "builder#4", func(d *domain.Dispatch) {
		d.PeakContext = domain.ContextFigure{Tokens: domain.Tokens(0), Band: domain.BandBelowOptimal}
	}), 140, tallH)

	absentRow, zeroRow := rowFor(t, absent, "builder#4"), rowFor(t, zero, "builder#4")

	if countOf(absentRow, screens.AbsentMarker) <= countOf(zeroRow, screens.AbsentMarker) {
		t.Errorf("an absent peak must show the marker where a present zero shows 0: %q vs %q", absentRow, zeroRow)
	}
	assertNotContains(t, absentRow, "$0.00", "0.0%")
}

func TestDispatches_OrchestratorRowShowsItsOwnPeakContext(t *testing.T) {
	colourOff(t)

	row := rowFor(t, dispatchesView(dispatchRun(), 140, tallH), "orchestrator")

	assertContains(t, row, "210,000")
}

func TestDispatches_CostIsDrawnInTheMoneyRoleAndPartialOrUnpricedInWarning(t *testing.T) {
	colourOn(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)

	assertDrawnInRole(t, rawLineWith(t, view, longInstance), "$2.28", screens.MoneyRole(known(228), true))
	assertDrawnInRole(t, rawLineWith(t, view, "builder#3"), "$0.35 (partial)", screens.MoneyRole(known(35), false))
	assertDrawnInRole(t, rawLineWith(t, view, "builder#4"), screens.UnpricedMarker, screens.MoneyRole(domain.UnpricedMoney(), false))
}

func TestDispatches_WithColourOffEveryMarkerAndStatusRemainsAsText(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), 140, tallH)

	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences: %q", view)
	}
	assertContains(t, view, "$0.35 (partial)", screens.UnpricedMarker, screens.OpenLabel, screens.NoStatusLabel,
		"BLOCKED", "PARTIALLY_DONE", "SUCCESS")
}

func TestDispatches_PartialCostIsMarkedAndAnUnpricedOneIsNeverZero(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), 140, tallH)

	assertContains(t, rowFor(t, view, "builder#3"), "(partial)")
	assertNotContains(t, rowFor(t, view, "builder#1"), "(partial)")
	assertNotContains(t, strings.Join(strings.Fields(rowFor(t, view, "builder#4")), " "), "$0")
}

func TestDispatches_IncompletenessLabelsStayOnTheRowOfTheirDispatch(t *testing.T) {
	colourOff(t)
	run := withDispatch(dispatchRun(), "builder#1", func(d *domain.Dispatch) { d.Totals.AssumedTierCacheWrite = 4321 })
	run = withDispatch(run, "builder#4", func(d *domain.Dispatch) { d.MissingUsage = true })

	view := dispatchesView(run, tableW, tallH)

	affected := rowFor(t, view, "builder#1")
	assertContains(t, affected, screens.AssumedTierLabel, "4,321")
	assertContains(t, rowFor(t, view, "builder#4"), screens.MissingUsageLabel, screens.UnpricedMarker)
	assertNotContains(t, rowFor(t, view, "reviewer#1"), screens.AssumedTierLabel, screens.MissingUsageLabel)
	assertNotContains(t, rowFor(t, view, longInstance), screens.AssumedTierLabel, screens.MissingUsageLabel)
}

func TestDispatches_RunWithoutSubagentsStillListsTheOrchestrator(t *testing.T) {
	colourOff(t)

	view := dispatchesView(overviewRun(), 100, 20)

	assertContains(t, view, "orchestrator", "$4.00")
	assertWithinBounds(t, view, 100, 20)
}
