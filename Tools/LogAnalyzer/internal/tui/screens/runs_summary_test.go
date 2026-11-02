package screens_test

// The summary lines of the runs list: the all-runs line above the table and the
// unattributable section below it. Both stay text lines and are never table rows,
// so sorting leaves them where they are.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func summarisedRunsReport() domain.Report {
	report := runsReport()
	report.AllRuns = domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(1_357_913), Output: domain.Tokens(2_468)},
		Money:  completeMoney(2_690),
	}
	report.Unattributable = &domain.RunReport{
		Run: domain.UnattributableRun(),
		Totals: domain.Totals{
			Tokens: domain.TokenUsage{Input: domain.Tokens(9_753_100)},
			Money:  completeMoney(50),
		},
	}
	return report
}

func TestRuns_AllRunsSummaryLineIsTextAboveTheRunRows(t *testing.T) {
	colourOff(t)
	report := summarisedRunsReport()

	view := runsView(report, tableW, tallH)

	assertContains(t, view, "All runs", screens.FormatTotalsLine(domain.CurrencyUSD, report.AllRuns))
	summary := lineNumber(t, view, screens.FormatTotalsLine(domain.CurrencyUSD, report.AllRuns))
	for _, id := range []string{runIDOne, runIDTwo, runIDThree, runIDFour, runIDLong} {
		if lineNumber(t, view, id) < summary {
			t.Errorf("the all-runs line must come before the row of %s\n%s", id, view)
		}
	}
}

func TestRuns_UnattributableSummaryFollowsTheRunRowsWhateverTheSort(t *testing.T) {
	colourOff(t)
	report := summarisedRunsReport()
	s := screens.NewRunsScreen(report, tableW, tallH, paletteStyles())

	for i := 0; i < 5; i++ {
		view := s.View(tableW, tallH)

		assertContains(t, view, screens.UnattributableLabel, screens.FormatTotalsLine(domain.CurrencyUSD, report.Unattributable.Totals))
		section := lineNumber(t, view, screens.UnattributableLabel)
		for _, id := range []string{runIDOne, runIDTwo, runIDThree, runIDFour, runIDLong} {
			if lineNumber(t, view, id) > section {
				t.Errorf("sort step %d: the unattributable section must follow the row of %s\n%s", i, id, view)
			}
		}
		if n := strings.Count(stripStyling(view), screens.UnattributableLabel); n != 1 {
			t.Errorf("sort step %d: the unattributable label appears %d times, want once\n%s", i, n, view)
		}
		press(s, screens.KeySort)
	}
}
