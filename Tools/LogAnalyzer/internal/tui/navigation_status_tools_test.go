package tui

// Navigation to and from the by-status and tools per-run views, reusing the
// helpers of the other per-run view navigation tests.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// statusToolsKeys maps the key of each of the two views to the screen it opens.
var statusToolsKeys = map[string]screens.ScreenID{
	screens.KeyViewStatus: screens.ScreenStatus,
	screens.KeyViewTools:  screens.ScreenTools,
}

// statusToolsRun is navViewsRun plus one status group with the run's money and
// one tool named after the run, so each view shows which run it was built from.
func statusToolsRun(id string, cents int64, tool string) domain.RunReport {
	run := navViewsRun(id, cents)
	run.ByStatus = domain.StatusBreakdown{
		Run: []domain.StatusGroup{{
			Kind: domain.StatusReported, Code: "SUCCESS",
			Figures: domain.GroupFigures{Invocations: 1, Totals: navCents(cents)},
		}},
		ByAgentType: []domain.AgentTypeStatus{{AgentType: "builder", Groups: []domain.StatusGroup{{
			Kind: domain.StatusReported, Code: "SUCCESS",
			Figures: domain.GroupFigures{Invocations: 1, Totals: navCents(cents)},
		}}}},
	}
	run.Tools = domain.ToolsBreakdown{
		Run: []domain.ToolFigures{{Tool: tool, Calls: 2}},
	}
	return run
}

func TestStatusTools_RunDetailReachesBothViewsWithContent(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 1000, "ToolA"))
	want := map[string]string{screens.KeyViewStatus: "SUCCESS", screens.KeyViewTools: "ToolA"}
	for key, id := range statusToolsKeys {
		m := send(onRunDetail(t, report), key)

		assertOn(t, m, id)
		if view := m.View(); !strings.Contains(view, want[key]) {
			t.Errorf("key %q: view must show %q\n%s", key, want[key], view)
		}
	}
}

func TestStatusTools_ReachableFromEveryOtherPerRunView(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 1000, "ToolA"))
	starts := []string{
		screens.KeyViewDispatches, screens.KeyViewOverview, screens.KeyViewAgentTypes,
		screens.KeyViewStatus, screens.KeyViewTools,
	}
	for _, start := range starts {
		for key, id := range statusToolsKeys {
			m := send(onRunDetail(t, report), start)

			m = send(m, key)

			assertOn(t, m, id)
		}
	}
}

func TestStatusTools_ExistingViewsStayReachableFromTheNewViews(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 1000, "ToolA"))
	targets := map[string]screens.ScreenID{
		screens.KeyViewDispatches: screens.ScreenRunDetail,
		screens.KeyViewOverview:   screens.ScreenOverview,
		screens.KeyViewAgentTypes: screens.ScreenAgentTypes,
	}
	for key := range statusToolsKeys {
		for target, id := range targets {
			m := send(onRunDetail(t, report), key)
			assertOn(t, m, statusToolsKeys[key])

			m = send(m, target)

			assertOn(t, m, id)
		}
	}
}

func TestStatusTools_PerDispatchListStillListsOrchestratorAndDispatches(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 1000, "ToolA"))
	for key := range statusToolsKeys {
		m := send(onRunDetail(t, report), key)
		assertOn(t, m, statusToolsKeys[key])

		m = send(m, screens.KeyViewDispatches)

		assertOn(t, m, screens.ScreenRunDetail)
		view := m.View()
		if !strings.Contains(view, "builder#1") || !strings.Contains(strings.ToLower(view), "orchestrator") {
			t.Errorf("per-dispatch list must list the orchestrator and builder#1\n%s", view)
		}
	}
}

func TestStatusTools_BackReturnsToRunsList(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 1000, "ToolA"))
	for key, id := range statusToolsKeys {
		for _, back := range []string{"esc", "q"} {
			m := send(onRunDetail(t, report), key)
			assertOn(t, m, id)

			m = send(m, back)

			assertOn(t, m, screens.ScreenRuns)
		}
	}
}

func TestStatusTools_ShowTheSelectedRunNotTheFirst(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 700, "ToolA"), statusToolsRun(runB, 300, "ToolB"))

	status := send(onRunsList(report), "down", "enter", screens.KeyViewStatus)
	tools := send(onRunsList(report), "down", "enter", screens.KeyViewTools)

	assertOn(t, status, screens.ScreenStatus)
	// The runs list is sorted by cost, most expensive first: the second row is the
	// cheaper run, runB ($3.00), and the first row is runA ($7.00).
	if view := status.View(); !strings.Contains(view, "$3.00") || strings.Contains(view, "$7.00") {
		t.Errorf("status view must show the second row's run ($3.00) and not the first's ($7.00)\n%s", view)
	}
	assertOn(t, tools, screens.ScreenTools)
	if view := tools.View(); !strings.Contains(view, "ToolB") || strings.Contains(view, "ToolA") {
		t.Errorf("tools view must show the second run's ToolB and not ToolA\n%s", view)
	}
}

func TestStatusTools_ReopeningAfterBackShowsTheSameRunAgain(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 700, "ToolA"), statusToolsRun(runB, 300, "ToolB"))

	m := send(onRunsList(report), "down", "enter", screens.KeyViewStatus, "esc")
	assertOn(t, m, screens.ScreenRuns)
	m = send(m, "enter", screens.KeyViewStatus)

	assertOn(t, m, screens.ScreenStatus)
	if !strings.Contains(m.View(), "$3.00") {
		t.Errorf("runs cursor must survive the round trip\n%s", m.View())
	}
}

func TestStatusTools_AfterRepricingStatusShowsTheNewFigures(t *testing.T) {
	m := onRunsList(navViewsReport(statusToolsRun(runA, 300, "ToolA")))
	after := navViewsReport(statusToolsRun(runA, 700, "ToolA"))

	updated, _ := m.Update(pricingEditDoneMsg{result: app.PricingEditResult{
		Outcome: app.PricingEditSaved, Model: "model-a", Report: after, ConfigPath: "/test/pricing.yaml",
	}})
	m = send(asModel(updated), "enter", screens.KeyViewStatus)

	assertOn(t, m, screens.ScreenStatus)
	if view := m.View(); !strings.Contains(view, "$7.00") || strings.Contains(view, "$3.00") {
		t.Errorf("status view must show the re-priced $7.00 and not the old $3.00\n%s", view)
	}
}

// shortTerminalH is short enough to scroll 30 rows, yet tall enough for the Tools
// three-line header group, a row and the content-capture note.
const shortTerminalH = 14

func TestStatusTools_ScrollOnAShortTerminal(t *testing.T) {
	run := statusToolsRun(runA, 1000, "ToolA")
	for i := 1; i <= 30; i++ {
		run.ByStatus.ByAgentType = append(run.ByStatus.ByAgentType, domain.AgentTypeStatus{
			AgentType: fmt.Sprintf("type-%02d", i),
			Groups: []domain.StatusGroup{{
				Kind: domain.StatusReported, Code: "SUCCESS", Figures: domain.GroupFigures{Invocations: 1, Totals: navCents(1)},
			}},
		})
		run.Tools.Run = append(run.Tools.Run, domain.ToolFigures{Tool: fmt.Sprintf("tool-%02d", i), Calls: 1})
	}
	cases := map[string]string{screens.KeyViewStatus: "type-30", screens.KeyViewTools: "tool-30"}
	for key, last := range cases {
		m := onRunDetail(t, navViewsReport(run))
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: shortTerminalH})
		m = send(asModel(updated), key)

		view := m.View()
		if n := strings.Count(view, "\n") + 1; n > shortTerminalH {
			t.Errorf("key %q: view has %d lines on a %d-line terminal\n%s", key, n, shortTerminalH, view)
		}
		if strings.Contains(view, last) {
			t.Errorf("key %q: %s must be scrolled out of view\n%s", key, last, view)
		}

		m = send(m, strings.Fields(strings.Repeat("down ", 120))...)
		if !strings.Contains(m.View(), last) {
			t.Errorf("key %q: scrolling down must reach %s\n%s", key, last, m.View())
		}
	}
}

func TestStatusTools_BackAfterSwitchingBetweenTheNewViewsReturnsToRunsList(t *testing.T) {
	report := navViewsReport(statusToolsRun(runA, 1000, "ToolA"))
	m := send(onRunDetail(t, report), screens.KeyViewStatus)
	assertOn(t, m, screens.ScreenStatus)
	m = send(m, screens.KeyViewTools)
	assertOn(t, m, screens.ScreenTools)

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenRuns)
}
