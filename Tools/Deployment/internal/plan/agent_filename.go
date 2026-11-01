package plan

import (
	"path/filepath"

	"mosaic-deploy/internal/agentkey"
	"mosaic-deploy/internal/domain"
)

// ghcpHarnessIDs are the harnesses whose deployed agent files carry the extension their
// descriptor declares instead of the catalog source file's own name.
var ghcpHarnessIDs = map[string]bool{"ghcp-cli": true, "vscode-ghcp": true}

// AgentFileName returns the file name the deploy flows give an agent in the harness's agents
// folder. The GHCP harnesses name it after the agent key plus the descriptor's agent extension
// (X.agent.md); every other harness keeps the catalog source file's base name.
func AgentFileName(module domain.HarnessModule, agent domain.Agent) string {
	base := filepath.Base(agent.SourcePath)
	if !ghcpHarnessIDs[module.Ref().ID] {
		return base
	}
	ext := module.Descriptor().Extensions[domain.ArtifactAgent]
	if ext == "" {
		return base
	}
	return agentkey.FromFileName(base) + ext
}
