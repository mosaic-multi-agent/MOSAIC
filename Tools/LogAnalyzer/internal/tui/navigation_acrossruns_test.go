package tui

// Navigation to and from the across-runs view, reusing the helpers of the
// per-run view navigation tests. The view opens from the runs list only and
// shows the report's across-runs figures.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// acrossNavReport has two runs and carries their across-runs figures: run A
// totals $3.00 and run B $7.00; the "builder" type averages $5.00 per run.
func acrossNavReport() domain.Report {
	report := navViewsReport(navViewsRun(runA, 300), navViewsRun(runB, 700))
	report.AcrossRuns = domain.AcrossRuns{
		Runs: []domain.RunSummary{
			{Run: domain.NamedRun(runA), Totals: navCents(300)},
			{Run: domain.NamedRun(runB), Totals: navCents(700)},
		},
		ByAgentType: []domain.AgentTypeAcrossRuns{{
			AgentType: "builder", Runs: 2,
			Figures:           domain.GroupFigures{Invocations: 2, Totals: navCents(1000)},
			PerRunInvocations: domain.Ratio{Numerator: 2, Denominator: 2},
			PerRunMoney: domain.MoneyStats{
				Basis: 2, Min: navCents(300).Money.Total, Max: navCents(700).Money.Total,
				Average: navCents(500).Money.Total, Complete: true,
			},
		}},
	}
	return report
}

func TestAcrossRuns_RunsListReachesTheViewWithContent(t *testing.T) {
	m := send(onRunsList(acrossNavReport()), screens.KeyAcrossRuns)

	assertOn(t, m, screens.ScreenAcrossRuns)
	view := m.View()
	for _, want := range []string{runA, runB, "$3.00", "$7.00", "builder", "$5.00"} {
		if !strings.Contains(view, want) {
			t.Errorf("across-runs view must contain %q\n%s", want, view)
		}
	}
}

func TestAcrossRuns_BackReturnsToRunsList(t *testing.T) {
	for _, back := range []string{"esc", "q"} {
		m := send(onRunsList(acrossNavReport()), screens.KeyAcrossRuns)
		assertOn(t, m, screens.ScreenAcrossRuns)

		m = send(m, back)

		assertOn(t, m, screens.ScreenRuns)
	}
}

func TestAcrossRuns_ExistingViewsStayReachableFromTheRunsList(t *testing.T) {
	report := acrossNavReport()

	m := send(onRunsList(report), "enter")
	assertOn(t, m, screens.ScreenRunDetail)

	m = send(onRunsList(report), "s")
	assertOn(t, m, screens.ScreenSource)
}

func TestAcrossRuns_ReturningToTheRunsListKeepsRunSelectionAndDetailReachable(t *testing.T) {
	m := send(onRunsList(acrossNavReport()), "down", screens.KeyAcrossRuns)
	assertOn(t, m, screens.ScreenAcrossRuns)

	m = send(m, "esc")
	assertOn(t, m, screens.ScreenRuns)

	m = send(m, "enter")

	assertOn(t, m, screens.ScreenRunDetail)
	if view := m.View(); !strings.Contains(view, runB) {
		t.Errorf("the run detail must show the second run %s selected before the detour\n%s", runB, view)
	}
}

func TestAcrossRuns_KeyDoesNothingOutsideTheRunsList(t *testing.T) {
	m := send(onRunDetail(t, acrossNavReport()), screens.KeyAcrossRuns)

	assertOn(t, m, screens.ScreenRunDetail)
}

func TestAcrossRuns_ScrollOnAShortTerminal(t *testing.T) {
	report := acrossNavReport()
	report.AcrossRuns.ByAgentType = nil
	report.AcrossRuns.Runs = nil
	for i := 1; i <= 30; i++ {
		report.AcrossRuns.Runs = append(report.AcrossRuns.Runs, domain.RunSummary{
			Run: domain.NamedRun(fmt.Sprintf("run-%02d", i)), Totals: navCents(int64(i)),
		})
	}
	m := onRunsList(report)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	m = send(asModel(updated), screens.KeyAcrossRuns)
	assertOn(t, m, screens.ScreenAcrossRuns)

	view := m.View()
	if n := strings.Count(view, "\n") + 1; n > 8 {
		t.Errorf("view has %d lines on an 8-line terminal\n%s", n, view)
	}
	if strings.Contains(view, "run-30") {
		t.Errorf("run-30 must be scrolled out of view\n%s", view)
	}

	m = send(m, strings.Fields(strings.Repeat("down ", 150))...)
	if !strings.Contains(m.View(), "run-30") {
		t.Errorf("scrolling down must reach run-30\n%s", m.View())
	}
}
