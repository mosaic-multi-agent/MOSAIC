package app

// orchestrator_parse_check.go marks a deployed orchestrator file that cannot be read as an
// agent document, so a workflow update raises a conflict for it as the other flows do for any
// agent, instead of planning an ordinary update over it.

import (
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// unreadableAgentProblem returns why the file at targetPath is no agent document (it cannot be
// read or decoded in the harness's agent format, does not parse, or has no frontmatter), or ""
// when state shows no file at targetPath itself (absent, or read from a legacy file) or the
// file reads as an agent document. The file is read through the decode funnel, so a
// non-Markdown agent format (such as Codex TOML) is judged in its canonical form.
func unreadableAgentProblem(workspace, targetPath string, state domain.DeployedArtifactState, src *domain.HarnessDescriptor) string {
	if !state.Present || state.ParseFailed {
		return ""
	}
	read := readDeployedArtifact(filepath.Join(workspace, filepath.FromSlash(targetPath)), domain.ArtifactAgent, src)
	if !read.Present || read.ReadErr != nil {
		return ""
	}
	if read.DecodeErr != nil {
		return read.DecodeErr.Error()
	}
	doc, err := ParseGenericSource(read.Canonical)
	if err != nil {
		return err.Error()
	}
	if !doc.Frontmatter().Present() {
		return "no frontmatter"
	}
	return ""
}
