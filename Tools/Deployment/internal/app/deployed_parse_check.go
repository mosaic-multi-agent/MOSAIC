package app

import (
	"path/filepath"

	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/domain"
)

// probePlannedPathForAgent probes the planned target path of an id-bearing agent that has no
// index match. A file there that cannot be parsed cleanly (parse error, no frontmatter, or no
// id) is reported present and parse-failed so the planner raises a conflict instead of
// planning a create that would overwrite it. A cleanly parsed file (with a different id) or an
// absent file yields Present: false.
func probePlannedPathForAgent(workspace, targetPath, modelKey string, scanFlagged bool, src *domain.HarnessDescriptor) domain.DeployedArtifactState {
	state := probeDeployedArtifact(workspace, targetPath, modelKey, domain.ArtifactAgent, src)
	if !state.Present {
		return domain.DeployedArtifactState{Present: false}
	}

	problem := plannedPathParseProblem(filepath.Join(workspace, targetPath), src)
	if problem == "" && !scanFlagged {
		return domain.DeployedArtifactState{Present: false}
	}

	state.ParseFailed = true
	state.ParseProblem = problem
	if problem != "" {
		// Stamps read from a file that is not a clean agent document are not trustworthy.
		state.Version = ""
		state.HarnessVersion = ""
	}
	return state
}

// plannedPathParseProblem returns a description of why the file at fullPath cannot be used as a
// deployed agent, or "" when it parses cleanly and carries an id.
func plannedPathParseProblem(fullPath string, src *domain.HarnessDescriptor) string {
	read := readDeployedArtifact(fullPath, domain.ArtifactAgent, src)
	if read.ReadErr != nil {
		return read.ReadErr.Error()
	}
	if read.DecodeErr != nil {
		return read.DecodeErr.Error()
	}
	doc, err := ParseGenericSource(read.Canonical)
	if err != nil {
		return err.Error()
	}
	fm := doc.Frontmatter()
	if !fm.Present() {
		return "no frontmatter"
	}
	idField, _ := agentfields.ByGeneric("id")
	for _, key := range agentfields.ReadOrder(idField) {
		if v, ok := fm.Get(key); ok && v.Kind == domain.KindScalar && v.Scalar != "" {
			return ""
		}
	}
	return "frontmatter has no id"
}
