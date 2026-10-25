package screens_test

// Tests for reasoning in the TUI token displays.
//
// Rules under test:
//   - One-line summaries read "output: <n> (reasoning: <r>)", with the absent
//     marker for <r> when no request reported reasoning.
//   - Per-category rows (overview "Token categories") add a reasoning row
//     directly under output, indented, with the token figure and no money.
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
	line := screens.FormatTotalsLine(domain.Totals{Tokens: usageWithReasoning(200, domain.Tokens(40))})

	assertContains(t, line, "output: 200 (reasoning: 40)")
}

func TestFormatTotalsLine_AbsentReasoning_ShowsAbsentMarker(t *testing.T) {
	line := screens.FormatTotalsLine(domain.Totals{Tokens: usageWithReasoning(200, domain.AbsentTokens())})

	assertContains(t, line, "output: 200 (reasoning: "+screens.AbsentMarker+")")
}

func TestFormatTotalsLine_PresentZeroReasoning_ShowsZeroNotMarker(t *testing.T) {
	line := screens.FormatTotalsLine(domain.Totals{Tokens: usageWithReasoning(200, domain.Tokens(0))})

	assertContains(t, line, "output: 200 (reasoning: 0)")
}

func TestFormatTotalsLine_GroupsReasoningDigits(t *testing.T) {
	line := screens.FormatTotalsLine(domain.Totals{Tokens: usageWithReasoning(2_000_000, domain.Tokens(1_234_567))})

	assertContains(t, line, "output: 2,000,000 (reasoning: 1,234,567)")
}

func TestFormatTotalsLine_PartlyReportedReasoning_ShowsPresentSumWithoutMarker(t *testing.T) {
	partial := domain.SumUsage(
		domain.TokenUsage{Output: domain.Tokens(100), Reasoning: domain.Tokens(25)},
		domain.TokenUsage{Output: domain.Tokens(50)},
	)

	line := screens.FormatTotalsLine(domain.Totals{Tokens: partial})

	// Other categories are unreported in this fixture and legitimately show the
	// marker, so only the reasoning part is checked for it.
	assertContains(t, line, "output: 150 (reasoning: 25)")
	assertNotContains(t, line, "(reasoning: "+screens.AbsentMarker)
}

func TestFormatTotalsLine_ReasoningAddsNoMoneyPart(t *testing.T) {
	money := domain.CategoryMoney{Total: known(1000), Complete: true}
	with := screens.FormatTotalsLine(domain.Totals{Tokens: usageWithReasoning(200, domain.Tokens(40)), Money: money})
	without := screens.FormatTotalsLine(domain.Totals{Tokens: usageWithReasoning(200, domain.AbsentTokens()), Money: money})

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

// tokenCategoryRows returns the lines of the overview "Token categories"
// section, up to the next blank line.
func tokenCategoryRows(t *testing.T, view string) []string {
	t.Helper()
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		if strings.Contains(l, "Token categories") {
			var rows []string
			for _, r := range lines[i+1:] {
				if strings.TrimSpace(r) == "" {
					break
				}
				rows = append(rows, r)
			}
			return rows
		}
	}
	t.Fatalf("no Token categories section in view:\n%s", view)
	return nil
}

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

func TestOverview_PresentReasoning_RowDirectlyUnderOutputIndentedWithoutMoney(t *testing.T) {
	rows := tokenCategoryRows(t, overviewView(runWithReasoning(domain.Tokens(40)), tallH))

	out := -1
	for i, r := range rows {
		if strings.HasPrefix(strings.TrimSpace(r), "output") {
			out = i
		}
	}
	if out < 0 || out+1 >= len(rows) {
		t.Fatalf("output row missing or nothing after it: %q", rows)
	}
	reasoning := rows[out+1]
	if !strings.HasPrefix(strings.TrimSpace(reasoning), "reasoning") || !strings.Contains(reasoning, "40") {
		t.Errorf("row after output = %q, want a reasoning row showing 40", reasoning)
	}
	if indentOf(reasoning) <= indentOf(rows[out]) {
		t.Errorf("reasoning row %q is not indented under output row %q", reasoning, rows[out])
	}
	assertNotContains(t, reasoning, "$", screens.UnpricedMarker, screens.AbsentMarker)
}

func TestOverview_AbsentReasoning_RowShowsAbsentMarkerWithoutMoney(t *testing.T) {
	view := overviewView(runWithReasoning(domain.AbsentTokens()), tallH)

	var reasoning string
	for _, r := range tokenCategoryRows(t, view) {
		if strings.HasPrefix(strings.TrimSpace(r), "reasoning") {
			reasoning = r
		}
	}
	if reasoning == "" {
		t.Fatalf("no reasoning row; view:\n%s", view)
	}
	assertContains(t, reasoning, screens.AbsentMarker)
	assertNotContains(t, reasoning, "$", screens.UnpricedMarker)
}

func TestOverview_PresentZeroReasoning_RowShowsZeroNotMarker(t *testing.T) {
	view := overviewView(runWithReasoning(domain.Tokens(0)), tallH)

	var reasoning string
	for _, r := range tokenCategoryRows(t, view) {
		if strings.HasPrefix(strings.TrimSpace(r), "reasoning") {
			reasoning = r
		}
	}
	if reasoning == "" {
		t.Fatalf("no reasoning row; view:\n%s", view)
	}
	assertContains(t, reasoning, "0")
	assertNotContains(t, reasoning, "$", screens.UnpricedMarker, screens.AbsentMarker)
}

func TestOverview_Reasoning_LeavesEveryOtherLineIdentical(t *testing.T) {
	without := overviewView(runWithReasoning(domain.AbsentTokens()), tallH)
	with := overviewView(runWithReasoning(domain.Tokens(40)), tallH)

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

func TestOverview_ReasoningRow_ComesFromTheReasoningCountNotOutput(t *testing.T) {
	run := runWithReasoning(domain.Tokens(4242))
	run.Overview.Totals.Tokens.Output = domain.Tokens(9191)

	view := overviewView(run, tallH)

	assertContains(t, lineWith(view, "reasoning"), "4,242")
	assertNotContains(t, lineWith(view, "reasoning"), "9,191")
}

func TestRunDetail_ShowsReasoningInRunAndActorSummaries(t *testing.T) {
	run := runWithReasoning(domain.Tokens(40))
	run.Orchestrator.Totals.Tokens.Reasoning = domain.Tokens(41)
	run.Agents[0].Totals.Tokens.Reasoning = domain.Tokens(42)

	view := screens.NewRunDetailScreen(run, wideW, tallH, plain()).View(wideW, tallH)

	assertContains(t, view, "output: 200 (reasoning: 40)", "output: 77 (reasoning: 41)", "output: 88 (reasoning: 42)")
}

func TestRunDetail_AbsentReasoning_ShowsMarkerInSummaries(t *testing.T) {
	view := screens.NewRunDetailScreen(runWithReasoning(domain.AbsentTokens()), wideW, tallH, plain()).View(wideW, tallH)

	assertContains(t, view, "output: 200 (reasoning: "+screens.AbsentMarker+")", "output: 77 (reasoning: "+screens.AbsentMarker+")")
}

func TestRuns_ShowsReasoningInAllRunsRunRowAndUnattributable(t *testing.T) {
	run := runWithReasoning(domain.Tokens(40))
	run.Totals.Tokens.Reasoning = domain.Tokens(41)
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

	view := screens.NewRunsScreen(report, wideW, tallH, plain()).View(wideW, tallH)

	assertContains(t, view, "(reasoning: 41)", "(reasoning: 42)", "(reasoning: 43)")
}
