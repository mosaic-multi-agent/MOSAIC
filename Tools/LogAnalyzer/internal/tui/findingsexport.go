package tui

// findingsexport.go drives the findings export: key e inside an open findings
// overlay opens the path entry, the write runs as an async command, an existing
// file is only replaced after confirmation, and the outcome is a one-line status
// on the screen the user came from, whose overlay stays open.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/tui/screens"
)

// findingsExportDoneMsg is sent when Service.ExportFindings completes.
type findingsExportDoneMsg struct {
	result app.FindingsExportResult
	err    error
}

// findingsExportFlow is the state of a findings export in progress.
type findingsExportFlow struct {
	scope screens.FindingsScope // what the overlay counts and the file holds
	path  string                // the path being exported to
}

// scopedOverlay is a screen whose findings overlay can be exported.
type scopedOverlay interface {
	FindingsOpen() bool
	FindingsScope() screens.FindingsScope
}

// openOverlayScope returns the scope of the findings overlay that is open on the
// current screen.
func (m Model) openOverlayScope() (screens.FindingsScope, bool) {
	var view scopedOverlay
	switch m.screen {
	case screenRuns:
		if m.runsScreen == nil {
			return screens.FindingsScope{}, false
		}
		view = m.runsScreen
	case screenRunDetail, screenOverview, screenAgentTypes, screenStatus, screenTools, screenTimeline, screenContext, screenSession:
		var ok bool
		if view, ok = m.runView.(scopedOverlay); !ok {
			return screens.FindingsScope{}, false
		}
	default:
		return screens.FindingsScope{}, false
	}
	if !view.FindingsOpen() {
		return screens.FindingsScope{}, false
	}
	return view.FindingsScope(), true
}

// startFindingsExport opens the findings path entry for the overlay's scope.
func (m Model) startFindingsExport(scope screens.FindingsScope) Model {
	m.export.screen = screens.NewFindingsExportScreen(scope.Label, m.width, stylesFromTheme(m.theme))
	m.export.origin = m.screen
	m.export.findings = &findingsExportFlow{scope: scope}
	m.screen = screenExport
	return m
}

// updateFindingsExport routes a message to the findings prompt and acts on its request.
func (m Model) updateFindingsExport(msg tea.Msg) (Model, tea.Cmd) {
	sc, flow := m.export.screen, m.export.findings
	_, cmd := sc.Update(msg)
	switch sc.Action() {
	case screens.ExportCancel:
		return m.finishFindingsExport("Export cancelled", tuicommon.RoleSignalWarning), nil
	case screens.ExportDecline:
		return m.finishFindingsExport("Export cancelled; nothing written", tuicommon.RoleSignalWarning), nil
	case screens.ExportSubmit:
		flow.path = sc.Path()
		sc.Reset()
		m.screen = screenLoading
		return m, m.buildFindingsExportCmd(false)
	case screens.ExportOverwrite:
		sc.Reset()
		m.screen = screenLoading
		return m, m.buildFindingsExportCmd(true)
	}
	return m, cmd
}

// finishFindingsExport ends the flow and returns to the origin screen with a status.
func (m Model) finishFindingsExport(status string, role tuicommon.StyleRole) Model {
	m.export.findings = nil
	return m.finishExport(status, role)
}

// buildFindingsExportCmd returns a Cmd that runs Service.ExportFindings in a
// goroutine under m.ctx and returns only a findingsExportDoneMsg.
func (m Model) buildFindingsExportCmd(overwrite bool) tea.Cmd {
	svc, ctx, flow := m.opts.Service, m.ctx, m.export.findings
	req := app.FindingsExportRequest{Findings: flow.scope.Findings, OutPath: flow.path, Overwrite: overwrite}
	return func() tea.Msg {
		res, err := svc.ExportFindings(ctx, req)
		return findingsExportDoneMsg{result: res, err: err}
	}
}

// handleFindingsExportDone reports the outcome of a finished findings export.
func (m Model) handleFindingsExportDone(done findingsExportDoneMsg) (tea.Model, tea.Cmd) {
	if done.err != nil {
		return m.finishFindingsExport("Export failed: "+done.err.Error(), tuicommon.RoleSignalBad), nil
	}
	res := done.result
	switch res.Status {
	case app.FindingsExportWritten:
		return m.finishFindingsExport(fmt.Sprintf("Exported %d finding(s) to %s", res.Findings, res.OutPath), tuicommon.RoleSignalGood), nil
	case app.FindingsExportFileExists:
		if m.export.screen == nil {
			return m.finishFindingsExport("Export failed: file already exists", tuicommon.RoleSignalBad), nil
		}
		m.export.screen.AskOverwrite()
		m.screen = screenExport
		return m, nil
	}
	return m.finishFindingsExport("Export failed: invalid path", tuicommon.RoleSignalBad), nil
}
