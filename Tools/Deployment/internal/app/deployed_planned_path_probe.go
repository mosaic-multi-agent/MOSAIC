package app

// deployed_planned_path_probe.go decides what the deployed-state probe reports for the planned
// path of an id-bearing agent that no indexed file resolved to.

import (
	"mosaic-deploy/internal/domain"
)

// markdownAgentFormatID is the descriptor value of the Markdown identity agent format.
const markdownAgentFormatID = "markdown"

// isMarkdownAgentFormat reports whether src describes a Markdown agent format: a nil
// descriptor and an absent agent format id both mean Markdown.
func isMarkdownAgentFormat(src *domain.HarnessDescriptor) bool {
	return src == nil || src.AgentFormatID == "" || src.AgentFormatID == markdownAgentFormatID
}

// probeAgentAtPlannedPath probes the planned path of an id-bearing agent that has no index
// match. claimed is true when an indexed file of another agent sits at that path; scanFlagged
// is true when the workspace scan flagged the path as parse-failed.
//
// For a Markdown harness a file that cannot be parsed cleanly (or that the scan flagged) is
// reported present and parse-failed, and a cleanly parsed file with a different id yields
// Present: false (see probePlannedPathForAgent).
//
// For any other format a claimed path is never probed (it belongs to another agent), and an
// unclaimed path is probed as is: a file written without an id stamp (a first deploy, or a tool
// that omits the stamp) is reported present with its versions so the planner sees it as
// current, and is parse-failed only when the scan flagged it.
func probeAgentAtPlannedPath(
	workspace, targetPath, modelKey string, src *domain.HarnessDescriptor, claimed, scanFlagged bool,
) domain.DeployedArtifactState {
	if isMarkdownAgentFormat(src) {
		return probePlannedPathForAgent(workspace, targetPath, modelKey, scanFlagged, src)
	}
	if claimed {
		return domain.DeployedArtifactState{Present: false}
	}
	state := probeDeployedArtifact(workspace, targetPath, modelKey, domain.ArtifactAgent, src)
	state = markDecodeProblem(state, workspace, targetPath, src)
	if state.Present && scanFlagged {
		state.ParseFailed = true
	}
	return state
}
