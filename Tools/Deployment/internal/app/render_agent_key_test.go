package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
)

// renderKeyFor renders a generic subagent saved under fileName into a workspace and returns
// the render result.
func renderKeyFor(t *testing.T, fileName string) app.RenderAgentResult {
	t.Helper()
	svc := app.New(newRenderDeps(t))
	src := writeRenderSourceFile(t, t.TempDir(), fileName, minimalGenericSubagentBytes())

	result, err := svc.RenderAgent(context.Background(), app.RenderAgentRequest{
		SourcePath:      src,
		TargetHarnessID: renderHarness.ID,
		WorkspaceRoot:   t.TempDir(),
		TargetModel:     "render-model-x",
		DryRun:          true,
	})
	if err != nil {
		t.Fatalf("RenderAgent(%s) returned error: %v", fileName, err)
	}
	return result
}

func TestRenderAgent_SourceAgentMdSuffix_KeyDropsWholeSuffix(t *testing.T) {
	result := renderKeyFor(t, "x.agent.md")

	if result.AgentKey != "x" {
		t.Errorf("AgentKey = %q, want %q", result.AgentKey, "x")
	}
}

func TestRenderAgent_SourceAgentMdSuffix_DestinationNamedFromKey(t *testing.T) {
	result := renderKeyFor(t, "x.agent.md")

	if got, want := filepath.Base(result.DestinationPath), "x.md"; got != want {
		t.Errorf("destination file name = %q, want %q", got, want)
	}
}

func TestRenderAgent_SourcePlainMd_KeyUnchanged(t *testing.T) {
	result := renderKeyFor(t, "x.md")

	if result.AgentKey != "x" {
		t.Errorf("AgentKey = %q, want %q", result.AgentKey, "x")
	}
	if got, want := filepath.Base(result.DestinationPath), "x.md"; got != want {
		t.Errorf("destination file name = %q, want %q", got, want)
	}
}

// A source file name that ends in neither ".agent.md" nor ".md" has nothing to strip, so the
// key is the whole file name (the shared derivation does not strip arbitrary extensions).
func TestRenderAgent_SourceNonMdExtension_KeyIsWholeFileName(t *testing.T) {
	result := renderKeyFor(t, "x.txt")

	if result.AgentKey != "x.txt" {
		t.Errorf("AgentKey = %q, want %q", result.AgentKey, "x.txt")
	}
}

func TestRenderAgent_SourceAgentInMiddleOfName_KeyKeepsIt(t *testing.T) {
	result := renderKeyFor(t, "my.agent.notes.md")

	if result.AgentKey != "my.agent.notes" {
		t.Errorf("AgentKey = %q, want %q", result.AgentKey, "my.agent.notes")
	}
}
