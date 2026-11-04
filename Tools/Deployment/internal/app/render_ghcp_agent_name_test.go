package app_test

// render_ghcp_agent_name_test.go specifies that render resolves the agent destination of both
// GHCP targets to .github/agents/X.agent.md.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestRenderAgent_GhcpTargets_DestinationIsNamedAgentMd(t *testing.T) {
	for _, harnessID := range []string{hCLI, hVSCode} {
		t.Run(harnessID, func(t *testing.T) {
			deps := newRenderDeps(t)
			m := rbModule(t, harnessID)
			deps.Registry = &stubRegistry{
				list:    []domain.HarnessRef{m.Ref()},
				modules: map[string]domain.HarnessModule{harnessID: m},
			}
			src := writeRenderSourceFile(t, t.TempDir(), "worker.md", minimalGenericSubagentBytes())
			ws := t.TempDir()

			result, err := app.New(deps).RenderAgent(context.Background(), app.RenderAgentRequest{
				SourcePath:      src,
				TargetHarnessID: harnessID,
				WorkspaceRoot:   ws,
				TargetModel:     "render-model-x",
				DryRun:          true,
			})
			if err != nil {
				t.Fatalf("RenderAgent: %v", err)
			}

			if got, want := filepath.ToSlash(result.DestinationPath), ".github/agents/worker.agent.md"; !strings.HasSuffix(got, want) {
				t.Errorf("DestinationPath = %q, want it to end with %s", got, want)
			}
		})
	}
}
