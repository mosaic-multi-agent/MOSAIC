package screens_test

// The drill-down from a type to its dispatches uses the table style of the
// Dispatches view: the same columns, badge, bar, band roles and sort cycling.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// drillTable opens the drill-down of the type reached by the given keys on the
// dispatch fixture (types by cost: builder, the long type, reviewer).
func drillTable(width, height int, keys ...string) (*screens.AgentTypesScreen, string) {
	s := screens.NewAgentTypesScreen(dispatchRun(), width, height, paletteStyles())
	press(s, keys...)
	return s, s.View(width, height)
}

func TestAgentTypesDrillTable_ShowsTheDispatchesOfTheTypeWithTheDispatchesColumns(t *testing.T) {
	colourOff(t)

	s, view := drillTable(tableW, tallH, "enter")

	if got, ok := s.DrilledType(); !ok || got != "builder" {
		t.Fatalf("DrilledType() = (%q, %v), want (\"builder\", true)", got, ok)
	}
	assertContains(t, rowFor(t, view, "builder#1"), "$3.17", "SUCCESS", "120,000", "75.0%", "12,345", "$0.77", "$1.11", "4,567", "$0.29")
	assertContains(t, rowFor(t, view, "builder#2"), "$0.54", "SUCCESS", "150,000", "50.0%", "7,777")
	assertContains(t, rowFor(t, view, "builder#3"), "$0.35 (partial)", screens.NoStatusLabel, "40,000")
	assertContains(t, rowFor(t, view, "builder#4"), screens.UnpricedMarker, screens.OpenLabel)
	assertNotContains(t, view, "reviewer#1", longInstance)
	assertContains(t, view, "unread cache write")
}

func TestAgentTypesDrillTable_HasNoOrchestratorRowAndNoRunTotalsSummary(t *testing.T) {
	colourOff(t)

	_, view := drillTable(tableW, tallH, "enter")

	assertNotContains(t, view, "orchestrator")
	assertNotContains(t, view, screens.FormatTotalsLine(domain.CurrencyUSD, dispatchD1().Totals))
}

func TestAgentTypesDrillTable_ShareBarGrowsWithTheDispatchsShareOfRunCost(t *testing.T) {
	colourOff(t)

	_, view := drillTable(tableW, tallH, "enter")

	d1, d4, d5 := barWeight(rowFor(t, view, "builder#1")), barWeight(rowFor(t, view, "builder#2")), barWeight(rowFor(t, view, "builder#3"))
	if !(d1 > d4 && d4 > d5 && d5 > 0) {
		t.Errorf("share bars must follow the cost shares ($3.17, $0.54, $0.35 of $10.80): got %d %d %d\n%s", d1, d4, d5, view)
	}
	if w := barWeight(rowFor(t, view, "builder#4")); w != 0 {
		t.Errorf("an unpriced dispatch has no share bar, weight = %d", w)
	}
}

func TestAgentTypesDrillTable_StatusPeakAndCostAreDrawnInTheirRoles(t *testing.T) {
	colourOn(t)

	_, view := drillTable(tableW, tallH, "enter")

	// builder#1 sorts first and can hold the cursor; the other rows carry the roles.
	ok := rawLineWith(t, view, "builder#2")
	assertDrawnInRole(t, ok, "SUCCESS", screens.StatusRole(dispatchD4().Invocation))
	assertDrawnInRole(t, ok, "150,000", screens.BandRole(domain.BandOptimal))
	assertDrawnInRole(t, ok, "$0.54", screens.MoneyRole(known(54), true))
	assertDrawnInRole(t, rawLineWith(t, view, "builder#3"), screens.NoStatusLabel, screens.StatusRole(dispatchD5().Invocation))
	assertDrawnInRole(t, rawLineWith(t, view, "builder#4"), screens.OpenLabel, screens.StatusRole(dispatchD6().Invocation))
}

func TestAgentTypesDrillTable_LongNamesNeverBreakTheColumns(t *testing.T) {
	colourOff(t)
	for _, width := range []int{tableW, 120, 100, 80, 60, 40} {
		s, view := drillTable(width, tallH, "down", "enter") // the long type is second by cost

		if got, ok := s.DrilledType(); !ok || got != longType {
			t.Fatalf("DrilledType() = (%q, %v), want the long type", got, ok)
		}
		assertWithinBounds(t, view, width, tallH)
		if width >= 60 {
			assertNameShown(t, view, longInstance, 4)
		}
		if width >= 80 {
			assertContains(t, rowFor(t, view, "$2.28"), "senior", "BLOCKED")
		}
	}
}

// mixedLengthRun is the dispatch fixture with a second, short-named dispatch under
// the long type, so one drill-down holds a 26-character and a 7-character id.
func mixedLengthRun() domain.RunReport {
	run := dispatchRun()
	short := dispatchOf("audit#7", longType, "SUCCESS", 91)
	run.ByAgentType[1].Dispatches = append(run.ByAgentType[1].Dispatches, short)
	run.Agents = append(run.Agents, domain.ActorReport{Actor: short.Invocation.Actor, Totals: short.Totals})
	return run
}

func TestAgentTypesDrillTable_CostsAreAlignedWhateverTheInstanceLength(t *testing.T) {
	colourOff(t)
	for _, width := range []int{tableW, 100, 80} {
		s := screens.NewAgentTypesScreen(mixedLengthRun(), width, tallH, paletteStyles())
		press(s, "down", "enter") // the long type is second by cost
		view := s.View(width, tallH)

		a := displayColumn(rowFor(t, view, "$2.28"), "$2.28")
		b := displayColumn(rowFor(t, view, "$0.91"), "$0.91")
		if a < 0 || a != b {
			t.Errorf("width %d: cost cells of the long and the short instance id are not aligned (columns %d and %d)\n%s", width, a, b, view)
		}
	}
}

func TestAgentTypesDrillTable_SortKeyCyclesTheDispatchesAndHeaderShowsIt(t *testing.T) {
	colourOff(t)
	s, _ := drillTable(tableW, tallH, "enter")
	ascending := map[screens.SortKey][]string{
		screens.SortCost:        {"builder#3", "builder#2", "builder#1"},
		screens.SortTokens:      {"builder#1", "builder#2", "builder#3"},
		screens.SortDuration:    {"builder#1", "builder#2", "builder#3"},
		screens.SortPeakContext: {"builder#3", "builder#1", "builder#2"},
		screens.SortName:        {"builder#1", "builder#2", "builder#3"},
	}

	assertContains(t, s.View(tableW, tallH), "sort: cost")
	walkSorts(t, s, tableW, tallH, ascending, screens.SortCost, screens.SortName)

	if _, ok := s.DrilledType(); !ok {
		t.Error("cycling the sort must keep the drill-down open")
	}
}

func TestAgentTypesDrillTable_WithColourOffMarkersAndStatusesRemainAsText(t *testing.T) {
	colourOff(t)

	_, view := drillTable(tableW, tallH, "enter")

	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences: %q", view)
	}
	assertContains(t, view, "$0.35 (partial)", screens.UnpricedMarker, screens.OpenLabel, screens.NoStatusLabel, "SUCCESS")
}
