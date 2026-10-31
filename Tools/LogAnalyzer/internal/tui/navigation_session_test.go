package tui

// Navigation to and from the Session screen: enter on a row of the Dispatches
// table, the By type drill-down and the Context table opens the Session screen of
// that actor, and esc returns to the screen it was opened from with its cursor,
// sort and drill state kept. Exports started from the Session screen keep working.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// sessionNavReport is a report whose run has the orchestrator, the dispatch
// builder#1 and the Context invocation worker-a with one per-call record.
func sessionNavReport() domain.Report {
	return navViewsReport(contextNavRun(runA, 1000, "worker-a"))
}

// selectedRow is the table row the cursor marks in the model's view.
func selectedRow(m Model) string {
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), ">") {
			return l
		}
	}
	return ""
}

func TestSession_DispatchesEnterOnADispatchOpensItsSessionAndEscReturnsWithTheCursor(t *testing.T) {
	m := send(onDispatches(t), "down")
	before := m.View()
	if row := selectedRow(m); !strings.Contains(row, "builder#1") {
		t.Fatalf("setup: the cursor must mark the dispatch builder#1, marks %q", row)
	}

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); !strings.Contains(view, "builder#1") || strings.Contains(view, "Run totals") {
		t.Errorf("the Session screen of builder#1 must replace the Dispatches table\n%s", view)
	}

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenRunDetail)
	if after := m.View(); after != before {
		t.Errorf("esc must show the Dispatches table as it was left\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// onDispatches is the Dispatches view of the session report.
func onDispatches(t *testing.T) Model {
	t.Helper()
	return onRunDetail(t, sessionNavReport())
}

func TestSession_DispatchesEnterOnTheOrchestratorRowOpensTheOrchestratorSession(t *testing.T) {
	m := onDispatches(t)
	if row := selectedRow(m); !strings.Contains(row, "orchestrator") {
		t.Fatalf("setup: the cursor starts on the orchestrator row, marks %q", row)
	}

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); strings.Contains(view, "builder#1") {
		t.Errorf("the Session screen of the orchestrator must not list the dispatches\n%s", view)
	}
}

func TestSession_DispatchesEscKeepsTheSortOfTheTable(t *testing.T) {
	m := send(onDispatches(t), screens.KeySort, screens.KeySort, "down")
	before := m.View()

	m = send(m, "enter", "esc")

	assertOn(t, m, screens.ScreenRunDetail)
	if after := m.View(); after != before {
		t.Errorf("the sort and cursor of the Dispatches table must be kept\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestSession_ByTypeDrillDownEnterOpensTheSessionAndEscReturnsToTheDrillDown(t *testing.T) {
	m := send(onDispatches(t), screens.KeyViewAgentTypes, "enter")
	assertOn(t, m, screens.ScreenAgentTypes)
	before := m.View()
	if !strings.Contains(before, "builder#1") {
		t.Fatalf("setup: the drill-down must list the dispatch builder#1\n%s", before)
	}

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); strings.Contains(view, "Dispatches of") {
		t.Errorf("the Session screen must replace the drill-down\n%s", view)
	}

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenAgentTypes)
	if after := m.View(); after != before {
		t.Errorf("esc must return to the drill-down as it was left\nbefore:\n%s\nafter:\n%s", before, after)
	}
	m = send(m, "esc")
	assertOn(t, m, screens.ScreenAgentTypes)
	if strings.Contains(m.View(), "builder#1") {
		t.Errorf("the next esc closes the drill-down to the type list\n%s", m.View())
	}
	assertOn(t, send(m, "esc"), screens.ScreenRuns)
}

func TestSession_ByTypeEnterOnTheTypeTableDrillsDownAndDoesNotOpenASession(t *testing.T) {
	m := send(onDispatches(t), screens.KeyViewAgentTypes, "enter")

	if string(m.screen) == string(screens.ScreenSession) {
		t.Error("enter on the type table must show the dispatches of the type first")
	}
}

func TestSession_ContextEnterOnAnInvocationShowsItsCallsAndEscKeepsTheCursor(t *testing.T) {
	m := send(onDispatches(t), screens.KeyViewContext)
	m = selectContextRow(t, m, "worker-a")
	before := m.View()

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); !strings.Contains(view, contextCallLabel) || !strings.Contains(view, "worker-a") {
		t.Errorf("the Session screen must list worker-a and its call at %s\n%s", contextCallLabel, view)
	}

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenContext)
	if after := m.View(); after != before {
		t.Errorf("esc must return to the Context table with worker-a still selected\nbefore:\n%s\nafter:\n%s", before, after)
	}
	assertOn(t, send(m, "esc"), screens.ScreenRuns)
}

func TestSession_ContextEnterOnTheOrchestratorRowOpensTheOrchestratorSession(t *testing.T) {
	m := send(onDispatches(t), screens.KeyViewContext, "down")
	if row := selectedRow(m); !strings.Contains(row, "orchestrator") {
		t.Fatalf("setup: the first row down is the orchestrator, marks %q", row)
	}

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); strings.Contains(view, "worker-a") {
		t.Errorf("the Session screen of the orchestrator must not list the invocation worker-a\n%s", view)
	}
}

func TestSession_QReturnsToTheOpenerLikeEsc(t *testing.T) {
	m := send(onDispatches(t), screens.KeyViewContext)
	m = selectContextRow(t, m, "worker-a")

	m = send(m, "enter", "q")

	assertOn(t, m, screens.ScreenContext)
}

func TestSession_FrameHighlightsTheTabOfTheScreenItWasOpenedFrom(t *testing.T) {
	cases := []struct {
		name string
		open func() Model
		tab  string
	}{
		{"dispatches", func() Model { return send(onDispatches(t), "enter") }, "[1 Dispatches]"},
		{"by type", func() Model { return send(onDispatches(t), screens.KeyViewAgentTypes, "enter", "enter") }, "[3 By type]"},
		{"context", func() Model {
			return send(selectContextRow(t, send(onDispatches(t), screens.KeyViewContext), "worker-a"), "enter")
		}, "[7 Context]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := c.open()

			assertOn(t, m, screens.ScreenSession)
			if lines := strings.Split(m.View(), "\n"); len(lines) < 2 || !strings.Contains(lines[1], c.tab) {
				t.Errorf("the tab bar must highlight %s\n%s", c.tab, m.View())
			}
		})
	}
}

func TestSession_EveryOtherPerRunViewKeyLeavesTheSessionForThatView(t *testing.T) {
	targets := map[string]screens.ScreenID{screens.KeyViewTimeline: screens.ScreenTimeline, screens.KeyViewOverview: screens.ScreenOverview}
	for key, id := range otherViewKeys {
		targets[key] = id
	}
	for key, id := range targets {
		m := send(onDispatches(t), "enter")
		assertOn(t, m, screens.ScreenSession)

		m = send(m, key)

		assertOn(t, m, id)
	}
}

func TestSession_TheOpenersOwnViewKeyShowsThatViewAfresh(t *testing.T) {
	m := send(onDispatches(t), screens.KeyViewContext)
	m = send(selectContextRow(t, m, "worker-a"), "enter")
	assertOn(t, m, screens.ScreenSession)

	m = send(m, screens.KeyViewContext)

	assertOn(t, m, screens.ScreenContext)
	if view := m.View(); strings.Contains(view, contextCallLabel) {
		t.Errorf("the Context view must show its table, not the calls of an actor\n%s", view)
	}
}

func TestSession_ShowsTheRunOfTheSelectedRunNotTheFirst(t *testing.T) {
	report := navViewsReport(contextNavRun(runA, 700, "worker-a"), contextNavRun(runB, 300, "worker-b"))
	m := send(onRunsList(report), "down", "enter", screens.KeyViewContext)
	m = selectContextRow(t, m, "worker-b")

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); !strings.Contains(view, "worker-b") || strings.Contains(view, "worker-a") {
		t.Errorf("the Session screen must show worker-b of the second run and not worker-a\n%s", view)
	}
}

func TestSession_ExportFromTheSessionScreenExportsTheShownRunAndReturnsToIt(t *testing.T) {
	m := openRunDetailOfSecondRun(newExportModel(t, newTestDigestStore(false)))
	m = send(m, "enter")
	assertOn(t, m, screens.ScreenSession)

	m = press(m, "e")

	requireExportScreen(t, m)
	if view := m.View(); !strings.Contains(view, "Export run "+exportRunB) {
		t.Errorf("the export view must name the shown run %s\n%s", exportRunB, view)
	}

	m = pressType(m, tea.KeyEsc)

	assertOn(t, m, screens.ScreenSession)
	if view := m.View(); !strings.Contains(view, "Export cancelled") {
		t.Errorf("the Session screen must show the export outcome\n%s", view)
	}
}

func TestSession_ExportAllFromTheSessionScreenOpensTheAllRunsExport(t *testing.T) {
	m := send(openRunDetailOfSecondRun(newExportModel(t, newTestDigestStore(false))), "enter")
	assertOn(t, m, screens.ScreenSession)

	m = press(m, "E")

	requireExportScreen(t, m)
	if view := m.View(); !strings.Contains(view, "Export all runs") {
		t.Errorf("the export view must offer all runs\n%s", view)
	}
}
