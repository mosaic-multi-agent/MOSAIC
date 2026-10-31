package screens_test

// The By agent type view as a table: the figures of each type, the share bar,
// per-type peak context and cache figures, repeat spend and likely rework with
// their labels, incompleteness flags, the unattributed row and long type names.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestAgentTypesTable_EveryColumnShowsTheFiguresOfItsType(t *testing.T) {
	colourOff(t)
	view := typesFiguresView(typesFiguresRun(), tableW, tallH)

	assertContains(t, rowFor(t, view, "builder"), "$60.25", "$5.02", "$1.37", "$9.64",
		"111,111", "150,000", "75.0%", "23,456", "$0.88", "$4.44", "$1.33")
	assertFields(t, rowFor(t, view, "builder"), "12")
	assertContains(t, rowFor(t, view, "reviewer"), "$20.50", "$1.36", "$0.45", "$3.88",
		"262,626", "380,380", "12.5%", "65,432", "$5.43", "$2.52", "$1.78")
	assertFields(t, rowFor(t, view, "reviewer"), "15")
	assertContains(t, rowFor(t, view, screens.UnknownTypeLabel), "$7.77", "150,150")
	assertFields(t, rowFor(t, view, screens.UnknownTypeLabel), "21")
}

func TestAgentTypesTable_RepeatDispatchAndLikelyReworkCountsShowOnTheirOwnRow(t *testing.T) {
	colourOff(t)
	view := typesFiguresView(typesFiguresRun(), tableW, tallH)

	builder, reviewer := rowFor(t, view, "builder"), rowFor(t, view, "reviewer")

	assertFields(t, builder, "11", "10")
	assertFields(t, reviewer, "14", "13")
	assertNoFields(t, builder, "14", "13")
	assertNoFields(t, reviewer, "11", "10")
}

func TestAgentTypesTable_FiguresOfOneTypeNeverAppearOnAnotherRow(t *testing.T) {
	colourOff(t)
	view := typesFiguresView(typesFiguresRun(), tableW, tallH)

	assertNotContains(t, rowFor(t, view, "builder"), "65,432", "$5.43", "12.5%", "262,626", "$2.52")
	assertNotContains(t, rowFor(t, view, "reviewer"), "23,456", "$0.88", "75.0%", "111,111", "$4.44")
}

func TestAgentTypesTable_ShareBarGrowsWithTheTypesShareOfRunCost(t *testing.T) {
	colourOff(t)
	view := typesFiguresView(typesFiguresRun(), tableW, tallH)

	b := barWeight(rowFor(t, view, "builder"))
	r := barWeight(rowFor(t, view, "reviewer"))
	u := barWeight(rowFor(t, view, screens.UnknownTypeLabel))

	if !(b > r && r > u && u > 0) {
		t.Errorf("share bars must follow the shares of the $100.00 run ($60.25, $20.50, $7.77): got %d %d %d\n%s", b, r, u, view)
	}
}

func TestAgentTypesTable_RepeatSpendAndLikelyReworkAreLabelledWithTheirLimits(t *testing.T) {
	colourOff(t)

	view := strings.ToLower(typesFiguresView(typesFiguresRun(), tableW, tallH))

	assertContains(t, view, "repeat spend includes planned repeats",
		"likely rework is a heuristic; logs do not record the workflow step")
}

func TestAgentTypesTable_PeakContextIsDrawnInItsBandRole(t *testing.T) {
	colourOn(t)
	view := typesFiguresView(typesFiguresRun(), tableW, tallH)

	// The first row can hold the cursor, so the other two rows are checked.
	reviewer, unknown := rawLineWith(t, view, "reviewer"), rawLineWith(t, view, screens.UnknownTypeLabel)
	assertDrawnInRole(t, reviewer, "262,626", screens.BandRole(domain.BandOf(262_626)))
	assertDrawnInRole(t, reviewer, "380,380", screens.BandRole(domain.BandOf(380_380)))
	assertDrawnInRole(t, unknown, "150,150", screens.BandRole(domain.BandOf(150_150)))
}

func TestAgentTypesTable_AbsentContextAndCacheFiguresShowTheMarkerNotZero(t *testing.T) {
	colourOff(t)
	populated := typesFiguresView(typesFiguresRun(), tableW, tallH)
	run := typesFiguresRun()
	run.ByAgentType[1].PeakContext = domain.PeakContextStats{}
	run.ByAgentType[1].Cache = domain.CacheFigures{}
	absent := typesFiguresView(run, tableW, tallH)

	populatedRow, absentRow := rowFor(t, populated, "reviewer"), rowFor(t, absent, "reviewer")

	if got := countOf(absentRow, "—") - countOf(populatedRow, "—"); got < 3 {
		t.Errorf("absent peak and cache figures must show the marker (only %d more markers): %q vs %q", got, absentRow, populatedRow)
	}
	assertNotContains(t, absentRow, "0.0%", "$0.00")
}

func TestAgentTypesTable_IncompletenessFlagsStayOnTheRowOfTheirTypeAndAreNotRunOnText(t *testing.T) {
	colourOff(t)
	run := typesFiguresRun()
	g := &run.ByAgentType[0]
	g.Figures.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	g.Figures.Totals.AssumedTierCacheWrite = 4321
	g.Figures.MissingUsage = 1
	g.Figures.Open = 2

	view := typesFiguresView(run, tableW, tallH)

	builder := rowFor(t, view, "builder")
	assertContains(t, builder, screens.UnpricedMarker, screens.AssumedTierLabel(nil), screens.MissingUsageLabel, screens.OpenLabel)
	assertNotContains(t, builder, "; ")
	assertNotContains(t, rowFor(t, view, "reviewer"), screens.UnpricedMarker, screens.AssumedTierLabel(nil),
		screens.MissingUsageLabel, screens.OpenLabel)
}

func TestAgentTypesTable_LongTypeNamesNeverBreakTheColumns(t *testing.T) {
	colourOff(t)
	for _, width := range []int{tableW, 120, 100, 80, 60, 40} {
		view := typesFiguresView(dispatchRun(), width, tallH)

		assertWithinBounds(t, view, width, tallH)
		if width == tableW {
			assertContains(t, rowFor(t, view, longType), "$2.28")
		}
		if width >= 60 {
			assertNameShown(t, view, longType, 6)
		}
		if width >= 80 {
			assertContains(t, rowFor(t, view, "$2.28"), longType[:6])
			assertContains(t, rowFor(t, view, "$13.37"), "builder")
			assertContains(t, rowFor(t, view, "$1.39"), "review")
		}
	}
}

func TestAgentTypesTable_ValuesAreAlignedWhateverTheNameLength(t *testing.T) {
	colourOff(t)
	for _, width := range []int{tableW, 100, 80} {
		view := typesFiguresView(dispatchRun(), width, tallH)

		a := displayColumn(rowFor(t, view, "$13.37"), "$13.37")
		b := displayColumn(rowFor(t, view, "$2.28"), "$2.28")
		// $13.37 is one character longer: cells are aligned when they start at one column (left)
		// or end at one (right).
		if a < 0 || b < 0 || (a != b && a+len("$13.37") != b+len("$2.28")) {
			t.Errorf("width %d: totals are not aligned (columns %d and %d)\n%s", width, a, b, view)
		}
	}
}

func TestAgentTypesTable_UnattributedRowKeepsItsFiguresAndStaysAfterTheTypes(t *testing.T) {
	colourOff(t)
	view := typesFiguresView(withUnattributed(typesFiguresRun()), tableW, tallH)

	row := rowFor(t, strings.ToLower(view), "unattributed")

	assertContains(t, row, "$2.00")
	assertAppearInOrder(t, strings.ToLower(view), []string{"builder", "reviewer", "unattributed"}, true)
}

func TestAgentTypesTable_UnattributedRowIsReadableAtEightyColumns(t *testing.T) {
	colourOff(t)

	view := typesFiguresView(withUnattributed(typesFiguresRun()), 80, tallH)

	assertWithinBounds(t, view, 80, tallH)
	assertContains(t, strings.ToLower(view), "unattr")
	assertContains(t, view, "$2.00")
}

func TestAgentTypesTable_UnattributedRowStaysVisibleWhileScrolling(t *testing.T) {
	colourOff(t)
	for _, height := range []int{shortTableH, 12, 20} {
		s := screens.NewAgentTypesScreen(withUnattributed(manyTypesRun(30)), tableW, height, paletteStyles())

		top := s.View(tableW, height)
		press(s, repeat("down", 40)...)
		bottom := s.View(tableW, height)

		for _, view := range []string{top, bottom} {
			assertFits(t, view, height)
			assertContains(t, strings.ToLower(view), "unattributed")
			assertContains(t, view, "$2.00")
		}
		assertContains(t, bottom, "type-30")
		assertNotContains(t, bottom, "type-01")
	}
}

func TestAgentTypesTable_WithColourOffEveryLabelAndMarkerRemainsAsText(t *testing.T) {
	colourOff(t)
	run := typesFiguresRun()
	run.ByAgentType[1].PerInvocation.Complete = false

	view := typesFiguresView(run, tableW, tallH)

	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences: %q", view)
	}
	assertContains(t, view, "(partial)", screens.AbsentMarker)
	assertContains(t, strings.ToLower(view), "heuristic")
}

func TestAgentTypesTable_SortKeyCyclesThroughEveryOrderAndHeaderShowsIt(t *testing.T) {
	colourOff(t)
	s := screens.NewAgentTypesScreen(typesFiguresRun(), tableW, tallH, paletteStyles())

	assertContains(t, s.View(tableW, tallH), "sort: cost")
	seen := walkSorts(t, s, tableW, tallH, typesSortAscending(),
		screens.SortCost, screens.SortInvocations, screens.SortPeakContext, screens.SortName)

	if len(seen) == 0 || seen[0] != screens.SortCost {
		t.Errorf("the cycle must start on cost, got %v", seen)
	}
}

func TestAgentTypesTable_SortingNeverDropsATypeOrTheUnattributedRow(t *testing.T) {
	colourOff(t)
	s := screens.NewAgentTypesScreen(withUnattributed(typesFiguresRun()), tableW, tallH, paletteStyles())

	for i := 0; i < 5; i++ {
		view := strings.ToLower(s.View(tableW, tallH))
		assertContains(t, view, "builder", "reviewer", strings.ToLower(screens.UnknownTypeLabel), "unattributed")
		press(s, screens.KeySort)
	}
}

func TestAgentTypesTable_SortKeyIsListedInTheHelpOverlayOfTheListAndTheDrillDown(t *testing.T) {
	colourOff(t)
	for _, keys := range [][]string{nil, {"enter"}} {
		s := screens.NewAgentTypesScreen(typesFiguresRun(), 120, tallH, paletteStyles())
		press(s, keys...)

		overlay := strings.Join(overlayLines(openOverlay(s, 120, tallH)), "\n")

		if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+.*(?i:sort)`).MatchString(overlay) {
			t.Errorf("the help overlay must list the sort key %q (keys %v)\n%s", screens.KeySort, keys, overlay)
		}
	}
}

func TestAgentTypesTable_EnterDrillsIntoTheTypeUnderTheCursorOfTheSortedList(t *testing.T) {
	s := screens.NewAgentTypesScreen(typesFiguresRun(), tableW, tallH, paletteStyles())

	press(s, "down", "enter")

	if got, ok := s.DrilledType(); !ok || got != "reviewer" {
		t.Errorf("cost-sorted rows are builder, reviewer, unknown, so the second row is reviewer; got (%q, %v)", got, ok)
	}
}
