package screens_test

// The across-runs view as two tables: the runs with a cost bar each, and the
// agent types with total, average per run and a bar; every figure of the view
// stays available, markers stay as text, roles colour the money and the
// provisional marker, the page header counts the runs, and the view sorts and
// offers the help overlay.

import (
	"regexp"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func newAcross(report domain.Report, width, height int) *screens.AcrossRunsScreen {
	return screens.NewAcrossRunsScreen(report, width, height, paletteStyles())
}

func TestAcrossRunsTable_EveryRunRowShowsItsOwnSubagentCount(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	a, b := rowFor(t, view, acrossRunA), rowFor(t, view, acrossRunB)

	assertFields(t, a, "3")
	assertFields(t, b, "2")
	assertNoFields(t, b, "3")
	assertFields(t, rowFor(t, view, acrossRunC), "2")
}

func TestAcrossRunsTable_CostBarGrowsWithTheCostOfTheRun(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	a, b, c := barWeight(rowFor(t, view, acrossRunA)), barWeight(rowFor(t, view, acrossRunB)), barWeight(rowFor(t, view, acrossRunC))

	if !(a > b && b > c && c > 0) {
		t.Errorf("cost bars must follow the costs $10.00, $6.00 and $3.21: got %d %d %d\n%s", a, b, c, view)
	}
}

func TestAcrossRunsTable_AgentTypeBarGrowsWithTheCostOfTheType(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	coder, tester, unknown := barWeight(rowFor(t, view, "coder")), barWeight(rowFor(t, view, "tester")),
		barWeight(rowFor(t, view, screens.UnknownTypeLabel))

	if !(coder > tester && tester > unknown && coder > 0) {
		t.Errorf("type bars must follow the costs $16.00, $2.40 and $0.77: got %d %d %d\n%s", coder, tester, unknown, view)
	}
}

// Only meaningful alongside CostBarGrows: the no-bar check is vacuous while no run draws a bar.
func TestAcrossRunsTable_UnpricedRunKeepsItsMarkerAndDrawsNoBar(t *testing.T) {
	report := acrossReport()
	report.AcrossRuns.Runs[1].Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney(), Complete: false}

	row := rowFor(t, acrossView(t, report, tableW, tallH), acrossRunB)

	assertContains(t, row, screens.UnpricedMarker)
	assertNotContains(t, row, "$0.00")
	if w := barWeight(row); w != 0 {
		t.Errorf("a run without a known cost must draw no bar, got weight %d: %q", w, row)
	}
}

func TestAcrossRunsTable_EveryAgentTypeFigureOfTodayStaysOnItsRow(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	coder := withoutThousands(rowFor(t, view, "coder"))
	assertFields(t, coder, "47", "3")
	assertContains(t, coder, coderPerRunInvocations, coderPerRunTokens, "$16.00", "$5.33")
	tester := rowFor(t, view, "tester")
	assertFields(t, tester, "2")
	assertFields(t, tester, "1.0")
	assertContains(t, tester, "$2.40", "$1.20")
	unknown := rowFor(t, view, screens.UnknownTypeLabel)
	assertFields(t, unknown, "1")
	assertContains(t, unknown, "$0.77")
}

func TestAcrossRunsTable_UndefinedPerRunTokensShowTheAbsentMarker(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	// The tester and unknown types carry no per-run token ratio.
	assertContains(t, rowFor(t, view, "tester"), screens.AbsentMarker)
	assertContains(t, rowFor(t, view, screens.UnknownTypeLabel), screens.AbsentMarker)
	assertContains(t, withoutThousands(rowFor(t, view, "coder")), coderPerRunTokens)
}

func TestAcrossRunsTable_ProvisionalRunsAreMarkedOnTheirRows(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	assertContains(t, rowFor(t, view, acrossRunC), screens.ProvisionalLabel)
	assertNotContains(t, rowFor(t, view, acrossRunA), screens.ProvisionalLabel)
	assertNotContains(t, rowFor(t, view, acrossRunB), screens.ProvisionalLabel)
	assertContains(t, rowFor(t, view, "coder"), screens.ProvisionalLabel)
	assertContains(t, rowFor(t, view, "tester"), screens.ProvisionalLabel)
	assertNotContains(t, rowFor(t, view, screens.UnknownTypeLabel), screens.ProvisionalLabel)
}

func TestAcrossRunsTable_IncompletePerRunAverageOfACompleteTotalIsMarkedPartial(t *testing.T) {
	report := acrossReport()
	report.AcrossRuns.ByAgentType[1].PerRunMoney.Complete = false

	view := acrossView(t, report, tableW, tallH)

	assertContains(t, rowFor(t, view, "tester"), "$1.20 (partial)")
	assertNotContains(t, rowFor(t, view, screens.UnknownTypeLabel), "(partial)")
}

func TestAcrossRunsTable_LongRunLabelsAndTypeNamesNeverBreakTheColumns(t *testing.T) {
	report := acrossReport()
	report.AcrossRuns.ByAgentType[0].AgentType = "a-very-long-agent-type-name-for-testing"
	for _, width := range []int{tableW, 120, 100, 80, 60, 40} {
		view := acrossView(t, report, width, tallH)

		assertWithinBounds(t, view, width, tallH)
		if width >= 60 {
			assertNameShown(t, view, "a-very-long-agent-type-name-for-testing", 6)
			assertNameShown(t, view, acrossRunA, 8)
		}
		if width >= 80 {
			// One line per row: the name and the total are never split over lines.
			assertContains(t, rowFor(t, view, "$10.00"), acrossRunA[:12])
			assertContains(t, rowFor(t, view, "$16.00"), "a-very")
		}
	}
}

func TestAcrossRunsTable_CostsAreAlignedWhateverTheLabelLength(t *testing.T) {
	report := acrossReport()
	report.AcrossRuns.Runs[1].Run = domain.NamedRun("run-b")
	for _, width := range []int{tableW, 100, 80} {
		view := acrossView(t, report, width, tallH)

		long := displayColumn(rowFor(t, view, acrossRunA[:12]), "$10.00")
		short := displayColumn(rowFor(t, view, "run-b"), "$6.00")
		// $10.00 is one character longer: cells are aligned when they start at one column
		// (left) or end at one (right).
		if long < 0 || short < 0 || (long != short && long+len("$10.00") != short+len("$6.00")) {
			t.Errorf("width %d: cost cells are not aligned (columns %d and %d)\n%s", width, long, short, view)
		}
	}
}

func TestAcrossRunsTable_MoneyAndProvisionalMarkerAreDrawnInTheirRoles(t *testing.T) {
	colourOn(t)
	view := newAcross(acrossReport(), tableW, tallH).View(tableW, tallH)

	// The first row of each table can hold the cursor, so the other rows are checked.
	assertDrawnInRole(t, rawLineWith(t, view, acrossRunB), "$6.00", screens.MoneyRole(known(600), true))
	assertDrawnInRole(t, rawLineWith(t, view, acrossRunC), "$3.21 (partial)", screens.MoneyRole(known(321), false))
	assertDrawnInRole(t, rawLineWith(t, view, acrossRunC), screens.ProvisionalLabel, tuicommon.RoleSignalWarning)
	assertDrawnInRole(t, rawLineWith(t, view, "tester"), "$2.40", screens.MoneyRole(known(240), true))
	assertDrawnInRole(t, rawLineWith(t, view, screens.UnknownTypeLabel), "$0.77", screens.MoneyRole(known(77), true))
}

func TestAcrossRunsTable_WithColourOffEveryLabelAndMarkerRemainsAsText(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences: %q", view)
	}
	assertContains(t, view, "(partial)", screens.ProvisionalLabel, screens.AssumedTierLabel, screens.OpenLabel,
		screens.MissingUsageLabel, "sort: cost")
}

func TestAcrossRunsTable_PageHeaderShowsTheTitleAndTheCountOfRuns(t *testing.T) {
	view := acrossView(t, acrossReport(), tableW, tallH)

	header := headOf(viewLines(view))

	assertContains(t, header, "Across Runs", "3 runs")
	assertNotContains(t, header, "finding")
}

func TestAcrossRunsTable_PageHeaderCountsASingleRunInTheSingular(t *testing.T) {
	report := acrossReport()
	report.AcrossRuns.Runs = report.AcrossRuns.Runs[:1]

	header := headOf(viewLines(acrossView(t, report, tableW, tallH)))

	assertContains(t, header, "1 run")
	assertNotContains(t, header, "1 runs")
}

func TestAcrossRunsTable_ReadableAtEightyColumnsOnOneLinePerRow(t *testing.T) {
	view := acrossView(t, acrossReport(), 80, tallH)

	assertWithinBounds(t, view, 80, tallH)
	assertContains(t, rowFor(t, view, "$10.00"), acrossRunA[:12])
	assertContains(t, rowFor(t, view, "$16.00"), "coder")
	assertContains(t, rowFor(t, view, "$2.40"), "tester")
}

func TestAcrossRunsTable_NothingIsDrawnPastTheTerminalAtAnySize(t *testing.T) {
	colourOff(t)
	s := newAcross(acrossReport(), 100, tallH)

	for height := 0; height <= 25; height++ {
		for _, width := range []int{100, 80, 60, 30} {
			assertWithinBounds(t, s.View(width, height), width, height)
		}
	}
}

func acrossRunsSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCost: {acrossRunC, acrossRunB, acrossRunA},
		screens.SortName: {acrossRunA, acrossRunB, acrossRunC},
	}
}

func acrossTypesSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCost: {screens.UnknownTypeLabel, "tester", "coder"},
		screens.SortName: {screens.UnknownTypeLabel, "coder", "tester"},
	}
}

func TestAcrossRunsTable_DefaultSortIsCostDescendingAndTheHeaderSaysSo(t *testing.T) {
	colourOff(t)

	key, desc := activeSort(t, newAcross(acrossReport(), tableW, tallH).View(tableW, tallH))

	if key != screens.SortCost || !desc {
		t.Errorf("default sort = %q desc=%v, want cost descending", key, desc)
	}
}

func TestAcrossRunsTable_SortKeyOrdersTheRunsAndTheTypesAndWrapsAround(t *testing.T) {
	colourOff(t)
	for name, order := range map[string]map[screens.SortKey][]string{
		"runs": acrossRunsSortAscending(), "types": acrossTypesSortAscending(),
	} {
		s := newAcross(acrossReport(), tableW, tallH)

		seen := walkSorts(t, s, tableW, tallH, order, screens.SortCost, screens.SortName)

		if len(seen) == 0 || seen[0] != screens.SortCost {
			t.Errorf("%s: the cycle must start on cost, got %v", name, seen)
		}
	}
}

func TestAcrossRunsTable_SortingNeverDropsARunOrAType(t *testing.T) {
	colourOff(t)
	s := newAcross(acrossReport(), tableW, tallH)

	for i := 0; i < 4; i++ {
		view := s.View(tableW, tallH)
		assertContains(t, view, acrossRunA, acrossRunB, acrossRunC, "coder", "tester", screens.UnknownTypeLabel)
		press(s, screens.KeySort)
	}
}

func overlayOf(s screens.Screen, width, height int) string {
	return strings.Join(overlayLines(openOverlay(s, width, height)), "\n")
}

func TestAcrossRunsHelpOverlay_ReplacesTheBodyAndListsSortNavigationAndBackButNoViews(t *testing.T) {
	colourOff(t)
	s := newAcross(acrossReport(), 120, 40)
	before := s.View(120, 40)

	overlay := openOverlay(s, 120, 40)

	if overlay == before {
		t.Fatal("? must change the view")
	}
	assertContains(t, headOf(viewLines(overlay)), "Across Runs")
	assertNotContains(t, overlay, acrossRunA)
	assertWithinBounds(t, overlay, 120, 40)
	text := overlayOf(newAcross(acrossReport(), 120, 60), 120, 60)
	assertContains(t, text, "This screen", "Navigation")
	assertNotContains(t, text, "Views")
	for _, k := range []string{screens.KeySort, "esc", screens.KeyHelp} {
		if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(k) + `\s+\S`).MatchString(text) {
			t.Errorf("overlay must list key %q on its own entry\n%s", k, text)
		}
	}
	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+.*(?i:sort)`).MatchString(text) {
		t.Errorf("overlay must say the sort key sorts\n%s", text)
	}
}

func TestAcrossRunsHelpOverlay_QuestionMarkClosesItAndEscLeavesOnlyAfterwards(t *testing.T) {
	colourOff(t)
	s := newAcross(acrossReport(), 120, 40)
	before := s.View(120, 40)

	openOverlay(s, 120, 40)
	press(s, screens.KeyHelp)
	if after := s.View(120, 40); after != before {
		t.Errorf("? must close the overlay and restore the view\n%s\nvs\n%s", after, before)
	}

	openOverlay(s, 120, 40)
	press(s, "esc")
	if s.Back() {
		t.Fatal("the first esc must close the overlay without leaving the view")
	}
	press(s, "esc")
	if !s.Back() {
		t.Error("esc after closing the overlay must leave the view")
	}
}

func TestAcrossRunsHelpOverlay_IgnoresEveryOtherKeyWhileOpen(t *testing.T) {
	colourOff(t)
	s := newAcross(acrossReport(), 120, 40)
	before := s.View(120, 40)

	openOverlay(s, 120, 40)
	press(s, "q", screens.KeySort)
	if s.Back() {
		t.Error("q must be ignored while the overlay is open")
	}
	press(s, screens.KeyHelp)

	if after := s.View(120, 40); after != before {
		t.Errorf("keys pressed while the overlay was open must change nothing\n%s\nvs\n%s", after, before)
	}
}
