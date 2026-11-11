package app

// orchestrator_parse_check.go marks a deployed orchestrator file that cannot be read as an
// agent document, so a workflow update raises a conflict for it as the other flows do for any
// agent, instead of planning an ordinary update over it.

import (
	"os"
	"path/filepath"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// unreadableAgentProblem returns why the file at targetPath is no agent document (it does not
// parse or has no frontmatter), or "" when state shows no file at targetPath itself (absent, or
// read from a legacy file) or the file reads as an agent document.
func unreadableAgentProblem(workspace, targetPath string, state domain.DeployedArtifactState) string {
	if !state.Present || state.ParseFailed {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(targetPath)))
	if err != nil {
		return ""
	}
	doc, err := docformat.Parse(data)
	if err != nil {
		return err.Error()
	}
	if !doc.Frontmatter().Present() {
		return "no frontmatter"
	}
	return ""
}
