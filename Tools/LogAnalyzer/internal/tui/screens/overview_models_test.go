package screens_test

// The by-model table of the visual Overview: figures per model, the cost bar,
// sort cycling with the active sort in the header, and narrow widths.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const longModel = "an-extremely-long-model-name-for-the-overview-table"

func modelSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCost:   {screens.UnknownModelLabel, "model-b", "model-a"},
		screens.SortTokens: {"model-a", screens.UnknownModelLabel, "model-b"},
	}
}

func TestOverviewModels_EveryColumnShowsTheFiguresOfItsModel(t *testing.T) {
	colourOff(t)
	view := overviewView(analysesRun(), overviewTallH)

	a, b, unknown := rowFor(t, view, "model-a"), rowFor(t, view, "model-b"), rowFor(t, view, screens.UnknownModelLabel)

	assertContains(t, a, "$6.50")
	assertFields(t, a, "3")
	assertContains(t, b, "$2.75")
	assertFields(t, b, "2")
	assertContains(t, unknown, "$0.75")
	assertFields(t, unknown, "1")
	assertNotContains(t, a, "$2.75", "$0.75")
}

func TestOverviewModels_CostBarGrowsWithTheModelsCost(t *testing.T) {
	colourOff(t)
	view := overviewView(analysesRun(), overviewTallH)

	a, b, u := barWeight(rowFor(t, view, "model-a")), barWeight(rowFor(t, view, "model-b")),
		barWeight(rowFor(t, view, screens.UnknownModelLabel))

	if !(a > b && b > u && u > 0) {
		t.Errorf("cost bars must follow the model costs ($6.50, $2.75, $0.75): got %d %d %d\n%s", a, b, u, view)
	}
}

func TestOverviewModels_CostsAreAlignedWhateverTheNameLength(t *testing.T) {
	colourOff(t)
	for _, width := range []int{wideW, 100, 80} {
		view := screens.NewOverviewScreen(analysesRun(), width, overviewTallH, paletteStyles()).View(width, overviewTallH)

		a := displayColumn(rowFor(t, view, "model-a"), "$6.50")
		u := displayColumn(rowFor(t, view, screens.UnknownModelLabel), "$0.75")

		if a < 0 || a != u {
			t.Errorf("width %d: costs are not aligned (columns %d and %d)\n%s", width, a, u, view)
		}
	}
}

func TestOverviewModels_DefaultSortIsCostWithTheMostExpensiveFirst(t *testing.T) {
	colourOff(t)
	view := overviewView(analysesRun(), overviewTallH)

	key, desc := activeSort(t, view)

	if key != screens.SortCost || !desc {
		t.Errorf("default sort = %q desc=%v, want cost descending", key, desc)
	}
	assertAppearInOrder(t, view, []string{"model-a", "model-b", screens.UnknownModelLabel}, true)
}

func TestOverviewModels_SortKeyCyclesThroughEveryOrderAndHeaderShowsIt(t *testing.T) {
	colourOff(t)
	s := screens.NewOverviewScreen(analysesRun(), 140, overviewTallH, paletteStyles())

	assertContains(t, s.View(140, overviewTallH), "sort: cost")
	seen := walkSorts(t, s, 140, overviewTallH, modelSortAscending(), screens.SortCost, screens.SortTokens, screens.SortName)

	if len(seen) == 0 || seen[0] != screens.SortCost {
		t.Errorf("the cycle must start on cost, got %v", seen)
	}
}

func TestOverviewModels_SortingNeverDropsAModelRow(t *testing.T) {
	colourOff(t)
	s := screens.NewOverviewScreen(analysesRun(), 140, overviewTallH, paletteStyles())

	for i := 0; i < 6; i++ {
		view := s.View(140, overviewTallH)
		assertContains(t, view, "model-a", "model-b", screens.UnknownModelLabel)
		press(s, screens.KeySort)
	}
}

func TestOverviewModels_SortKeyIsListedInTheHelpOverlay(t *testing.T) {
	colourOff(t)
	s := screens.NewOverviewScreen(analysesRun(), 120, tallH, paletteStyles())

	overlay := strings.Join(overlayLines(openOverlay(s, 120, tallH)), "\n")

	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+.*(?i:sort)`).MatchString(overlay) {
		t.Errorf("the help overlay must list the sort key %q\n%s", screens.KeySort, overlay)
	}
}

func TestOverviewModels_UnpricedModelKeepsItsMarkerInTheTableRow(t *testing.T) {
	colourOff(t)
	run := analysesRun()
	run.Overview.ByModel[1] = domain.ModelFigures{
		Model: "model-b", Priced: false, Requests: 2,
		Totals: domain.Totals{Money: domain.CategoryMoney{Total: domain.UnpricedMoney()}},
	}

	row := rowFor(t, overviewView(run, overviewTallH), "model-b")

	assertContains(t, row, screens.UnpricedMarker)
	assertNotContains(t, row, "$0.00")
}

func TestOverviewModels_LongModelNamesNeverBreakTheTableOrTheBounds(t *testing.T) {
	colourOff(t)
	run := analysesRun()
	run.Overview.ByModel = append(run.Overview.ByModel,
		domain.ModelFigures{Model: longModel, Priced: true, Requests: 9, Totals: modelTotals(1337, 5000)})

	for _, width := range []int{wideW, 100, 80, 60, 40} {
		view := screens.NewOverviewScreen(run, width, 24, paletteStyles()).View(width, 24)
		assertWithinBounds(t, view, width, 24)

		all := scrollThrough(screens.NewOverviewScreen(run, width, 24, paletteStyles()), width, 24, 150)
		if width >= 60 {
			assertNameShown(t, all, longModel, 6)
		}
		if width >= 80 {
			assertContains(t, rowFor(t, all, "$13.37"), longModel[:6])
		}
	}
}
