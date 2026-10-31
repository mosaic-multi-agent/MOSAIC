package screens_test

// Rewrite figures in the TUI: the cache block of the Overview shows the run's and
// the orchestrator's rewrites beside the hit ratio and net benefit, and the
// dispatch, agent type and Context tables gain a rewrites count column.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

var rewriteWord = regexp.MustCompile(`(?i)rewrite`)

// rewriteFigures builds figures of the given counts (model changed, cold, partial)
// with known money in cents.
func rewriteFigures(counts [3]int, tokens, costCents, extraCents int64) domain.RewriteFigures {
	return domain.RewriteFigures{
		ModelChanged: counts[0], ColdIdle: counts[1], Partial: counts[2],
		Tokens: domain.Tokens(tokens), Cost: known(costCents), ExtraCost: known(extraCents),
		Complete: true,
	}
}

// rewriteOverviewRun has run rewrites 3 / 5 / 7 (123,456 tokens, $6.37, extra
// $5.91) of which the orchestrator's are 1 / 2 / 0 (45,678 tokens, $2.14, extra $1.83).
func rewriteOverviewRun() domain.RunReport {
	run := analysesRun()
	run.Cache.Rewrites = rewriteFigures([3]int{3, 5, 7}, 123_456, 637, 591)
	run.OrchestratorCache = run.Cache
	run.OrchestratorCache.Rewrites = rewriteFigures([3]int{1, 2, 0}, 45_678, 214, 183)
	return run
}

// rewriteLineWith is the rewrite line of the view that carries the needle.
func rewriteLineWith(t *testing.T, view, needle string) string {
	t.Helper()
	for _, l := range viewLines(view) {
		if rewriteWord.MatchString(l) && strings.Contains(l, needle) {
			return l
		}
	}
	t.Fatalf("no line about rewrites carries %q\n%s", needle, stripStyling(view))
	return ""
}

func TestOverviewCache_ShowsTheRunsRewriteFiguresBesideTheCacheFigures(t *testing.T) {
	colourOff(t)
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(rewriteOverviewRun(), overviewTallH)

			line := rewriteLineWith(t, view, "$6.37")

			assertContains(t, line, "$5.91", "123,456")
			assertMatches(t, line, `\b3\b`, `\b5\b`, `\b7\b`)
			cache := windowAround(view, netBenefitPattern, 4)
			assertContains(t, cache, "$6.37")
		})
	}
}

func TestOverviewCache_ShowsTheOrchestratorsRewriteFigures(t *testing.T) {
	colourOff(t)
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(rewriteOverviewRun(), overviewTallH)

			line := rewriteLineWith(t, view, "$2.14")

			assertContains(t, line, "$1.83", "45,678")
		})
	}
}

func TestOverviewCache_OrchestratorFiguresAreNotTakenFromTheRunFigures(t *testing.T) {
	colourOff(t)
	run := rewriteOverviewRun()
	run.OrchestratorCache.Rewrites = domain.RewriteFigures{}

	view := overviewView(run, overviewTallH)

	assertNotContains(t, stripStyling(view), "$2.14", "45,678")
	rewriteLineWith(t, view, "$6.37")
}

// rewriteCell is the cell under the header column "rewrites" on the row holding
// the needle. The header is the line naming both "rewrites" and the anchor column.
func rewriteCell(t *testing.T, view, rowNeedle, anchor string) string {
	t.Helper()
	if block, ok := parseHeader(view, anchor); ok {
		// Wrapped titles: the column is read from the rendered header block.
		col, found := block.col("rewrites")
		if !found {
			t.Fatalf("the table anchored at %q has no \"rewrites\" column\n%s", anchor, stripStyling(view))
		}
		cells := runesOf(rowFor(t, stripStyling(view), rowNeedle))
		if len(cells) < col.end {
			t.Fatalf("row of %s is shorter than the rewrites column: %q", rowNeedle, string(cells))
		}
		return strings.TrimSpace(string(cells[col.start:col.end]))
	}
	var header string
	for _, l := range viewLines(view) {
		low := strings.ToLower(l)
		if strings.Contains(low, "rewrites") && strings.Contains(low, anchor) && !strings.Contains(low, "sort:") {
			header = low
			break
		}
	}
	if header == "" {
		t.Fatalf("no table header names the columns %q and \"rewrites\"\n%s", anchor, stripStyling(view))
	}
	start := len([]rune(header[:strings.Index(header, "rewrites")]))
	row := []rune(rowFor(t, stripStyling(view), rowNeedle))
	if len(row) < start+len("rewrites") {
		t.Fatalf("row of %s is shorter than the rewrites column: %q", rowNeedle, string(row))
	}
	return strings.TrimSpace(string(row[start : start+len("rewrites")]))
}

func TestDispatches_RewritesColumnCountsEachDispatchsRewrites(t *testing.T) {
	colourOff(t)
	run := withDispatch(dispatchRun(), "builder#1", func(d *domain.Dispatch) {
		d.Cache.Rewrites = rewriteFigures([3]int{1, 2, 1}, 1_000, 10, 9)
	})
	run = withDispatch(run, "reviewer#1", func(d *domain.Dispatch) {
		d.Cache.Rewrites = rewriteFigures([3]int{0, 0, 2}, 1_000, 10, 9)
	})

	view := dispatchesView(run, tableW, tallH)

	for id, want := range map[string]string{"builder#1": "4", "reviewer#1": "2"} {
		if got := rewriteCell(t, view, id, "instance"); got != want {
			t.Errorf("%s rewrites = %q, want %q", id, got, want)
		}
	}
}

func TestDispatches_OrchestratorRowShowsItsOwnRewriteCount(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.OrchestratorCache.Rewrites = rewriteFigures([3]int{0, 6, 0}, 1_000, 10, 9)
	run.Cache.Rewrites = rewriteFigures([3]int{0, 9, 0}, 1_000, 10, 9)

	view := dispatchesView(run, tableW, tallH)

	if got := rewriteCell(t, view, "orchestrator", "instance"); got != "6" {
		t.Errorf("orchestrator rewrites = %q, want 6 (its own, not the run's 9)", got)
	}
}

func TestAgentTypes_RewritesColumnCountsEachTypesRewrites(t *testing.T) {
	colourOff(t)
	run := typesFiguresRun()
	run.ByAgentType[0].Cache.Rewrites = rewriteFigures([3]int{0, 5, 3}, 1_000, 10, 9)
	run.ByAgentType[1].Cache.Rewrites = rewriteFigures([3]int{2, 0, 0}, 1_000, 10, 9)

	view := typesFiguresView(run, tableW, tallH)

	for typ, want := range map[string]string{"builder": "8", "reviewer": "2"} {
		if got := rewriteCell(t, view, typ, "invocations"); got != want {
			t.Errorf("%s rewrites = %q, want %q", typ, got, want)
		}
	}
}

func TestContext_RewritesColumnCountsTheActorsRewrites(t *testing.T) {
	run := contextRun()
	run.Context.Invocations[1].Rewrites = []domain.ContextRewrite{
		{Call: 1, Kind: domain.RewriteColdIdle}, {Call: 3, Kind: domain.RewritePartial}, {Call: 4, Kind: domain.RewriteModelChanged},
	}
	run.Context.Orchestrator.Rewrites = []domain.ContextRewrite{{Call: 2, Kind: domain.RewriteColdIdle}}

	view := ctxView(t, run, tableW, tallH)

	ccAssertCell(t, view, "coder#1", "model-c", "rewrites", "3")
	ccAssertCell(t, view, "orchestrator", "model-o", "rewrites", "1")
	ccAssertCell(t, view, "tester#1", "model-t", "rewrites", "0")
}
