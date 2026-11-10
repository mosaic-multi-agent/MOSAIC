package screens_test

// Behaviour of the overview and by-agent screens for run-level unattributed
// usage: the overview shows its share next to orchestrator and subagents, the
// by-agent screen shows its own row after the agent-type groups, and a run
// without it renders as before.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// withUnattributed adds $2.00 of unattributed usage (16.7% of a $12.00 run) to
// run, as the analysis does: the run-level actor report, the overview totals
// and the overview share.
func withUnattributed(run domain.RunReport) domain.RunReport {
	totals := moneyTotals(200)
	run.Unattributed = &domain.ActorReport{Actor: domain.Unattributed(), Totals: totals}
	run.Overview.Unattributed = &totals
	run.Overview.UnattributedShare = domain.Ratio{Numerator: 200 * 10_000_000, Denominator: 1200 * 10_000_000}
	return run
}

func TestOverview_FiguresOnlyShowsUnattributedShareAndAmount(t *testing.T) {
	view := overviewFiguresView(withUnattributed(overviewRun()), tallH)

	assertContains(t, strings.ToLower(view), "unattributed")
	line := lineWith(strings.ToLower(view), "unattributed")
	for _, want := range []string{"16.7%", "$2.00"} {
		if !strings.Contains(line, want) {
			t.Errorf("unattributed line %q must show %q\n%s", line, want, view)
		}
	}
}

func TestOverview_VisualNamesUnattributedWithAShareInTheCostLegend(t *testing.T) {
	colourOff(t)

	view := overviewView(withUnattributed(overviewRun()), tallH)

	assertMatches(t, view, `(?i)unattributed[^%]*\d+%`)
}

func TestOverview_UnattributedStaysNextToOrchestratorAndSubagentsWithOwnFigures(t *testing.T) {
	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(withUnattributed(overviewRun()), tallH)

			low := strings.ToLower(view)
			o, s, u := strings.Index(low, "orchestrator"), strings.Index(low, "subagent"), strings.Index(low, "unattributed")
			if o < 0 || s < 0 || u < 0 {
				t.Fatalf("orchestrator, subagents and unattributed must all be shown\n%s", view)
			}
			if u < o || u < s {
				t.Errorf("unattributed share must follow the orchestrator and subagent shares (got %d, %d, %d)", o, s, u)
			}
		})
	}
	assertContains(t, overviewFiguresView(withUnattributed(overviewRun()), tallH), "40.0%", "60.0%", "$4.00", "$6.00")
}

func TestOverview_RunWithoutUnattributedUsageShowsNoUnattributedLine(t *testing.T) {
	view := overviewView(overviewRun(), tallH)

	assertNotContains(t, strings.ToLower(view), "unattributed")
}

func TestOverview_UnattributedRunKeepsFullViewWithinShortTerminalScrolling(t *testing.T) {
	run := withUnattributed(manyModelsRun(20))

	view := overviewView(run, 200)

	assertContains(t, strings.ToLower(view), "unattributed")
	assertContains(t, view, "model-01", "model-20")
}

func TestAgentTypes_ShowsUnattributedRowWithItsFigures(t *testing.T) {
	view := typesView(withUnattributed(agentTypesRun()), tallH)

	row := lineWith(strings.ToLower(view), "unattributed")
	if row == "" {
		t.Fatalf("no unattributed row\n%s", view)
	}
	if !strings.Contains(row, "$2.00") {
		t.Errorf("unattributed row %q must show its total $2.00", row)
	}
}

func TestAgentTypes_UnattributedRowFollowsEveryAgentTypeGroup(t *testing.T) {
	view := typesView(withUnattributed(agentTypesRun()), tallH)

	low := strings.ToLower(view)
	u := strings.Index(low, "unattributed")
	for _, name := range []string{"builder", "reviewer", strings.ToLower(screens.UnknownTypeLabel)} {
		i := strings.Index(low, name)
		if i < 0 || u < 0 || u < i {
			t.Errorf("unattributed row (at %d) must come after the %q group (at %d)\n%s", u, name, i, view)
		}
	}
}

func TestAgentTypes_UnattributedUsageIsNotFoldedIntoAnyGroup(t *testing.T) {
	plainView := typesView(agentTypesRun(), tallH)
	view := typesView(withUnattributed(agentTypesRun()), tallH)

	for _, figure := range []string{"$60.25", "$20.50", "$7.77"} {
		assertContains(t, plainView, figure)
		assertContains(t, view, figure)
	}
	if got := strings.Count(view, "builder"); got != strings.Count(plainView, "builder") {
		t.Errorf("builder appears %d times, want unchanged %d", got, strings.Count(plainView, "builder"))
	}
}

func TestAgentTypes_UnattributedRowShownEvenWithoutAgentTypeGroups(t *testing.T) {
	run := withUnattributed(overviewRun())

	view := typesView(run, tallH)

	assertContains(t, strings.ToLower(view), "unattributed")
	assertContains(t, view, "$2.00")
}

func TestAgentTypes_RunWithoutUnattributedUsageShowsNoUnattributedRow(t *testing.T) {
	view := typesView(agentTypesRun(), tallH)

	assertNotContains(t, strings.ToLower(view), "unattributed")
}

func TestAgentTypes_UnattributedRowStaysWithinViewportWhenGroupsFillIt(t *testing.T) {
	const height = 12
	run := withUnattributed(manyTypesRun(30))

	view := typesView(run, height)

	assertContains(t, strings.ToLower(view), "unattributed")
	if got := strings.Count(view, "\n") + 1; got > height {
		t.Errorf("view has %d lines, want at most %d\n%s", got, height, view)
	}
}
