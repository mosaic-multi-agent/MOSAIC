package tui

// settingsedit.go drives marking a tool as user-interaction from the Tools
// view: the save and recompute run as an async command and the outcome is shown
// as a one-line status on the Tools view.

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/tui/screens"
)

// settingsEditDoneMsg is sent when the app-layer mark-tool flow completes.
type settingsEditDoneMsg struct {
	result app.SettingsEditResult
	err    error
}

// markToolCmd returns the command for a mark request pending on the Tools view,
// clearing the request; nil when none is pending.
func (m Model) markToolCmd() tea.Cmd {
	tools, ok := m.runView.(*screens.ToolsScreen)
	if !ok {
		return nil
	}
	tool, harnesses, pending := tools.MarkRequested()
	if !pending {
		return nil
	}
	tools.ResetMark()
	svc, sess, report, ctx := m.opts.Service, m.session, m.report, m.ctx
	return func() tea.Msg {
		result, err := svc.MarkUserInteractionTool(ctx, sess, report, tool, harnesses)
		return settingsEditDoneMsg{result: result, err: err}
	}
}

// handleSettingsEditDone applies the outcome of a mark-tool command: on success
// the recomputed report replaces the model's report, the runs list is refreshed
// and the Tools view is reopened from it; on failure the report is kept.
func (m Model) handleSettingsEditDone(done settingsEditDoneMsg) (tea.Model, tea.Cmd) {
	if done.err != nil {
		m = m.setStatus("Could not save settings: "+done.err.Error(), tuicommon.RoleSignalBad)
		return m, nil
	}
	m.report = done.result.Report
	if m.runsScreen != nil {
		m.runsScreen.SetReport(m.report)
	}
	if m.screen == screenTools {
		m = m.openRunView(m.viewRunID, screens.ScreenTools)
	}
	m = m.setStatus("Marked "+done.result.Tool+" as user-interaction in "+done.result.ConfigPath, tuicommon.RoleSignalGood)
	return m, nil
}
