package tui

// The model hands each opened per-run view the findings of its run and routes
// the help overlay keys to it.

import (
	"strings"
	"testing"

	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func reportWithFindings() domain.Report {
	report := navViewsReport(navViewsRun(runA, 1000), navViewsRun(runB, 500), navViewsRun("20260303T120000Z-cccc", 100))
	report.Quality = domain.NewQualitySummary([]domain.Finding{
		{Kind: domain.FindingMalformedLine, Run: domain.NamedRun(runA), Path: "a1"},
		{Kind: domain.FindingMissingTokenUsage, Run: domain.NamedRun(runA), Path: "a2"},
		{Kind: domain.FindingMissingModel, Run: domain.NamedRun(runB), Path: "b1"},
	})
	return report
}

func headerOf(m Model) string {
	lines := frameLines(m.View())
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func TestFrameWiring_HeaderShowsTheFindingsOfTheOpenedRunInEveryView(t *testing.T) {
	m := onRunDetail(t, reportWithFindings())
	if h := headerOf(m); !strings.Contains(h, "2 findings") {
		t.Errorf("run detail header must show %q: %q", "2 findings", h)
	}
	for _, key := range []string{
		screens.KeyViewOverview, screens.KeyViewAgentTypes, screens.KeyViewStatus,
		screens.KeyViewTools, screens.KeyViewTimeline, screens.KeyViewDispatches,
	} {
		m = send(m, key)

		if h := headerOf(m); !strings.Contains(h, "2 findings") {
			t.Errorf("view %q: header must still show 2 findings after switching: %q", key, h)
		}
	}
}

func TestFrameWiring_HeaderCountsOnlyTheOpenedRunsFindings(t *testing.T) {
	m := onRunsList(reportWithFindings())
	m = send(m, "down", "enter") // second run: one finding

	assertOn(t, m, screens.ScreenRunDetail)
	if h := headerOf(m); !strings.Contains(h, "1 findings") && !strings.Contains(h, "1 finding") {
		t.Errorf("second run's header must show its single finding: %q", h)
	}
	if h := headerOf(m); strings.Contains(h, "2 findings") || strings.Contains(h, "3 findings") {
		t.Errorf("findings of other runs must not be counted: %q", h)
	}
}

func TestFrameWiring_RunWithoutFindingsShowsNoFindingBadge(t *testing.T) {
	m := onRunsList(reportWithFindings())
	m = send(m, "down", "down", "enter")

	assertOn(t, m, screens.ScreenRunDetail)
	if h := headerOf(m); strings.Contains(strings.ToLower(h), "finding") {
		t.Errorf("a run without findings must show no finding badge: %q", h)
	}
}

func TestFrameWiring_QuestionMarkOpensTheOverlayAndEscClosesItBeforeLeaving(t *testing.T) {
	m := onRunDetail(t, reportWithFindings())
	before := m.View()

	m = send(m, screens.KeyHelp)
	assertOn(t, m, screens.ScreenRunDetail)
	if m.View() == before {
		t.Fatal("? must open the help overlay")
	}

	m = send(m, "q")
	assertOn(t, m, screens.ScreenRunDetail)
	m = send(m, "esc")
	assertOn(t, m, screens.ScreenRunDetail)
	if m.View() != before {
		t.Errorf("esc must close the overlay and restore the view\n%s\nvs\n%s", m.View(), before)
	}

	m = send(m, "esc")
	assertOn(t, m, screens.ScreenRuns)
}

func TestFrameWiring_FramedViewsStayWithinTheTerminal(t *testing.T) {
	m := onRunDetail(t, reportWithFindings())
	for _, key := range []string{
		screens.KeyViewOverview, screens.KeyViewAgentTypes, screens.KeyViewStatus,
		screens.KeyViewTools, screens.KeyViewTimeline, screens.KeyViewDispatches,
	} {
		m = send(m, key)
		for _, open := range []bool{false, true} {
			if open {
				m = send(m, screens.KeyHelp)
			}
			lines := frameLines(m.View())
			if len(lines) > m.height {
				t.Errorf("view %q (overlay %v): %d lines on a %d-line terminal", key, open, len(lines), m.height)
			}
			for _, l := range lines {
				if n := textwidth.Of(l); n > m.width {
					t.Errorf("view %q (overlay %v): line of %d runes on a %d-column terminal: %q", key, open, n, m.width, l)
				}
			}
		}
		m = send(m, screens.KeyHelp)
	}
}
