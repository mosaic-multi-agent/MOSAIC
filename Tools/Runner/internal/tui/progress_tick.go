package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens/runflow"
)

// handleProgressTick delivers a progress tick to the progress screen whichever
// screen is current, so the one-second chain never ends while an overlay is
// open. It also refreshes the pending-request display from the shared state,
// which the session changes without sending a message.
func (m *rootModel) handleProgressTick(msg runflow.TickMsg) (tea.Model, tea.Cmd) {
	if m.progressScreen == nil {
		m.tickChainRunning = false
		return m, nil
	}
	m.tickChainRunning = true
	m.syncPendingRequests()
	return m, m.progressScreen.Update(msg)
}

// startProgressTicks returns the progress screen's tick command, or nil when a
// chain is already running, so a restart never doubles the ticking.
func (m *rootModel) startProgressTicks() tea.Cmd {
	if m.tickChainRunning {
		return nil
	}
	return m.progressScreen.Init()
}
