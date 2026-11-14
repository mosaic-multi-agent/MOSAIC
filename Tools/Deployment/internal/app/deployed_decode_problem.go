package app

import (
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// markDecodeProblem marks state parse-failed when the deployed file at the workspace-relative
// targetPath cannot be read or decoded through the decode funnel. Stamps read from a file that
// cannot be decoded are not trustworthy, so they are cleared. A state that is not present, or a
// file that decodes cleanly, is returned unchanged.
func markDecodeProblem(
	state domain.DeployedArtifactState, workspace, targetPath string, src *domain.HarnessDescriptor,
) domain.DeployedArtifactState {
	if !state.Present {
		return state
	}
	read := readDeployedArtifact(filepath.Join(workspace, targetPath), domain.ArtifactAgent, src)
	problem := read.ReadErr
	if problem == nil {
		problem = read.DecodeErr
	}
	if problem == nil {
		return state
	}
	state.ParseFailed = true
	state.ParseProblem = problem.Error()
	state.Version = ""
	state.HarnessVersion = ""
	return state
}
