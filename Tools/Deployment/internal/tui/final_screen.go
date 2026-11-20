package tui

// final_screen.go holds how the root model handles input on the final screen of a run
// (error, cancelled, transform cancelled, run summary, promote summary, transform summary):
// a key returns to the first screen with the whole session state of the finished run cleared,
// so a further run can be configured and executed in the same session.

import (
	tea "github.com/charmbracelet/bubbletea"
)

// finalScreenHelp is the help line of the final screens that do not scroll.
const finalScreenHelp = "any key: back to start  ctrl+c: quit"

// updateDone handles a message on the final screen. A key on a result screen is first offered
// to that screen (the run summary scrolls on its scroll keys); once the screen reports it is
// dismissed, or on the bare done views, a key restarts the session.
func (m *rootModel) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch {
	case m.promoteScreen != nil:
		if m.promoteScreen.Update(msg) {
			m.restartFromFinalScreen()
		}
	case m.transformScreen != nil:
		if m.transformScreen.Update(msg) {
			m.restartFromFinalScreen()
		}
	case m.summaryScreen != nil:
		if m.summaryScreen.Update(msg) {
			m.restartFromFinalScreen()
		}
	default:
		if _, ok := msg.(tea.KeyMsg); ok {
			m.restartFromFinalScreen()
		}
	}
	return m, nil
}

// restartFromFinalScreen returns to the first screen and clears everything the finished run
// left behind. The first screen is the harness screen, or the mode screen when the session
// was launched with a preset harness (which is kept).
func (m *rootModel) restartFromFinalScreen() {
	m.screen = screenHarness
	m.selections = entrySelections{}
	if m.presetHarness != "" {
		m.screen = screenMode
		m.selections.harnessID = m.presetHarness
	}
	m.errMsg = ""
	m.statusMsg = ""
	m.runCancelled = false
	m.summaryScreen = nil
	m.promoteScreen = nil
	m.transformScreen = nil
	m.stack.key = "" // the next stack screen restarts from its path's stack
	m.clearOverlays()
}

// clearOverlays drops the active question and every question overlay.
func (m *rootModel) clearOverlays() {
	m.activeQuestion = nil
	m.selectOverlay = nil
	m.confirmOverlay = nil
	m.modelOverlay = nil
	m.textOverlay = nil
	m.conflictOverlay = nil
	m.reviewOverlay = nil
	m.workflowOverlay = nil
	m.agentOverlay = nil
	m.infraAgentOverlay = nil
	m.deployAgentOverlay = nil
	m.hookOverlay = nil
	m.standaloneOverlay = nil
	m.promoteAgentOverlay = nil
}
