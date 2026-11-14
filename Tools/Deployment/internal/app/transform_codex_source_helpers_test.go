package app_test

// transform_codex_source_helpers_test.go holds the shared builders of the Transform-from-Codex
// tests: Codex TOML source agent files written into a workspace's Codex agents folder, a run
// wired with the real Codex module as the SOURCE harness, and readers of the written
// Markdown targets. Nothing here reads the repository Catalog.

import (
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/app"
)

const (
	// cxSourceModelSub and cxSourceModelOrch are the models the Codex source fixtures declare.
	cxSourceModelSub  = "gpt-5.6-sol"
	cxSourceModelOrch = "gpt-5.6-terra"

	// cxTargetModelSub and cxTargetModelOrch are the pre-answered target models.
	cxTargetModelSub  = "claude-sonnet-4-5"
	cxTargetModelOrch = "claude-opus-4-5"

	// cxBodyText is the instruction text of the Codex source fixtures.
	cxBodyText = "CODEX-SOURCE-BODY-TEXT"
)

// cxOpts shapes one Codex source agent file.
type cxOpts struct {
	name  string
	model string // empty: no model key
	role  string // empty: no mosaic_role stamp
	body  string // empty: the default identity body
}

// codexSourceAgent returns a stamped Codex TOML agent file: stamp comment block, owned keys
// and the canonical body as developer_instructions.
func codexSourceAgent(o cxOpts) string {
	body := o.body
	if body == "" {
		body = "<Identity type=\"core\">\nYou are the " + o.name + " agent. " + cxBodyText + "\n</Identity>\n"
	}
	var b strings.Builder
	b.WriteString("# mosaic_version: 2.3.0\n# mosaic_harness_version: 1.2.0\n")
	if o.role != "" {
		b.WriteString("# mosaic_role: " + o.role + "\n")
	}
	b.WriteString("\nname = \"" + o.name + "\"\n")
	b.WriteString("description = \"Codex source agent " + o.name + "\"\n")
	if o.model != "" {
		b.WriteString("model = \"" + o.model + "\"\n")
	}
	b.WriteString("sandbox_mode = \"read-only\"\n")
	b.WriteString("developer_instructions = \"\"\"\n" + body + "\"\"\"\n")
	return b.String()
}

// newCodexSourceRun wires a run from the Codex harness to the Claude Code harness.
func newCodexSourceRun(t *testing.T) *trRun {
	t.Helper()
	return trNew(t, nil, hCodex, hClaude, withCodexTarget(t))
}

// cxPut writes a file of the given name (with extension) into the Codex agents folder of the
// run's workspace and returns its workspace-relative path.
func cxPut(t *testing.T, r *trRun, fileName, content string) string {
	t.Helper()
	rel := codexAppModule(t).Descriptor().Paths.Agents.Project + "/" + fileName
	trWrite(t, r.ws, rel, content)
	return rel
}

// cxRequest is the pre-answered request of a Codex-source run: one target model per source
// model, so a model read from the decoded source selects the answer.
func cxRequest(r *trRun) app.TransformHarnessRequest {
	req := r.request()
	req.ModelMap = app.TransformModelMapping{
		cxSourceModelSub:  cxTargetModelSub,
		cxSourceModelOrch: cxTargetModelOrch,
	}
	return req
}

// cxParse parses a written Markdown target file.
func cxParse(t *testing.T, content string) *docformat.Document {
	t.Helper()
	doc, err := docformat.Parse([]byte(content))
	if err != nil {
		t.Fatalf("parse written target: %v\n%s", err, content)
	}
	return doc
}

// cxFrontmatterScalar returns a frontmatter scalar of a parsed target and whether it exists.
func cxFrontmatterScalar(doc *docformat.Document, key string) (string, bool) {
	v, ok := doc.Frontmatter().Get(key)
	return v.Scalar, ok
}
