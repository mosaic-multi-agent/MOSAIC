package screens_test

// The three-line head of the runs list: run count, period and totals on the
// first line, the typical cost and duration of a run on the next two.

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const headSep = " - " // the separator in ASCII render mode

func cost(cents int64) string { return screens.FormatMoney(domain.CurrencyUSD, known(cents)) }

func dur(d time.Duration) string { return screens.FormatDuration(d, true) }

// headRun is a finished run costing cents and lasting d from start (UTC).
func headRun(id string, cents int64, start time.Time, d time.Duration) domain.RunReport {
	run := statsRun(id, usd(cents).Nanos(), d)
	run.Timeline.Axis = domain.TimeInterval{Start: start, End: start.Add(d)}
	return run
}

// headReport has five runs:
//   - cost line counts three (10, 20, 60 cents): the fourth run is in progress and
//     the fifth has an unpriced model;
//   - time line counts four (1m, 2m, 3m, 5m): everything but the run in progress.
//
// The first run starts at Jan 2 03:00 in a +10h zone, which is Jan 1 in UTC; the
// run in progress ends last, on Jan 9.
func headReport() domain.Report {
	zone := time.FixedZone("plus10", 10*3600)
	first := headRun("run-a", 10, time.Date(2026, 1, 2, 3, 0, 0, 0, zone), time.Minute)
	second := headRun("run-b", 20, time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC), 2*time.Minute)
	third := headRun("run-c", 60, time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), 3*time.Minute)
	inProgress := headRun("run-d", 5, time.Date(2026, 1, 9, 9, 0, 0, 0, time.UTC), 30*time.Second)
	inProgress.Provisional = true
	unpriced := headRun("run-e", 0, time.Date(2026, 1, 4, 9, 0, 0, 0, time.UTC), 5*time.Minute)
	unpriced.Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}
	unpriced.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	return domain.Report{
		Runs:     []domain.RunReport{third, inProgress, first, unpriced, second},
		Currency: domain.Currency,
		Quality:  domain.NewQualitySummary(nil),
		AllRuns: domain.Totals{
			Tokens: domain.TokenUsage{Input: domain.Tokens(1_357_913), CacheRead: domain.Tokens(500), Output: domain.Tokens(2_468)},
			Money:  completeMoney(9_500),
		},
	}
}

const headTokens = 1_357_913 + 500 + 2_468

// headLines are the styling-free lines of the view from the "All runs" line on.
func headLines(t *testing.T, view string) []string {
	t.Helper()
	lines := strings.Split(stripStyling(view), "\n")
	for i, l := range lines {
		if strings.Contains(l, "All runs") {
			return lines[i:]
		}
	}
	t.Fatalf("no \"All runs\" line\n%s", stripStyling(view))
	return nil
}

func assertInOrder(t *testing.T, line string, items ...string) {
	t.Helper()
	pos := 0
	for _, item := range items {
		i := strings.Index(line[pos:], item)
		if i < 0 {
			t.Errorf("line %q lacks %q after position %d", line, item, pos)
			return
		}
		pos += i + len(item)
	}
}

func TestRunsHead_ReplacesTheAllRunsTotalsWithThreeLines(t *testing.T) {
	asciiOn(t)
	report := headReport()

	lines := headLines(t, runsView(report, tableW, tallH))

	if !strings.Contains(lines[1], "Per run") || !strings.Contains(lines[1], "cost") {
		t.Errorf("second head line = %q, want the Per run cost line", lines[1])
	}
	if !strings.Contains(lines[2], "time") || strings.Contains(lines[2], "Per run") {
		t.Errorf("third head line = %q, want the time line with no label", lines[2])
	}
	if strings.Contains(strings.Join(lines[:3], "\n"), screens.FormatTotalsLine(domain.CurrencyUSD, report.AllRuns)) {
		t.Errorf("the head still shows the per-category totals line")
	}
}

func TestRunsHead_FirstLineHasCountPeriodCostAndOneTokenFigure(t *testing.T) {
	asciiOn(t)
	line := headLines(t, runsView(headReport(), tableW, tallH))[0]

	assertInOrder(t, line, "All runs", "5 runs (1 in progress)", "Jan 1"+headSep+"Jan 9",
		"total "+screens.FormatTotal(domain.CurrencyUSD, completeMoney(9_500)),
		screens.FormatTokens(domain.Tokens(headTokens))+" tokens")
	for _, perCategory := range []string{"input:", "Input:", "cache", "Cache", "reasoning", "Reasoning"} {
		if strings.Contains(line, perCategory) {
			t.Errorf("first line %q shows a per-category figure (%q)", line, perCategory)
		}
	}
}

func TestRunsHead_RunCountNamesNoInProgressRunsWhenThereAreNone(t *testing.T) {
	asciiOn(t)
	report := domain.Report{Runs: statsReport(10, 20).Runs, Quality: domain.NewQualitySummary(nil)}

	line := headLines(t, runsView(report, tableW, tallH))[0]

	assertInOrder(t, line, "2 runs", "total")
	if strings.Contains(line, "in progress") {
		t.Errorf("first line %q names runs in progress, want none", line)
	}
}

func TestRunsHead_ShowsEveryRunsInProgressCount(t *testing.T) {
	asciiOn(t)
	report := headReport()
	report.Runs[2].Provisional = true

	line := headLines(t, runsView(report, tableW, tallH))[0]

	assertContains(t, line, "5 runs (2 in progress)")
}

func TestRunsHead_CostLineShowsAverageAndMedianWithItsCoverage(t *testing.T) {
	asciiOn(t)
	line := headLines(t, runsView(headReport(), tableW, tallH))[1]

	assertInOrder(t, line, "Per run", "cost", "avg "+cost(30)+headSep+"median "+cost(20), "(over 3 of 5 runs)")
	if strings.Contains(line, "p90") {
		t.Errorf("cost line %q shows p90 with only 3 runs counted", line)
	}
}

func TestRunsHead_TimeLineShowsAverageAndMedianWithItsCoverage(t *testing.T) {
	asciiOn(t)
	line := headLines(t, runsView(headReport(), tableW, tallH))[2]

	assertInOrder(t, line, "time", "avg "+dur(165*time.Second)+headSep+"median "+dur(150*time.Second), "(over 4 of 5 runs)")
}

func TestRunsHead_CostLineShowsP90FromTenRunsOn(t *testing.T) {
	asciiOn(t)
	cents := make([]int64, 10)
	for i := range cents {
		cents[i] = int64(i+1) * 10
	}
	report := statsReport(cents...)
	report.Quality = domain.NewQualitySummary(nil)

	line := headLines(t, runsView(report, tableW, tallH))[1]

	assertInOrder(t, line, "avg "+cost(55), "median "+cost(55), "p90 "+cost(90))
}

func TestRunsHead_StatisticsStateNoCoverageWhenEveryRunCounts(t *testing.T) {
	asciiOn(t)
	report := statsReport(10, 20, 30)
	report.Quality = domain.NewQualitySummary(nil)

	lines := headLines(t, runsView(report, tableW, tallH))

	for _, l := range lines[1:3] {
		if strings.Contains(l, "over ") {
			t.Errorf("line %q states a coverage although every run counts", l)
		}
	}
	assertContains(t, lines[1], "avg "+cost(20))
}

func TestRunsHead_TwentyThreeRunFixtureStatesItsCoverage(t *testing.T) {
	asciiOn(t)
	report := twentyThreeRuns()
	report.Quality = domain.NewQualitySummary(nil)

	lines := headLines(t, runsView(report, tableW, tallH))

	assertContains(t, lines[0], "23 runs (1 in progress)")
	assertContains(t, lines[1], "(over 21 of 23 runs)")
	assertContains(t, lines[2], "(over 22 of 23 runs)")
}

func TestRunsHead_EmptyStatisticsSayWhyInsteadOfShowingFigures(t *testing.T) {
	asciiOn(t)
	one := headRun("only", 10, statsEpoch, time.Minute)
	one.Provisional = true
	two := headRun("second", 20, statsEpoch, time.Minute)
	two.Provisional = true
	report := domain.Report{Runs: []domain.RunReport{one, two}, Quality: domain.NewQualitySummary(nil)}

	lines := headLines(t, runsView(report, tableW, tallH))

	assertContains(t, lines[0], "2 runs (2 in progress)")
	assertInOrder(t, lines[1], "cost", "no priced runs", "(over 0 of 2 runs)")
	assertInOrder(t, lines[2], "time", "no timed runs", "(over 0 of 2 runs)")
	for _, l := range lines[1:3] {
		if strings.Contains(l, "avg") || strings.Contains(l, "median") {
			t.Errorf("line %q shows figures although no run counts", l)
		}
	}
}

func TestRunsHead_AScreenWithNoRunsKeepsTheSingleTotalsLine(t *testing.T) {
	colourOff(t)
	report := domain.Report{Quality: domain.NewQualitySummary(nil)}
	report.AllRuns.Tokens = domain.TokenUsage{Input: domain.Tokens(42)}

	view := runsView(report, tableW, tallH)

	assertContains(t, view, "All runs", screens.FormatTotalsLine(domain.CurrencyUSD, report.AllRuns), "(no named runs)")
	assertNotContains(t, view, "Per run")
}

func TestRunsHead_StatisticsFollowAReprice(t *testing.T) {
	asciiOn(t)
	before := headReport()
	after := headReport()
	for i := range after.Runs { // every run now costs the same and is fully priced
		after.Runs[i].Totals.Money = completeMoney(200)
	}
	s := screens.NewRunsScreen(before, tableW, tallH, paletteStyles())
	if line := headLines(t, s.View(tableW, tallH))[1]; !strings.Contains(line, "avg "+cost(30)) {
		t.Fatalf("before the reprice the cost line is %q, want avg %s", line, cost(30))
	}

	s.SetReport(after)
	line := headLines(t, s.View(tableW, tallH))[1]

	assertContains(t, line, "avg "+cost(200)+headSep, "(over 4 of 5 runs)")
	assertNotContains(t, line, cost(30))
}

func TestRunsHead_LinesClipToANarrowTerminalKeepingCountAndTotalCostFirst(t *testing.T) {
	asciiOn(t)
	const width = 60
	report := headReport()

	lines := headLines(t, runsView(report, width, tallH))

	for i, l := range lines[:3] {
		if n := utf8.RuneCountInString(l); n > width {
			t.Errorf("head line %d is %d columns wide on a %d column terminal: %q", i, n, width, l)
		}
	}
	assertInOrder(t, lines[0], "All runs", "5 runs", "total "+cost(9_500))
}

func TestRunsHead_TakenOffTheTableBodyBudget(t *testing.T) {
	asciiOn(t)
	report := headReport()
	for _, h := range []int{16, 18, 22} {
		t.Run(fmt.Sprintf("height %d", h), func(t *testing.T) {
			view := runsView(report, tableW, h)

			assertWithinBounds(t, view, tableW, h)
		})
	}
}

func TestRunsHead_LeavesTheUnattributableLineBelowTheTable(t *testing.T) {
	asciiOn(t)
	report := headReport()
	report.Unattributable = &domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{Tokens: domain.TokenUsage{Input: domain.Tokens(9_753_100)}, Money: completeMoney(50)},
	}

	view := runsView(report, tableW, tallH)

	assertContains(t, view, screens.UnattributableLabel, screens.FormatTotalsLine(domain.CurrencyUSD, report.Unattributable.Totals))
	if lineNumber(t, view, screens.UnattributableLabel) < lineNumber(t, view, "run-a") {
		t.Errorf("the unattributable line must follow the table rows\n%s", view)
	}
	if lineNumber(t, view, "9,753,100") != lineNumber(t, view, screens.UnattributableLabel) {
		t.Errorf("the unattributable figures must stay on its own line\n%s", view)
	}
	if strings.Contains(headLines(t, view)[0], "9,753,100") {
		t.Errorf("the unattributable tokens leaked into the all-runs line")
	}
}
