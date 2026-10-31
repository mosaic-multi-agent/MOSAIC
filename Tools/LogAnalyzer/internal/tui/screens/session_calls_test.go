package screens_test

// The per-call table of the Session screen: the tools each call requested and the
// context growth of its singular calls, "not provided" where the logs carry no
// requested tools, and the sorts and marks the former drill-down had.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// sessCallRow is the per-call row of the call an actor made at the given second.
func sessCallRow(t *testing.T, id string, sec int) string {
	t.Helper()
	return ctxCallRow(t, sessOpen(t, id).View(tableW, tallH), sec)
}

func TestSessionCalls_RowShowsTheRequestedToolsSortedWithCountsAboveOne(t *testing.T) {
	row := sessCallRow(t, "builder#1", 10)

	assertContains(t, row, "glob, read_file x2")
	assertNotContains(t, row, "read_file x1", "glob x1", notProvided)
}

func TestSessionCalls_SingularCallShowsItsToolAndGrowth(t *testing.T) {
	row := sessCallRow(t, "builder#1", 70)

	assertContains(t, row, "edit", screens.FormatTokens(domain.Tokens(sessMeasuredGrowth)))
	assertNotContains(t, row, notProvided, "negative", "not measurable")
}

func TestSessionCalls_ProvidedEmptyToolsShowNoneAndNoGrowth(t *testing.T) {
	row := sessCallRow(t, "builder#1", 130)

	assertContains(t, row, "none")
	assertNotContains(t, row, notProvided, "negative", "not measurable")
}

func TestSessionCalls_NotProvidedToolsShowTheLabelInBothColumns(t *testing.T) {
	row := sessCallRow(t, "builder#1", 190)

	if n := strings.Count(row, notProvided); n != 2 {
		t.Errorf("a call without requested tools shows %q in the tools and in the growth column, got %d in %q", notProvided, n, row)
	}
}

func TestSessionCalls_NegativeAndNotMeasurableGrowthAreNamedNotShownAsFigures(t *testing.T) {
	negative := sessCallRow(t, "coder#1", 30)
	unmeasurable := sessCallRow(t, "coder#1", 90)

	assertContains(t, negative, "bash", "negative")
	assertNotContains(t, negative, "-500", "500 ")
	assertContains(t, unmeasurable, "bash", "not measurable")
	assertNotContains(t, unmeasurable, "negative", notProvided)
}

func TestSessionCalls_MeasuredGrowthIsShownInTheTokenFormatOfTheScreen(t *testing.T) {
	row := sessCallRow(t, "coder#1", 150)

	assertContains(t, row, "glob", screens.FormatTokens(domain.Tokens(sessCoderGrowth)))
}

func TestSessionCalls_CallThatRequestedSeveralToolsHasNoGrowth(t *testing.T) {
	row := sessCallRow(t, "coder#1", 210)

	assertContains(t, row, "glob, read_file")
	assertNotContains(t, row, notProvided, "negative", "not measurable")
}

func TestSessionCalls_OlderLogsWithoutRequestedToolsShowNotProvidedOnEveryRow(t *testing.T) {
	view := sessView(t, "reviewer#1")

	for _, sec := range []int{20, 80, 140} {
		row := ctxCallRow(t, view, sec)
		if n := strings.Count(row, notProvided); n != 2 {
			t.Errorf("the call at %ds shows %q %d times, want 2 (tools and growth) in %q", sec, notProvided, n, row)
		}
	}
	assertWithinBounds(t, view, tableW, tallH)
}

func TestSessionCalls_OrchestratorCallsShowTheirToolsToo(t *testing.T) {
	run := sessionRun()
	run.Context.Orchestrator.Calls[1].RequestedTools = sessOneTool("grep")
	run.Context.Orchestrator.Calls[1].Growth = domain.CallGrowth{State: domain.GrowthMeasured, Tool: "grep", Tokens: 4_321}

	row := ctxCallRow(t, stripStyling(sessOpenFrom(t, run, "orchestrator", screens.ScreenContext, tableW, tallH).View(tableW, tallH)), 60)

	assertContains(t, row, "grep", screens.FormatTokens(domain.Tokens(4_321)))
}

func TestSessionCalls_ADigestLoadedOlderRunOpensWithNotProvidedRows(t *testing.T) {
	colourOff(t)
	run := sampleRun(t)
	opened := 0
	for _, a := range append([]domain.ActorContext{run.Context.Orchestrator}, run.Context.Invocations...) {
		if len(a.Calls) == 0 {
			continue
		}
		opened++
		view := stripStyling(screens.NewSessionScreen(run, a.Actor, screens.ScreenContext, tableW, tallH, paletteStyles()).View(tableW, tallH))

		assertContains(t, view, a.Actor.Label(), a.Calls[0].At.Format("15:04:05"), notProvided)
		assertWithinBounds(t, view, tableW, tallH)
		if opened == 3 {
			break
		}
	}
	if opened == 0 {
		t.Fatal("the sample run has no actor with per-call records to open")
	}
}

func TestSessionCalls_SortByCostAndContextReorderTheRows(t *testing.T) {
	s := sessOpen(t, "builder#1")
	order := func() []string {
		var got []string
		for _, l := range viewLines(s.View(tableW, tallH)) {
			for _, sec := range []int{10, 70, 130, 190} {
				if strings.Contains(l, ctxAt(sec).Format("15:04:05")) {
					got = append(got, ctxAt(sec).Format("15:04:05"))
				}
			}
		}
		return got
	}
	byTime := ctxCallLabels(10, 70, 130, 190)
	if got := order(); strings.Join(got, ",") != strings.Join(byTime, ",") {
		t.Fatalf("call order = %v, want %v", got, byTime)
	}

	press(s, screens.KeySort) // cost, largest first: 50, 44, 33, 22 cents
	byCost := ctxCallLabels(10, 190, 130, 70)
	if got := order(); strings.Join(got, ",") != strings.Join(byCost, ",") {
		t.Errorf("cost order = %v, want %v", got, byCost)
	}

	press(s, screens.KeySort) // context, largest first: 380k, 360k, 260k, 30k
	byContext := ctxCallLabels(130, 190, 70, 10)
	if got := order(); strings.Join(got, ",") != strings.Join(byContext, ",") {
		t.Errorf("context order = %v, want %v", got, byContext)
	}

	press(s, screens.KeySort)
	if got := order(); strings.Join(got, ",") != strings.Join(byTime, ",") {
		t.Errorf("the sorts must cycle back to call order, got %v", got)
	}
}

func TestSessionCalls_CursorStartsOnTheFirstCallAndStopsAtTheLast(t *testing.T) {
	s := sessOpen(t, "builder#1")

	if first := selectedLine(s.View(tableW, tallH)); !strings.Contains(first, ctxAt(10).Format("15:04:05")) {
		t.Errorf("the cursor starts on the first call, got %q", first)
	}
	press(s, "down", "down")
	if third := selectedLine(s.View(tableW, tallH)); !strings.Contains(third, ctxAt(130).Format("15:04:05")) {
		t.Errorf("two moves down select the third call, got %q", third)
	}
	press(s, repeat("down", 20)...)
	if last := selectedLine(s.View(tableW, tallH)); !strings.Contains(last, ctxAt(190).Format("15:04:05")) {
		t.Errorf("the cursor stops on the last call, got %q", last)
	}
}
