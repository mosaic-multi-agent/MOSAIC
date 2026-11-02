package screens_test

// Sorting of the Dispatches view: cost is the default, the sort key cycles
// through the offered orders and wraps, the header states the active order and
// the rows follow the direction the header states.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// dispatchSortAscending lists the dispatches that have a figure for each sort
// key, from the lowest to the highest value (the unpriced, open dispatch has no
// cost, peak or duration and is left out).
func dispatchSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCost:        {"builder#3", "builder#2", "reviewer#1", longInstance, "builder#1"},
		screens.SortTokens:      {"builder#1", "builder#2", "builder#3", longInstance, "reviewer#1"},
		screens.SortDuration:    {"builder#1", "builder#2", "reviewer#1", "builder#3", longInstance},
		screens.SortPeakContext: {"builder#3", "builder#1", "builder#2", "reviewer#1", longInstance},
		screens.SortName:        {"builder#1", "builder#2", "builder#3", "reviewer#1", longInstance},
	}
}

func TestDispatches_DefaultSortIsCostWithTheMostExpensiveFirst(t *testing.T) {
	colourOff(t)

	view := dispatchesView(dispatchRun(), tableW, tallH)

	key, desc := activeSort(t, view)
	if key != screens.SortCost || !desc {
		t.Errorf("default sort = %q desc=%v, want cost descending", key, desc)
	}
	assertAppearInOrder(t, view, []string{"builder#1", longInstance, "reviewer#1", "builder#2", "builder#3"}, true)
}

func TestDispatches_HeaderShowsTheActiveSortLabel(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), 140, tallH, paletteStyles())

	before := s.View(140, tallH)
	assertContains(t, before, "sort: cost")
	press(s, screens.KeySort)
	after := s.View(140, tallH)

	if key, _ := activeSort(t, after); key == screens.SortCost {
		t.Errorf("the sort key must change the order\n%s", after)
	}
}

func TestDispatches_SortKeyCyclesThroughEveryOrderAndWrapsAround(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), tableW, tallH, paletteStyles())

	seen := walkSorts(t, s, tableW, tallH, dispatchSortAscending(),
		screens.SortCost, screens.SortTokens, screens.SortDuration, screens.SortPeakContext, screens.SortName)

	if len(seen) == 0 || seen[0] != screens.SortCost {
		t.Errorf("the cycle must start on cost, got %v", seen)
	}
}

func TestDispatches_SortingNeverDropsARowAndKeepsTheOrchestrator(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), tableW, tallH, paletteStyles())

	for i := 0; i < 6; i++ {
		view := s.View(tableW, tallH)

		for _, id := range append(dispatchIDs(), "orchestrator") {
			rowFor(t, view, id)
		}
		press(s, screens.KeySort)
	}
}

func TestDispatches_SortKeyIsListedInTheHelpOverlay(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), 120, tallH, paletteStyles())

	overlay := strings.Join(overlayLines(openOverlay(s, 120, tallH)), "\n")

	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+.*(?i:sort)`).MatchString(overlay) {
		t.Errorf("the help overlay must list the sort key %q\n%s", screens.KeySort, overlay)
	}
}
