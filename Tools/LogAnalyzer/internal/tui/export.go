package tui

// export.go drives the digest export action: opening the path entry from the
// runs and per-run screens, running the export as an async command, asking
// before overwriting, and reporting the outcome as a one-line status on the
// screen the user came from.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/tui/screens"
)

// screenExport is the path entry / overwrite confirmation screen.
const screenExport = screenName(screens.ScreenExport)

// exportDoneMsg is sent when Service.ExportSession completes.
type exportDoneMsg struct {
	result app.ExportResult
	err    error
}

// exportFlow is the state of the export action.
type exportFlow struct {
	screen *screens.ExportScreen
	origin screenName          // screen to return to
	runID  string              // "" = all runs
	path   string              // the path being exported to
	status string              // one-line outcome shown on the origin screen
	role   tuicommon.StyleRole // signal role the status is drawn in
}

// exportKeyHook handles export-related messages ahead of normal screen
// routing. It reports whether the message was consumed.
func (m Model) exportKeyHook(msg tea.Msg) (Model, tea.Cmd, bool) {
	key, isKey := msg.(tea.KeyMsg)
	if m.screen == screenExport {
		model, cmd := m.updateExport(msg)
		return model, cmd, true
	}
	if !isKey {
		return m, nil, false
	}
	m.export.status = ""
	if !m.exportOrigin() {
		return m, nil, false
	}
	switch key.String() {
	case screens.KeyExportRun:
		return m.startExport(false), nil, true
	case screens.KeyExportAll:
		return m.startExport(true), nil, true
	}
	return m, nil, false
}

// exportOrigin reports whether the current screen offers the export action.
func (m Model) exportOrigin() bool {
	switch m.screen {
	case screenRuns:
		return m.runsScreen == nil || !m.runsScreen.HelpOpen() // the open help overlay takes every key
	case screenRunDetail, screenOverview, screenAgentTypes, screenStatus, screenTools, screenTimeline, screenContext, screenSession:
		help, ok := m.runView.(interface{ HelpOpen() bool })
		return !ok || !help.HelpOpen() // the open help overlay takes every key
	}
	return false
}

// startExport opens the path entry for the selected / shown run or for all runs.
func (m Model) startExport(all bool) Model {
	if m.session == nil {
		return m
	}
	runID := ""
	if !all {
		runID = m.viewRunID
		if m.screen == screenRuns {
			runID = ""
			if m.runsScreen != nil {
				runID = m.runsScreen.SelectedRunID()
			}
		}
		if runID == "" {
			m = m.setStatus("No run selected", tuicommon.RoleSignalWarning)
			return m
		}
	}
	m.export.screen = screens.NewExportScreen(runID, m.width, stylesFromTheme(m.theme))
	m.export.origin = m.screen
	m.export.runID = runID
	m.screen = screenExport
	return m
}

// updateExport routes a message to the export screen and acts on its request.
func (m Model) updateExport(msg tea.Msg) (Model, tea.Cmd) {
	sc := m.export.screen
	if sc == nil {
		m.screen = m.export.origin
		return m, nil
	}
	_, cmd := sc.Update(msg)
	switch sc.Action() {
	case screens.ExportCancel:
		return m.finishExport("Export cancelled", tuicommon.RoleSignalWarning), nil
	case screens.ExportDecline:
		return m.finishExport("Export cancelled; nothing written", tuicommon.RoleSignalWarning), nil
	case screens.ExportSubmit:
		m.export.path = sc.Path()
		sc.Reset()
		m.screen = screenLoading
		return m, m.buildExportCmd(false)
	case screens.ExportOverwrite:
		sc.Reset()
		m.screen = screenLoading
		return m, m.buildExportCmd(true)
	}
	return m, cmd
}

// finishExport returns to the screen the export was started from with a status.
func (m Model) finishExport(status string, role tuicommon.StyleRole) Model {
	m = m.setStatus(status, role)
	m.export.screen = nil
	m.screen = m.export.origin
	return m
}

// buildExportCmd returns a Cmd that runs Service.ExportSession in a goroutine
// under m.ctx and returns only an exportDoneMsg.
func (m Model) buildExportCmd(overwrite bool) tea.Cmd {
	svc := m.opts.Service
	sess := m.session
	ctx := m.ctx
	req := app.ExportRequest{RunID: m.export.runID, OutPath: m.export.path, Overwrite: overwrite}
	return func() tea.Msg {
		res, err := svc.ExportSession(ctx, sess, req)
		return exportDoneMsg{result: res, err: err}
	}
}

// handleExportDone reports the outcome of a finished export.
func (m Model) handleExportDone(done exportDoneMsg) (tea.Model, tea.Cmd) {
	if done.err != nil {
		return m.finishExport("Export failed: "+done.err.Error(), tuicommon.RoleSignalBad), nil
	}
	res := done.result
	switch res.Status {
	case app.ExportWritten:
		msg := fmt.Sprintf("Exported %d run(s) to %s", res.RunsWritten, res.OutPath)
		if res.UnattributableWritten {
			msg += " (including the unattributable bucket)"
		}
		return m.finishExport(msg, tuicommon.RoleSignalGood), nil
	case app.ExportFileExists:
		if m.export.screen == nil {
			return m.finishExport("Export failed: file already exists", tuicommon.RoleSignalBad), nil
		}
		m.export.screen.AskOverwrite()
		m.screen = screenExport
		return m, nil
	case app.ExportRunNotFound:
		return m.finishExport("Export failed: run not found", tuicommon.RoleSignalBad), nil
	case app.ExportInvalidPath:
		return m.finishExport("Export failed: invalid path", tuicommon.RoleSignalBad), nil
	}
	return m.finishExport("Export failed: nothing to export", tuicommon.RoleSignalBad), nil
}

// viewExport renders the export screen.
func (m Model) viewExport() string {
	if m.export.screen == nil {
		return ""
	}
	return m.export.screen.View(m.width, m.height)
}

// setStatus sets the one-line status shown below the screen, drawn in role.
func (m Model) setStatus(status string, role tuicommon.StyleRole) Model {
	m.export.status, m.export.role = status, role
	return m
}

// withExportStatus renders a screen with room for the export status line
// below it, when there is a status to show. The status is cut to the width and
// drawn in its signal role.
func (m Model) withExportStatus(render func(height int) string) string {
	if m.export.status == "" || m.height < 2 {
		return render(m.height)
	}
	theme := stylesFromTheme(m.theme).Theme
	status := theme.Style(m.export.role).Render(textwidth.Truncate(m.export.status, m.width))
	return strings.Join([]string{render(m.height - 1), status}, "\n")
}
