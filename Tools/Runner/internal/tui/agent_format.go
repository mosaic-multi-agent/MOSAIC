package tui

import "mosaic-run/internal/agentfile"

// harnessAgentFormat returns the agent file data of harnessID. A harness
// without format data gets the zero Format: the orchestrator script is read as
// Markdown.
func harnessAgentFormat(harnessID string) agentfile.Format {
	f, _ := agentfile.For(harnessID)
	return f
}

// selectedAgentFormat returns the agent file data of the harness currently
// selected, or recorded in the run's settings.
func (m *rootModel) selectedAgentFormat() agentfile.Format {
	return harnessAgentFormat(m.selections.config.Harness)
}
