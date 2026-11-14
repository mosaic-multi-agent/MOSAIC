package app

// deployed_planned_path_probe.go decides what the deployed-state probe reports for the planned
// path of an id-bearing agent that no indexed file resolved to.

import (
	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
)

// stampsOutsideData reports whether src's agent format stores the provenance stamps outside
// its data model. A nil descriptor means Markdown; a format that does not resolve counts as
// having no such trait, because the decode at the same site reports the real problem.
func stampsOutsideData(src *domain.HarnessDescriptor) bool {
	id := ""
	if src != nil {
		id = src.AgentFormatID
	}
	traits, err := agentformat.TraitsOf(id)
	return err == nil && traits.StampsOutsideData
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
	if !stampsOutsideData(src) {
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
