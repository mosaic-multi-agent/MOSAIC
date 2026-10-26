package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens/runflow"
)

const guidanceSavedStatusText = "Guidance saved - applies from the next dispatch"

// syncGuidanceAvailable tells the progress screen whether the session holds
// standing guidance right now, so its key and help bar follow the session.
func (m *rootModel) syncGuidanceAvailable() {
	if m.progressScreen == nil {
		return
	}
	_, available := m.guidance.Current()
	m.progressScreen.SetGuidanceAvailable(available)
}

// openGuidanceFromProgress acts on the progress screen's guidance key. The run
// keeps going while the editor is open; the progress screen and its rows stay
// as they are and come back when the editor closes.
func (m *rootModel) openGuidanceFromProgress() tea.Cmd {
	if !m.progressScreen.GuidanceViewRequested() {
		return nil
	}
	m.progressScreen.ClearGuidanceViewRequest()
	current, ok := m.guidance.Current()
	if !ok {
		return nil
	}
	m.guidanceScreen = runflow.NewGuidanceScreen(current, m.width, m.height, stylesFromTheme(m.theme))
	m.screen = screenGuidance
	return m.guidanceScreen.Init()
}

// updateGuidance routes a message to the guidance editor. A saved edit goes
// through the session's holder; a failed save keeps the editor open with the
// error.
func (m *rootModel) updateGuidance(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.guidanceScreen == nil {
		return m.closeGuidance()
	}
	cmd := m.guidanceScreen.Update(msg)
	if edited, saved := m.guidanceScreen.Saved(); saved {
		if err := m.guidance.Update(m.ctx, edited); err != nil {
			m.guidanceScreen.SetError(err.Error())
			return m, cmd
		}
		m.progressScreen.SetStatus(guidanceSavedStatusText, false)
		return m.closeGuidance()
	}
	if m.guidanceScreen.Closed() {
		return m.closeGuidance()
	}
	return m, cmd
}

// closeGuidance returns to the progress screen, then handles the session
// message that arrived while the editor was open, if any.
func (m *rootModel) closeGuidance() (tea.Model, tea.Cmd) {
	m.guidanceScreen = nil
	m.screen = screenProgress
	held := m.heldWhileEditing
	m.heldWhileEditing = nil
	if held != nil {
		return m.Update(held)
	}
	return m, nil
}

// holdWhileEditing keeps a message that must not replace the open guidance
// editor, and reports whether it did.
func (m *rootModel) holdWhileEditing(msg tea.Msg) bool {
	if m.screen != screenGuidance {
		return false
	}
	m.heldWhileEditing = msg
	return true
}
