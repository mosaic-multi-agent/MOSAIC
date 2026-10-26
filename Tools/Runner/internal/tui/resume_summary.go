package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/resumecheck"
	"mosaic-run/internal/tui/screens/runconfig"
)

// startOrSummarise ends the setup sequence. A new run starts at once; a resume
// first shows the settings summary, which is the last screen before the session
// starts on every harness.
func (m *rootModel) startOrSummarise() tea.Cmd {
	if m.selections.isNewRun {
		return m.launchSession()
	}
	state, err := m.readRunState()
	if err != nil {
		// The artifact vanished since run selection; the session refuses it.
		return m.launchSession()
	}
	if m.needsRunnerAdoption() {
		cfg := m.selections.config.Settings
		state.Mode = cfg.Mode
		state.PreConsultation = cfg.PreConsultation
		state.ManualResolution = cfg.ManualResolution
	}
	var agents []domain.DeclaredInfraAgent
	if path := m.selections.orchestratorFile; path != "" {
		agents, _ = orchfile.EnumerateInfrastructureAgents(path)
	}
	lines := resumecheck.NewSummary(state, agents).Lines()
	m.summaryScreen = runconfig.NewSummaryScreen(lines, m.width, m.height, stylesFromTheme(m.theme))
	m.screen = screenSetupSummary
	return nil
}

// backToConfig returns from a later setup screen to the configuration wizard.
// A resume keeps its answers, so the wizard is reopened on its last question.
func (m *rootModel) backToConfig() tea.Cmd {
	m.screen = screenSetupConfig
	if m.selections.isNewRun {
		return nil
	}
	return m.configScreen.Reopen()
}

func (m *rootModel) updateSetupSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.summaryScreen.Update(msg)
	if m.summaryScreen.Back() {
		m.summaryScreen.Reset()
		if m.selections.config.Harness == "ghcp-cli" {
			m.ghcpModeScreen.Reset()
			m.screen = screenSetupGHCPMode
			return m, nil
		}
		return m, m.backToConfig()
	}
	if m.summaryScreen.Confirmed() {
		m.summaryScreen.Reset()
		m.configScreen.Reset()
		return m, m.launchSession()
	}
	return m, nil
}
