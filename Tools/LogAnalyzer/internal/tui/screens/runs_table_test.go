package screens_test

// The runs list as a table: one row per run with its cost, invocations,
// duration and markers; long run labels that never break the columns; colour
// roles for cost and markers; the page header with the run and finding counts.

import (
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestRuns_EveryRunHasARowWithCostAndDuration(t *testing.T) {
	colourOff(t)

	view := runsView(runsReport(), 140, tallH)

	for _, id := range []string{runIDOne, runIDTwo, runIDThree, runIDFour, runIDLong} {
		rowFor(t, view, id)
	}
	assertContains(t, rowFor(t, view, runIDOne), "$12.34", "1m30s")
	assertContains(t, rowFor(t, view, runIDTwo), "$5.67", "30s")
	assertContains(t, rowFor(t, view, runIDThree), "3m0s")
	assertContains(t, rowFor(t, view, runIDFour), "45s")
}

func TestRuns_EveryRunRowShowsItsOwnInvocationCount(t *testing.T) {
	colourOff(t)

	view := runsView(runsReport(), 140, tallH)

	assertFields(t, rowFor(t, view, runIDOne), "58")
	assertFields(t, rowFor(t, view, runIDTwo), "86")
	assertFields(t, rowFor(t, view, runIDThree), "31")
	assertFields(t, rowFor(t, view, runIDFour), "74")
	assertNoFields(t, rowFor(t, view, runIDOne), "86", "31", "74")
	assertNoFields(t, rowFor(t, view, runIDTwo), "58", "31", "74")
}

func TestRuns_ListsTheRunsMostExpensiveFirstWhateverTheReportOrder(t *testing.T) {
	colourOff(t)

	view := runsView(runsReport(), 140, tallH)

	assertAppearInOrder(t, view, []string{runIDOne, runIDFour, runIDTwo}, true)
}

func TestRuns_LongRunLabelsNeverBreakTheColumnsAtNarrowWidths(t *testing.T) {
	colourOff(t)
	for _, width := range []int{140, 120, 100, 80, 60, 40} {
		view := runsView(runsReport(), width, tallH)

		assertWithinBounds(t, view, width, tallH)
		if width >= 60 {
			assertNameShown(t, view, runIDLong, 8)
			for _, id := range []string{runIDOne, runIDTwo, runIDThree, runIDFour} {
				assertNameShown(t, view, id, 8)
			}
		}
		if width >= 80 {
			// One line per run: the label and the cost are never split over lines.
			assertContains(t, rowFor(t, view, "$12.34"), runIDOne[:12])
			assertContains(t, rowFor(t, view, "$5.67"), runIDTwo[:12])
		}
	}
}

func TestRuns_CostsAreAlignedWhateverTheLabelLength(t *testing.T) {
	colourOff(t)
	for _, width := range []int{140, 100, 80} {
		view := runsView(runsReport(), width, tallH)

		assertContains(t, rowFor(t, view, "$12.34"), runIDOne[:12])
		assertContains(t, rowFor(t, view, "$5.67"), runIDTwo[:12])
		long := displayColumn(rowFor(t, view, "$12.34"), "$12.34")
		short := displayColumn(rowFor(t, view, "$5.67"), "$5.67")
		// Cells are aligned when they start at one column (left) or end at one (right).
		if long < 0 || short < 0 || (long != short && long+len("$12.34") != short+len("$5.67")) {
			t.Errorf("width %d: cost cells are not aligned (columns %d and %d)\n%s", width, long, short, view)
		}
	}
}

func TestRuns_OnlyTheRunWithoutAnEndEventIsMarkedInProgress(t *testing.T) {
	colourOff(t)

	view := runsView(runsReport(), 140, tallH)

	assertContains(t, rowFor(t, view, runIDTwo), screens.ProvisionalLabel)
	for _, id := range []string{runIDOne, runIDThree, runIDFour, runIDLong} {
		assertNotContains(t, rowFor(t, view, id), screens.ProvisionalLabel)
	}
}

func TestRuns_AbsentUnpricedAndPartialCostsKeepTheirMarkers(t *testing.T) {
	colourOff(t)
	view := runsView(runsReport(), 140, tallH)

	unpriced, partial, absent := rowFor(t, view, runIDThree), rowFor(t, view, runIDFour), rowFor(t, view, runIDLong)

	assertContains(t, unpriced, screens.UnpricedMarker)
	assertContains(t, partial, "$7.89 (partial)")
	assertContains(t, absent, screens.AbsentMarker)
	assertNotContains(t, unpriced, "$0.00")
	assertNotContains(t, absent, "$0.00")
	assertNotContains(t, rowFor(t, view, runIDOne), "(partial)", screens.UnpricedMarker)
}

func TestRuns_ARunWithoutAxisShowsTheAbsentMarkerForItsDuration(t *testing.T) {
	colourOff(t)

	row := rowFor(t, runsView(runsReport(), 140, tallH), runIDLong)

	// No cost and no duration: both cells are the marker.
	if got := countOf(row, screens.AbsentMarker); got < 2 {
		t.Errorf("cost and duration of a run without data must both show %q: %q", screens.AbsentMarker, row)
	}
}

func TestRuns_CostAndMarkersAreDrawnInTheirRoles(t *testing.T) {
	colourOn(t)
	view := runsView(runsReport(), tableW, tallH)

	// The cheapest-listed rows are checked: the first row can hold the cursor.
	assertDrawnInRole(t, rawLineWith(t, view, runIDTwo), "$5.67", screens.MoneyRole(known(567), true))
	assertDrawnInRole(t, rawLineWith(t, view, runIDTwo), screens.ProvisionalLabel, tuicommon.RoleSignalWarning)
	assertDrawnInRole(t, rawLineWith(t, view, runIDFour), "$7.89 (partial)", screens.MoneyRole(known(789), false))
	assertDrawnInRole(t, rawLineWith(t, view, runIDThree), screens.UnpricedMarker, screens.MoneyRole(domain.UnpricedMoney(), false))
}

func TestRuns_WithColourOffTheScreenStaysUnderstandable(t *testing.T) {
	colourOff(t)

	view := runsView(runsReport(), 140, tallH)

	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences: %q", view)
	}
	assertContains(t, view, screens.ProvisionalLabel, "$7.89 (partial)", screens.UnpricedMarker, screens.AbsentMarker)
}

func TestRuns_PageHeaderShowsTheTitleAndTheCountOfNamedRuns(t *testing.T) {
	colourOff(t)
	report := runsReport()
	report.Unattributable = &domain.RunReport{Run: domain.UnattributableRun(), Totals: moneyTotals(50)}

	header := headOf(viewLines(runsView(report, 140, tallH)))

	assertContains(t, header, "Log Analyzer", "5 runs")
	assertNotContains(t, header, "6 runs", "finding")
}

func TestRuns_PageHeaderCountsASingleRunInTheSingular(t *testing.T) {
	colourOff(t)
	report := runsReport()
	report.Runs = report.Runs[:1]

	header := headOf(viewLines(runsView(report, 140, tallH)))

	assertContains(t, header, "1 run")
	assertNotContains(t, header, "1 runs")
}

func TestRuns_PageHeaderShowsFindingsInTheWarningOrBadRole(t *testing.T) {
	cases := []struct {
		name string
		kind domain.FindingKind
		role tuicommon.StyleRole
	}{
		{"findings without data loss", domain.FindingMissingTokenUsage, warning},
		{"findings with data loss", domain.FindingMalformedLine, bad},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			colourOn(t)
			report := runsReport()
			report.Quality = domain.NewQualitySummary([]domain.Finding{
				{Kind: c.kind, Path: "a", Line: 1}, {Kind: c.kind, Path: "b", Line: 2},
			})

			header := strings.SplitN(runsView(report, tableW, tallH), "\n", 2)[0]

			assertContains(t, header, styledBadge("2 findings", c.role))
		})
	}
}

func TestRuns_UnattributableBucketKeepsItsOwnSectionOutsideTheRunList(t *testing.T) {
	colourOff(t)
	report := runsReport()
	report.Unattributable = &domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{Money: completeMoney(50)},
	}

	view := runsView(report, 140, tallH)

	assertContains(t, view, screens.UnattributableLabel, "$0.50")
	if strings.Index(view, screens.UnattributableLabel) < strings.Index(view, runIDOne) {
		t.Errorf("the unattributable section must follow the run rows\n%s", view)
	}
}

func TestRuns_UnpricedModelsNoticeAndFindingsFooterStayVisible(t *testing.T) {
	colourOff(t)
	report := runsReport()
	report.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	report.Quality = domain.NewQualitySummary([]domain.Finding{{Kind: domain.FindingMalformedLine, Path: "a", Line: 1}})

	view := runsView(report, 140, tallH)

	assertContains(t, view, "no pricing", "finding")
}

func TestRuns_EmptyReportShowsThatThereAreNoRuns(t *testing.T) {
	colourOff(t)

	view := runsView(domain.Report{Currency: domain.Currency, Quality: domain.NewQualitySummary(nil)}, 100, 20)

	assertContains(t, strings.ToLower(view), "no named runs")
	assertWithinBounds(t, view, 100, 20)
}

func TestRuns_EnterOpensTheRunUnderTheCursorOfTheSortedList(t *testing.T) {
	s := screens.NewRunsScreen(runsReport(), 140, tallH, paletteStyles())

	press(s, "enter")
	first := s.SelectedRunID()
	s.Reset()
	press(s, "down", "enter")
	second := s.SelectedRunID()

	if first != runIDOne || second != runIDFour {
		t.Errorf("rows are cost-sorted, so enter opens %s then %s; got %s then %s", runIDOne, runIDFour, first, second)
	}
}
