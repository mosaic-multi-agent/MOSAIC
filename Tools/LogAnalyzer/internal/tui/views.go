package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// The per-run views share the model's screen name with their screens.ScreenID.
const (
	screenOverview   screenName = screenName(screens.ScreenOverview)
	screenAgentTypes screenName = screenName(screens.ScreenAgentTypes)
	screenStatus     screenName = screenName(screens.ScreenStatus)
	screenTools      screenName = screenName(screens.ScreenTools)
	screenTimeline   screenName = screenName(screens.ScreenTimeline)
	screenAcrossRuns screenName = screenName(screens.ScreenAcrossRuns)
	screenContext    screenName = screenName(screens.ScreenContext)
	screenCompare    screenName = screenName(screens.ScreenCompare)
)

// openRunView shows the per-run view id for the run with the given id, built
// from the model's current report so it reflects the latest pricing.
func (m Model) openRunView(runID string, id screens.ScreenID) Model {
	run, ok := m.report.FindRun(runID)
	if !ok {
		return m
	}
	style := m.screenStyles()
	switch id {
	case screens.ScreenOverview:
		m.runView = screens.NewOverviewScreen(run, m.width, m.height, style)
	case screens.ScreenAgentTypes:
		m.runView = screens.NewAgentTypesScreen(run, m.width, m.height, style)
	case screens.ScreenStatus:
		m.runView = screens.NewStatusScreen(run, m.width, m.height, style)
	case screens.ScreenTools:
		m.runView = screens.NewToolsScreen(run, m.width, m.height, style)
	case screens.ScreenContext:
		m.runView = screens.NewContextScreen(run, m.width, m.height, style)
	case screens.ScreenTimeline:
		m.runView = screens.NewTimelineScreen(run, m.width, m.height, style)
	default:
		m.runDetailScreen = screens.NewRunDetailScreen(run, m.width, m.height, style)
		m.runView = m.runDetailScreen
	}
	if q, ok := m.runView.(interface{ SetRunQuality(screens.RunQuality) }); ok {
		q.SetRunQuality(screens.RunQualityOf(m.report.Quality, run.Run))
	}
	if r, ok := m.runView.(interface{ SetUnpricedReasons([]domain.UnpricedReason) }); ok {
		r.SetUnpricedReasons(m.report.UnpricedReasons)
	}
	m.viewRunID = runID
	m.screen = screenName(id)
	return m
}

// updateRunView routes a message to the active per-run view and applies the
// back and view-switch requests it reports.
func (m Model) updateRunView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.runView == nil {
		return m, nil
	}
	updated, cmd := m.runView.Update(msg)
	m.runView = updated.(screens.RunView)
	if m.runView.Back() {
		m.runView.Reset()
		m.screen = screenRuns
		return m, nil
	}
	if id, ok := m.runView.Requested(); ok {
		m.runView.Reset()
		return m.openRunView(m.viewRunID, id), nil
	}
	if markCmd := m.markToolCmd(); markCmd != nil {
		return m, markCmd
	}
	return m, cmd
}

// viewRunView renders the active per-run view.
func (m Model) viewRunView() string {
	if m.runView == nil {
		return ""
	}
	return m.withExportStatus(func(h int) string { return m.runView.View(m.width, h) })
}

// openAcrossRuns shows the across-runs view built from the model's current report.
func (m Model) openAcrossRuns() Model {
	m.runView = screens.NewAcrossRunsScreen(m.report, m.width, m.height, m.screenStyles())
	m.screen = screenAcrossRuns
	return m
}

// openCompare shows the comparison of run idA (A) with run idB (B), computed from
// the model's current report; the screen stays where it is when either run is gone.
func (m Model) openCompare(idA, idB string) Model {
	cmp, ok := app.CompareRuns(m.report, idA, idB)
	if !ok {
		return m
	}
	m.runView = screens.NewCompareScreen(cmp, m.width, m.height, m.screenStyles())
	m.screen = screenCompare
	return m
}
