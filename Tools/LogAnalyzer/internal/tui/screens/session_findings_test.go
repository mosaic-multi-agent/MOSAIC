package screens_test

// The findings list of the Session screen: measured facts of the actor, one line
// each, between the cost split and the context curve. Every line states a figure
// and no cause; the findings of another actor never show.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// causeWords are words that would name a cause the logs do not prove.
var causeWords = []string{"because", "due to", "caused", "probably", "likely", "reason", "result of", "so that", "therefore"}

// findingLines are the lines of the view that state a finding: lines opening with the
// words of a finding, a list bullet allowed.
func findingLines(view string) []string {
	var out []string
	for _, l := range viewLines(view) {
		t := strings.TrimLeft(strings.TrimSpace(l), "-•*·› ")
		for _, m := range []string{"costliest call #", "context rewrites: ", "compactions: ", "failed tool ", "peak context ", "cost "} {
			vs := strings.Contains(t, " vs ")
			if strings.HasPrefix(t, m) && ((m != "peak context " && m != "cost ") || vs) {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

func TestSessionFindings_CostliestCallsListCostAndShareOfTheAgentCostliestFirst(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	lines := sessLinesWith(view, "costliest call #")

	if len(lines) != 3 {
		t.Fatalf("%d costliest call lines, want 3\n%s", len(lines), view)
	}
	wants := [][]string{
		{money(50), "(50% of this agent)"},
		{money(44), "(44% of this agent)"},
		{money(33), "(<1% of this agent)"}, // one third of a percent
	}
	for i, l := range lines {
		assertContains(t, l, wants[i]...)
	}
}

func TestSessionFindings_RewritesShowCountAndExtraCost(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	assertContains(t, view, "context rewrites: 2, extra cost "+money(7))
}

func TestSessionFindings_CompactionsShowCountUnappliedAndCompactionCost(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	assertContains(t, view, "compactions: 3 (1 unapplied), compaction cost "+money(9))
}

func TestSessionFindings_FailedToolsShowNameAndErrorCountMostErrorsFirst(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	lines := sessLinesWith(view, "failed tool ")

	want := []string{"failed tool Bash: 4 errors", "failed tool Edit: 2 errors", "failed tool Read: 2 errors"}
	if len(lines) != len(want) {
		t.Fatalf("%d failed tool lines, want %d\n%s", len(lines), len(want), view)
	}
	for i, l := range lines {
		assertContains(t, l, want[i])
	}
}

func TestSessionFindings_PeakContextAndCostAreComparedWithTheTypeAverage(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	assertContains(t, view,
		"peak context "+screens.FormatTokens(domain.Tokens(380_000))+" vs builder average "+screens.FormatTokens(domain.Tokens(250_000))+" (3 agents)",
		"cost "+money(1000)+" vs builder average "+money(600)+" (3 agents)")
}

func TestSessionFindings_FollowTheOrderOfTheirKind(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	last := -1
	for _, m := range []string{
		"costliest call #", "context rewrites: ", "compactions: ", "failed tool Bash", "failed tool Edit", "failed tool Read",
		"peak context " + screens.FormatTokens(domain.Tokens(380_000)) + " vs", "cost " + money(1000) + " vs",
	} {
		i := sessLineIndex(view, m)
		if i < 0 {
			t.Fatalf("no line holds %q\n%s", m, view)
		}
		if i <= last {
			t.Errorf("the line with %q (%d) comes before an earlier kind (%d)\n%s", m, i, last, view)
		}
		last = i
	}
}

func TestSessionFindings_SitBetweenTheCostSplitAndTheCallTable(t *testing.T) {
	view := sessFindingsView(t, "builder#1")

	split := sessLineIndex(view, "cache write", money(400)) // the cost split line of the cache write category
	first := sessLineIndex(view, "costliest call #")
	row := sessLineIndex(view, ctxAt(sessFirstCall["builder#1"]).Format("15:04:05"), "model-b")

	if split < 0 || first < 0 || row < 0 {
		t.Fatalf("split %d, first finding %d, first call row %d: all three must be on screen\n%s", split, first, row, view)
	}
	if !(split < first && first < row) {
		t.Errorf("the findings (line %d) must sit after the cost split (%d) and before the call rows (%d)", first, split, row)
	}
}

func TestSessionFindings_StateMeasuredFactsWithoutACause(t *testing.T) {
	for _, id := range []string{"builder#1", "coder#1", "orchestrator"} {
		t.Run(id, func(t *testing.T) {
			lines := findingLines(sessFindingsView(t, id))

			if len(lines) == 0 {
				t.Fatal("no finding lines on screen")
			}
			for _, l := range lines {
				assertNotContains(t, strings.ToLower(l), causeWords...)
			}
		})
	}
}

func TestSessionFindings_AllFindingLinesOfTheFixtureAreRecognised(t *testing.T) {
	// Guards the wording test above against scanning nothing: 3 + rewrites + compactions + 3 + 2.
	if got := len(findingLines(sessFindingsView(t, "builder#1"))); got != 10 {
		t.Errorf("%d finding lines recognised for builder#1, want 10", got)
	}
}

func TestSessionFindings_EachActorShowsOnlyItsOwn(t *testing.T) {
	coder := sessFindingsView(t, "coder#1")
	orch := sessFindingsView(t, "orchestrator")

	assertContains(t, coder, "costliest call #", money(65), "(26% of this agent)", "peak context 300,000 vs coder average 250,000 (2 agents)")
	assertNotContains(t, coder, "context rewrites", "compactions: ", "failed tool", "builder average")
	assertContains(t, orch, "costliest call #", money(40), "(40% of this agent)")
	assertNotContains(t, orch, "failed tool", "context rewrites")
	if n := len(findingLines(coder)); n != 2 {
		t.Errorf("coder#1 shows %d finding lines, want its 2", n)
	}
	if n := len(findingLines(orch)); n != 1 {
		t.Errorf("the orchestrator shows %d finding lines, want its 1", n)
	}
}

func TestSessionFindings_ActorWithoutFindingsOrWithoutFiguresShowsNone(t *testing.T) {
	for _, id := range []string{"reviewer#1", "tester#1", "ghost#1"} {
		t.Run(id, func(t *testing.T) {
			view := sessFindingsView(t, id)

			if lines := findingLines(view); len(lines) != 0 {
				t.Errorf("finding lines %q, want none", lines)
			}
			assertWithinBounds(t, view, headerW, legendOverlayH)
		})
	}
}

func TestSessionFindings_RunWithoutAnyActorSessionsOpensNormally(t *testing.T) {
	// A run built without the actor session figures, as before they existed.
	view := stripStyling(sessOpenFrom(t, sessionRun(), "builder#1", screens.ScreenContext, headerW, legendOverlayH).View(headerW, legendOverlayH))

	assertContains(t, view, "builder#1", "BLOCKED")
	if lines := findingLines(view); len(lines) != 0 {
		t.Errorf("finding lines %q, want none without session figures", lines)
	}
}

func TestSessionFindings_LinesFitANarrowTerminal(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		view := sessFindingsViewOf(t, sessionFindingsRun(), "builder#1", w)

		assertWithinBounds(t, view, w, legendOverlayH)
		if n := len(findingLines(view)); n != 10 {
			t.Errorf("width %d shows %d finding lines, want all 10 (wrapped or cut to one line each)\n%s", w, n, view)
		}
	}
}
