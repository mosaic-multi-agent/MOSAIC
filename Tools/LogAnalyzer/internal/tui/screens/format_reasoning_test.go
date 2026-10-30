package screens_test

// Tests for reasoning in the TUI token displays.
//
// Rules under test:
//   - One-line summaries read "output: <n> (reasoning: <r>)", with the absent
//     marker for <r> when no request reported reasoning.
//   - The figures-only overview shows the reasoning figure with output, with the
//     token figure and no money.
//   - Reasoning never changes or adds a money figure.
//   - A partly reported figure shows the present sum with no extra marker.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func usageWithReasoning(output int64, reasoning domain.TokenCount) domain.TokenUsage {
	return domain.TokenUsage{
		Input:     domain.Tokens(1000),
		CacheRead: domain.Tokens(3000),
		Output:    domain.Tokens(output),
		Reasoning: reasoning,
	}
}

func TestFormatTotalsLine_PresentReasoning_ShownInsideOutputPart(t *testing.T) {
	line := screens.FormatTotalsLine(domain.CurrencyUSD, domain.Totals{Tokens: usageWithReasoning(200, domain.Tokens(40))})

	assertContains(t, line, "output: 200 (reasoning: 40)")
}

func TestFormatTotalsLine_AbsentReasoning_ShowsAbsentMarker(t *testing.T) {
	line := screens.FormatTotalsLine(domain.CurrencyUSD, domain.Totals{Tokens: usageWithReasoning(200, domain.AbsentTokens())})

	assertContains(t, line, "output: 200 (reasoning: "+screens.AbsentMarker+")")
}

func TestFormatTotalsLine_PresentZeroReasoning_ShowsZeroNotMarker(t *testing.T) {
	line := screens.FormatTotalsLine(domain.CurrencyUSD, domain.Totals{Tokens: usageWithReasoning(200, domain.Tokens(0))})

	assertContains(t, line, "output: 200 (reasoning: 0)")
}

func TestFormatTotalsLine_GroupsReasoningDigits(t *testing.T) {
	line := screens.FormatTotalsLine(domain.CurrencyUSD, domain.Totals{Tokens: usageWithReasoning(2_000_000, domain.Tokens(1_234_567))})

	assertContains(t, line, "output: 2,000,000 (reasoning: 1,234,567)")
}

func TestFormatTotalsLine_PartlyReportedReasoning_ShowsPresentSumWithoutMarker(t *testing.T) {
	partial := domain.SumUsage(
		domain.TokenUsage{Output: domain.Tokens(100), Reasoning: domain.Tokens(25)},
		domain.TokenUsage{Output: domain.Tokens(50)},
	)

	line := screens.FormatTotalsLine(domain.CurrencyUSD, domain.Totals{Tokens: partial})

	// Other categories are unreported in this fixture and legitimately show the
	// marker, so only the reasoning part is checked for it.
	assertContains(t, line, "output: 150 (reasoning: 25)")
	assertNotContains(t, line, "(reasoning: "+screens.AbsentMarker)
}

func TestFormatTotalsLine_ReasoningAddsNoMoneyPart(t *testing.T) {
	money := domain.CategoryMoney{Total: known(1000), Complete: true}
	with := screens.FormatTotalsLine(domain.CurrencyUSD, domain.Totals{Tokens: usageWithReasoning(200, domain.Tokens(40)), Money: money})
	without := screens.FormatTotalsLine(domain.CurrencyUSD, domain.Totals{Tokens: usageWithReasoning(200, domain.AbsentTokens()), Money: money})

	assertContains(t, with, "(reasoning: 40)")
	if strings.Count(with, "$") != strings.Count(without, "$") || !strings.HasSuffix(with, "$10.00") {
		t.Errorf("money part changed with reasoning:\nwith:    %q\nwithout: %q", with, without)
	}
}

func runWithReasoning(reasoning domain.TokenCount) domain.RunReport {
	run := overviewRun()
	run.Totals.Tokens.Reasoning = reasoning
	run.Overview.Totals.Tokens.Reasoning = reasoning
	run.Orchestrator.Totals.Tokens = usageWithReasoning(77, reasoning)
	run.Agents = []domain.ActorReport{{
		Actor:  domain.AgentInstance("coder#1"),
		Totals: domain.Totals{Tokens: usageWithReasoning(88, reasoning)},
	}}
	return run
}

// reasoningLineFigure is the text of the figures-only Overview from the word
// "reasoning" to the end of its line, which holds the reasoning figure.
func reasoningLineFigure(t *testing.T, view string) string {
	t.Helper()
	text := textAfterWord(view, "reasoning")
	if text == "" {
		t.Fatalf("no line shows reasoning; view:\n%s", view)
	}
	return text
}

func TestOverview_PresentReasoning_FigureSitsWithOutputAndCarriesNoMoney(t *testing.T) {
	view := overviewFiguresView(runWithReasoning(domain.Tokens(40)), tallH)

	figure := reasoningLineFigure(t, view)

	assertFields(t, figure, "40")
	assertNotContains(t, figure, "$", screens.UnpricedMarker, screens.AbsentMarker)
	at := patternLine(view, `(?i)reasoning`)
	lines := strings.Split(stripStyling(view), "\n")
	near := lines[at]
	if at > 0 {
		near = lines[at-1] + "\n" + near
	}
	if !strings.Contains(strings.ToLower(near), strings.ToLower(domain.CategoryOutput.Label())) {
		t.Errorf("reasoning is part of output and must sit on or directly under the output figure\n%s", near)
	}
}

func TestOverview_AbsentReasoning_FigureShowsAbsentMarkerWithoutMoney(t *testing.T) {
	figure := reasoningLineFigure(t, overviewFiguresView(runWithReasoning(domain.AbsentTokens()), tallH))

	assertContains(t, figure, screens.AbsentMarker)
	assertNotContains(t, figure, "$", screens.UnpricedMarker)
}

func TestOverview_PresentZeroReasoning_FigureShowsZeroNotMarker(t *testing.T) {
	figure := reasoningLineFigure(t, overviewFiguresView(runWithReasoning(domain.Tokens(0)), tallH))

	assertFields(t, figure, "0")
	assertNotContains(t, figure, "$", screens.UnpricedMarker, screens.AbsentMarker)
}

func TestOverview_Reasoning_LeavesEveryOtherLineIdentical(t *testing.T) {
	without := overviewFiguresView(runWithReasoning(domain.AbsentTokens()), tallH)
	with := overviewFiguresView(runWithReasoning(domain.Tokens(40)), tallH)

	drop := func(view string) string {
		var kept []string
		for _, l := range strings.Split(view, "\n") {
			if !strings.Contains(l, "reasoning") {
				kept = append(kept, l)
			}
		}
		return strings.Join(kept, "\n")
	}
	assertContains(t, with, "reasoning")
	if drop(with) != drop(without) {
		t.Errorf("reasoning data changed other overview lines (money or categories):\nwith:\n%s\nwithout:\n%s", with, without)
	}
}

func TestOverview_ReasoningFigure_ComesFromTheReasoningCountNotOutput(t *testing.T) {
	run := runWithReasoning(domain.Tokens(4242))
	run.Overview.Totals.Tokens.Output = domain.Tokens(9191)

	figure := reasoningLineFigure(t, overviewFiguresView(run, tallH))

	assertContains(t, figure, "4,242")
	assertNotContains(t, figure, "9,191")
}

func TestOverview_VisualNamesReasoningWithOutput(t *testing.T) {
	view := overviewView(runWithReasoning(domain.Tokens(40)), tallH)

	assertContains(t, strings.ToLower(view), "reasoning")
}

// The run totals summary of the Dispatches view and the all-runs and
// unattributable summaries of the runs list stay one-line summaries that show the
// reasoning part inside the output part. The table rows carry no token summary.

func TestRunDetail_ShowsReasoningInTheRunTotalsSummary(t *testing.T) {
	run := runWithReasoning(domain.Tokens(40))

	view := screens.NewRunDetailScreen(run, tableW, tallH, plain()).View(tableW, tallH)

	assertContains(t, view, "output: 200 (reasoning: 40)")
}

func TestRunDetail_AbsentReasoning_ShowsMarkerInTheRunTotalsSummary(t *testing.T) {
	view := screens.NewRunDetailScreen(runWithReasoning(domain.AbsentTokens()), tableW, tallH, plain()).View(tableW, tallH)

	assertContains(t, view, "output: 200 (reasoning: "+screens.AbsentMarker+")")
}

func TestRuns_ShowsReasoningInAllRunsAndUnattributableSummaries(t *testing.T) {
	run := runWithReasoning(domain.Tokens(40))
	unattributable := runWithReasoning(domain.Tokens(0))
	unattributable.Totals.Tokens.Reasoning = domain.Tokens(43)
	allRuns := run.Totals
	allRuns.Tokens.Reasoning = domain.Tokens(42)
	report := domain.Report{
		Currency:       domain.Currency,
		AllRuns:        allRuns,
		Runs:           []domain.RunReport{run},
		Unattributable: &unattributable,
	}

	view := screens.NewRunsScreen(report, tableW, tallH, plain()).View(tableW, tallH)

	assertContains(t, view, "(reasoning: 42)", "(reasoning: 43)")
}
