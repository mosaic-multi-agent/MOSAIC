package tui

// Navigation to and from the Context per-run view, reusing the helpers of the
// other per-run view navigation tests.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// contextCallLabel is the clock label of the single call of contextNavRun.
const contextCallLabel = "10:00:10"

// contextNavRun is navViewsRun plus one invocation with one per-call record,
// named after the run so the Context view shows which run it was built from.
func contextNavRun(id string, cents int64, instance string) domain.RunReport {
	run := navViewsRun(id, cents)
	figure := domain.ContextFigure{Tokens: domain.Tokens(120_000), Band: domain.BandOptimal}
	run.Context = domain.RunContext{
		Invocations: []domain.ActorContext{{
			Actor: domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(instance), Type: "builder"},
			Calls: []domain.ContextCall{{
				At:       timelineBase.Add(10 * time.Second),
				RecordID: "rec-1",
				Model:    "model-x",
				Usage:    domain.TokenUsage{Input: domain.Tokens(100_000), CacheRead: domain.Tokens(15_000), CacheCreation: domain.Tokens(5_000), Output: domain.Tokens(300)},
				Context:  domain.Tokens(120_000),
			}},
			First: figure, Peak: figure, Final: figure,
		}},
	}
	return run
}

// selectContextRow moves the cursor of the Context view to the row of the
// instance.
func selectContextRow(t *testing.T, m Model, instance string) Model {
	t.Helper()
	for i := 0; i < 6; i++ {
		for _, l := range strings.Split(m.View(), "\n") {
			if strings.Contains(l, instance) && strings.HasPrefix(strings.TrimSpace(l), ">") {
				return m
			}
		}
		m = send(m, "down")
	}
	t.Fatalf("the cursor never reached the row of %s\n%s", instance, m.View())
	return m
}

func TestContext_RunDetailReachesTheContextViewWithContent(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 1000, "worker-a"))

	m := send(onRunDetail(t, report), screens.KeyViewContext)

	assertOn(t, m, screens.ScreenContext)
	if view := m.View(); !strings.Contains(view, "worker-a") {
		t.Errorf("context view must list the run's invocation worker-a\n%s", view)
	}
}

func TestContext_ReachableFromEveryOtherPerRunView(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 1000, "worker-a"))
	starts := map[string]screens.ScreenID{screens.KeyViewTimeline: screens.ScreenTimeline}
	for key, id := range otherViewKeys {
		starts[key] = id
	}
	for start := range starts {
		m := send(onRunDetail(t, report), start)

		m = send(m, screens.KeyViewContext)

		assertOn(t, m, screens.ScreenContext)
	}
}

func TestContext_EveryOtherPerRunViewStaysReachableFromIt(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 1000, "worker-a"))
	targets := map[string]screens.ScreenID{screens.KeyViewTimeline: screens.ScreenTimeline}
	for key, id := range otherViewKeys {
		targets[key] = id
	}
	for key, id := range targets {
		m := send(onRunDetail(t, report), screens.KeyViewContext)
		assertOn(t, m, screens.ScreenContext)

		m = send(m, key)

		assertOn(t, m, id)
	}
}

func TestContext_TabBarOfAnotherViewListsTheContextTab(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 1000, "worker-a"))

	m := onRunDetail(t, report)

	if view := m.View(); !strings.Contains(view, "7 Context") {
		t.Errorf("the tab bar must list the Context view under key 7\n%s", view)
	}
}

func TestContext_EscReturnsToTheRunsList(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 1000, "worker-a"))
	m := send(onRunDetail(t, report), screens.KeyViewContext)
	assertOn(t, m, screens.ScreenContext)

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenRuns)
}

func TestContext_ShowsTheSelectedRunNotTheFirst(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 700, "worker-a"), contextNavRun(runB, 300, "worker-b"))

	m := send(onRunsList(report), "down", "enter", screens.KeyViewContext)

	assertOn(t, m, screens.ScreenContext)
	if view := m.View(); !strings.Contains(view, "worker-b") || strings.Contains(view, "worker-a") {
		t.Errorf("context view must show the second run's worker-b and not worker-a\n%s", view)
	}
}

func TestContext_EnterOpensTheSessionScreenAndEscReturnsToTheTableBeforeLeaving(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 1000, "worker-a"))
	m := send(onRunDetail(t, report), screens.KeyViewContext)
	assertOn(t, m, screens.ScreenContext)
	if strings.Contains(m.View(), contextCallLabel) {
		t.Fatalf("the table must not list calls\n%s", m.View())
	}
	m = selectContextRow(t, m, "worker-a")

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); !strings.Contains(view, contextCallLabel) {
		t.Errorf("the Session screen must list the call made at %s\n%s", contextCallLabel, view)
	}

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenContext)
	if view := m.View(); strings.Contains(view, contextCallLabel) || !strings.Contains(view, "worker-a") {
		t.Errorf("the first esc must return to the table of actors\n%s", view)
	}

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenRuns)
}
