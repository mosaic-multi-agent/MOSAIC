package session

import "mosaic-run/internal/agentfile"

// agentFormat returns the agent file data of the run's harness. A harness
// without format data gets the zero Format: legacy Markdown resolution.
func agentFormat(harnessID string) agentfile.Format {
	f, _ := agentfile.For(harnessID)
	return f
}
