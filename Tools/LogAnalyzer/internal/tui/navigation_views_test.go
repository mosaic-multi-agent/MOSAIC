package tui

// Navigation between the per-run views: the per-dispatch list (run detail),
// the run overview and the by-agent-type view. Tests drive the Model through
// Update and identify the current screen by its screens.ScreenID value, which
// the Model's screen name must equal for the per-run views.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	runA = "20260101T120000Z-aaaa"
	runB = "20260202T120000Z-bbbb"
)

func navCents(cents int64) domain.Totals {
	m := domain.KnownMoney(domain.Money(cents * 10_000_000))
	return domain.Totals{Money: domain.CategoryMoney{Total: m, Complete: true}}
}

// navViewsRun is a run with total `cents`, one orchestrator, one builder
// dispatch builder#1 and the overview / by-agent-type figures that go with it.
func navViewsRun(id string, cents int64) domain.RunReport {
	dispatch := domain.Dispatch{
		Invocation: domain.Invocation{
			Actor:      domain.AgentInstance("builder#1"),
			State:      domain.InvocationClosed,
			StatusCode: "SUCCESS",
			EndEvents:  1,
		},
		Totals: navCents(cents / 2),
	}
	totals := navCents(cents)
	return domain.RunReport{
		Run:          domain.NamedRun(id),
		Totals:       totals,
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: navCents(cents / 2)},
		Agents: []domain.ActorReport{
			{Actor: domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "builder#1", Type: "builder"}, Totals: navCents(cents / 2)},
		},
		Overview: domain.RunOverview{
			Totals:        totals,
			Orchestrator:  navCents(cents / 2),
			Subagents:     domain.GroupFigures{Invocations: 1, Totals: navCents(cents / 2)},
			CacheHitRatio: domain.Ratio{Numerator: 1, Denominator: 4},
		},
		ByAgentType: []domain.AgentTypeGroup{{
			AgentType:     "builder",
			Figures:       domain.GroupFigures{Invocations: 1, Totals: navCents(cents / 2)},
			PerInvocation: domain.MoneyStats{Basis: 1, Min: totals.Money.Total, Max: totals.Money.Total, Average: totals.Money.Total, Complete: true},
			Dispatches:    []domain.Dispatch{dispatch},
		}},
	}
}

func navViewsReport(runs ...domain.RunReport) domain.Report {
	return domain.Report{
		Runs:     runs,
		Currency: domain.Currency,
		Quality:  domain.NewQualitySummary(nil),
	}
}

// onRunsList returns a Model on the runs list holding the report.
func onRunsList(report domain.Report) Model {
	updated, _ := newNavModel().Update(analysisDoneMsg{result: app.Result{Outcome: app.OutcomeReport, Report: report}})
	return asModel(updated)
}

func send(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		updated, _ := m.Update(msg)
		m = asModel(updated)
	}
	return m
}

// viewKeys maps the key of each new per-run view to the screen it opens.
var viewKeys = map[string]screens.ScreenID{
	screens.KeyViewOverview:   screens.ScreenOverview,
	screens.KeyViewAgentTypes: screens.ScreenAgentTypes,
}

func assertOn(t *testing.T, m Model, want screens.ScreenID) {
	t.Helper()
	if string(m.screen) != string(want) {
		t.Errorf("screen = %q, want %q", m.screen, want)
	}
}

// onRunDetail returns a Model showing the run detail of the first run.
func onRunDetail(t *testing.T, report domain.Report) Model {
	t.Helper()
	m := send(onRunsList(report), "enter")
	assertOn(t, m, screens.ScreenRunDetail)
	return m
}

func TestViews_RunDetailReachesOverviewAndAgentTypes(t *testing.T) {
	report := navViewsReport(navViewsRun(runA, 1000))
	cases := []struct {
		key  string
		want screens.ScreenID
	}{
		{screens.KeyViewOverview, screens.ScreenOverview},
		{screens.KeyViewAgentTypes, screens.ScreenAgentTypes},
	}
	for _, c := range cases {
		m := send(onRunDetail(t, report), c.key)

		assertOn(t, m, c.want)
		if strings.TrimSpace(m.View()) == "" {
			t.Errorf("key %q: the view must render content", c.key)
		}
	}
}

func TestViews_PerRunViewsSwitchBetweenEachOther(t *testing.T) {
	m := onRunDetail(t, navViewsReport(navViewsRun(runA, 1000)))

	m = send(m, screens.KeyViewOverview)
	assertOn(t, m, screens.ScreenOverview)
	m = send(m, screens.KeyViewAgentTypes)
	assertOn(t, m, screens.ScreenAgentTypes)
	m = send(m, screens.KeyViewOverview)
	assertOn(t, m, screens.ScreenOverview)
}

func TestViews_PerDispatchListStaysReachableFromTheNewViews(t *testing.T) {
	for key, id := range viewKeys {
		m := send(onRunDetail(t, navViewsReport(navViewsRun(runA, 1000))), key)
		assertOn(t, m, id)

		m = send(m, screens.KeyViewDispatches)

		assertOn(t, m, screens.ScreenRunDetail)
		view := m.View()
		if !strings.Contains(view, "builder#1") || !strings.Contains(strings.ToLower(view), "orchestrator") {
			t.Errorf("per-dispatch list must list the orchestrator and builder#1\n%s", view)
		}
	}
}

func TestViews_BackFromAnyPerRunViewReturnsToRunsList(t *testing.T) {
	for key, id := range viewKeys {
		for _, back := range []string{"esc", "q"} {
			m := send(onRunDetail(t, navViewsReport(navViewsRun(runA, 1000))), key)
			assertOn(t, m, id)

			m = send(m, back)

			assertOn(t, m, screens.ScreenRuns)
		}
	}
}

func TestViews_AgentTypesDrillDownBackReturnsToTypeListThenRuns(t *testing.T) {
	m := send(onRunDetail(t, navViewsReport(navViewsRun(runA, 1000))), screens.KeyViewAgentTypes)
	if strings.Contains(m.View(), "builder#1") {
		t.Fatalf("type list must not list individual dispatches\n%s", m.View())
	}

	m = send(m, "enter")
	assertOn(t, m, screens.ScreenAgentTypes)
	if !strings.Contains(m.View(), "builder#1") {
		t.Errorf("drill-down must list the dispatch builder#1\n%s", m.View())
	}

	m = send(m, "esc")
	assertOn(t, m, screens.ScreenAgentTypes)
	if strings.Contains(m.View(), "builder#1") {
		t.Errorf("back from the drill-down must show the type list again\n%s", m.View())
	}

	m = send(m, "esc")
	assertOn(t, m, screens.ScreenRuns)
}

func TestViews_ShowTheSelectedRunNotTheFirst(t *testing.T) {
	report := navViewsReport(navViewsRun(runA, 700), navViewsRun(runB, 300))

	m := send(onRunsList(report), "down", "enter", screens.KeyViewOverview)

	assertOn(t, m, screens.ScreenOverview)
	view := m.View()
	// The runs list is sorted by cost, most expensive first: the second row is the
	// cheaper run, runB ($3.00), and the first row is runA ($7.00).
	if !strings.Contains(view, "$3.00") || strings.Contains(view, "$7.00") {
		t.Errorf("overview must show the second row's run ($3.00) and not the first's ($7.00)\n%s", view)
	}
}

func TestViews_ReopeningAfterBackShowsTheSameRunAgain(t *testing.T) {
	report := navViewsReport(navViewsRun(runA, 700), navViewsRun(runB, 300))

	m := send(onRunsList(report), "down", "enter", screens.KeyViewOverview, "esc")
	assertOn(t, m, screens.ScreenRuns)
	m = send(m, "enter", screens.KeyViewOverview)

	assertOn(t, m, screens.ScreenOverview)
	if !strings.Contains(m.View(), "$3.00") {
		t.Errorf("runs cursor must survive the round trip\n%s", m.View())
	}
}

func TestViews_AfterRepricingTheViewsShowTheNewFigures(t *testing.T) {
	before := navViewsReport(navViewsRun(runA, 300))
	after := navViewsReport(navViewsRun(runA, 700))
	m := onRunsList(before)

	updated, _ := m.Update(pricingEditDoneMsg{result: app.PricingEditResult{
		Outcome: app.PricingEditSaved, Model: "model-a", Report: after, ConfigPath: "/test/pricing.yaml",
	}})
	m = asModel(updated)
	assertOn(t, m, screens.ScreenRuns)

	for key, id := range viewKeys {
		next := send(m, "enter", key)
		assertOn(t, next, id)
		view := next.View()
		if !strings.Contains(view, "$7.00") || strings.Contains(view, "$3.00") {
			t.Errorf("key %q: view must show the re-priced $7.00 and not the old $3.00\n%s", key, view)
		}
	}
}

func TestViews_OverviewScrollsOnAShortTerminal(t *testing.T) {
	run := navViewsRun(runA, 1000)
	for i := 1; i <= 30; i++ {
		run.Overview.ByModel = append(run.Overview.ByModel, domain.ModelFigures{
			Model: domain.ModelID(fmt.Sprintf("model-%02d", i)), Priced: true, Requests: 1, Totals: navCents(1),
		})
	}
	m := onRunDetail(t, navViewsReport(run))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	m = send(asModel(updated), screens.KeyViewOverview)

	view := m.View()
	if n := strings.Count(view, "\n") + 1; n > 8 {
		t.Errorf("view has %d lines on an 8-line terminal\n%s", n, view)
	}
	if strings.Contains(view, "model-30") {
		t.Errorf("the last model must be scrolled out of view\n%s", view)
	}

	m = send(m, strings.Fields(strings.Repeat("down ", 80))...)
	if !strings.Contains(m.View(), "model-30") {
		t.Errorf("scrolling down must reach the last model\n%s", m.View())
	}
}
