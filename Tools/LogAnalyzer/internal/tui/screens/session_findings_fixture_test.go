package screens_test

// Fixture and helpers of the findings and tools of the Session screen. The run is
// the standard Session run with the figures of its actors set directly, so every
// figure on the screen can only come from the actor it is shown for:
//
//	actor         findings                                                      tools
//	orchestrator  costliest call                                                Agent
//	builder#1     3 costliest calls, rewrites, compactions, 3 failed tools,     Bash, Edit, Glob (growth only),
//	              peak context and cost against the builder average             (start missing), Read
//	coder#1       costliest call, peak context against the coder average        Bash (no growth sample)
//	reviewer#1    none                                                          none
//	tester#1, ghost#1: the report holds no session figures for them (older run)

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// durations builds the duration figures of a tool from its paired calls.
func durations(count int, total, min, max time.Duration) domain.DurationStats {
	d := domain.DurationStats{Count: count, Total: total, Min: min, Max: max}
	if count > 0 {
		d.Average = total / time.Duration(count)
	}
	return d
}

// builderTools are the tool figures of builder#1: 12 calls in all.
func builderTools() []domain.ToolFigures {
	return []domain.ToolFigures{
		{Tool: "Bash", Calls: 5, Errors: 4, Unpaired: 1, Durations: durations(4, 8*time.Second, time.Second, 3*time.Second), Growth: leftOutOnly(2, 1)},
		{Tool: "Edit", Calls: 3, Errors: 2, Durations: durations(3, 9*time.Second, 2*time.Second, 4*time.Second), Growth: growthFigures(2, 1234, 777, 1691, 0, 0)},
		{Tool: "Glob", Growth: growthFigures(1, 4321, 4321, 4321, 0, 0)},
		{Tool: "Read", Calls: 2, Errors: 2, Durations: durations(2, 3*time.Second, time.Second, 2*time.Second)},
		{Tool: "", Calls: 2, Unpaired: 2},
	}
}

// builderFindings are every kind of finding, the failed tools tied on two errors.
// The shares are pinned per line and are not meant to sum up: the third costliest
// call has a share of one third of a percent (1/300) to pin the "<1%" wording.
func builderFindings() []domain.ActorFinding {
	return []domain.ActorFinding{
		{Kind: domain.ActorFindingCostliestCall, Call: 0, Cost: known(50), Share: domain.Ratio{Numerator: 50, Denominator: 100}},
		{Kind: domain.ActorFindingCostliestCall, Call: 3, Cost: known(44), Share: domain.Ratio{Numerator: 44, Denominator: 100}},
		{Kind: domain.ActorFindingCostliestCall, Call: 2, Cost: known(33), Share: domain.Ratio{Numerator: 1, Denominator: 300}},
		{Kind: domain.ActorFindingRewrites, Count: 2, Cost: known(7)},
		{Kind: domain.ActorFindingCompactions, Count: 3, Unapplied: 1, Cost: known(9)},
		{Kind: domain.ActorFindingFailedTool, Tool: "Bash", Count: 4},
		{Kind: domain.ActorFindingFailedTool, Tool: "Edit", Count: 2},
		{Kind: domain.ActorFindingFailedTool, Tool: "Read", Count: 2},
		{Kind: domain.ActorFindingPeakContextVsType, Tokens: domain.Tokens(380_000), TypeAverage: domain.Tokens(250_000), TypeMembers: 3},
		{Kind: domain.ActorFindingCostVsType, Cost: known(1000), TypeCost: known(600), TypeMembers: 3},
	}
}

// sessionFindingsRun is the standard Session run with findings and tools.
func sessionFindingsRun() domain.RunReport {
	run := sessionRun()
	run.ActorSessions = []domain.ActorSession{
		{
			Actor: domain.Orchestrator(),
			Findings: []domain.ActorFinding{
				{Kind: domain.ActorFindingCostliestCall, Call: 3, Cost: known(40), Share: domain.Ratio{Numerator: 40, Denominator: 100}},
			},
			Tools: []domain.ToolFigures{{Tool: "Agent", Calls: 2, Durations: durations(2, 30*time.Second, 10*time.Second, 20*time.Second)}},
		},
		{Actor: sessRef("builder#1"), Findings: builderFindings(), Tools: builderTools()},
		{
			Actor: sessRef("coder#1"),
			Findings: []domain.ActorFinding{
				{Kind: domain.ActorFindingCostliestCall, Call: 4, Cost: known(65), Share: domain.Ratio{Numerator: 65, Denominator: 250}},
				{Kind: domain.ActorFindingPeakContextVsType, Tokens: domain.Tokens(300_000), TypeAverage: domain.Tokens(250_000), TypeMembers: 2},
			},
			Tools: []domain.ToolFigures{{Tool: "Bash", Calls: 1, Unpaired: 1}},
		},
		{Actor: sessRef("reviewer#1")},
	}
	return run
}

// sessFindingsView is the styling-free view of an actor's screen, tall enough to show all of it.
func sessFindingsView(t *testing.T, id string) string {
	t.Helper()
	return sessFindingsViewOf(t, sessionFindingsRun(), id, headerW)
}

func sessFindingsViewOf(t *testing.T, run domain.RunReport, id string, w int) string {
	t.Helper()
	return stripStyling(sessOpenFrom(t, run, id, screens.ScreenContext, w, legendOverlayH).View(w, legendOverlayH))
}

// sessLinesWith are the lines of the view that contain the text.
func sessLinesWith(view, text string) []string {
	var out []string
	for _, l := range viewLines(view) {
		if strings.Contains(l, text) {
			out = append(out, l)
		}
	}
	return out
}

// sessLineIndex is the index of the first line holding all the texts, or -1.
func sessLineIndex(view string, texts ...string) int {
next:
	for i, l := range viewLines(view) {
		for _, x := range texts {
			if !strings.Contains(l, x) {
				continue next
			}
		}
		return i
	}
	return -1
}

// sessToolsBlock is the tools table of the Session screen.
func sessToolsBlock(t *testing.T, view string) headerBlock {
	t.Helper()
	return readHeader(t, view, "tool")
}

// sessToolCells fails unless, in the row of the tool, each named column holds
// exactly the wanted text. pairs are column title, wanted cell text, ...
func sessToolCells(t *testing.T, view, tool string, pairs ...string) {
	t.Helper()
	block := sessToolsBlock(t, view)
	name, ok := block.col("tool")
	if !ok {
		t.Fatalf("the tools table has no \"tool\" column; its columns are %q", block.titles())
	}
	for _, row := range block.rows {
		r := runesOf(row)
		if strings.TrimSpace(fragment(r, name.start, name.end)) != tool {
			continue
		}
		for i := 0; i+1 < len(pairs); i += 2 {
			col, ok := block.col(pairs[i])
			if !ok {
				t.Errorf("the tools table has no column %q; its columns are %q", pairs[i], block.titles())
				continue
			}
			if got := strings.TrimSpace(fragment(r, col.start, col.end)); got != pairs[i+1] {
				t.Errorf("tool %q, column %q = %q, want %q", tool, pairs[i], got, pairs[i+1])
			}
		}
		return
	}
	t.Fatalf("the tools table has no row for %q; rows:\n%s", tool, strings.Join(block.rows, "\n"))
}
