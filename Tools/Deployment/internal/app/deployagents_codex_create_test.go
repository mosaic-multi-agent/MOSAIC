package app_test

// deployagents_codex_create_test.go specifies that Deploy agents creates an agent in a fresh
// workspace (no agent file deployed yet) for a Markdown harness and for Codex alike: the
// agent file is written and the manifest records an entry for it. It runs the real planner
// and executor, because only the real planner derives the create action from an empty
// workspace.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestDeployAgents_FreshWorkspace_WritesAgentFileAndManifestEntry(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			// Arrange
			spy := &owSpy{}
			ws := t.TempDir()
			flow := owFlowNamed(t, "DeployAgents")

			// Act
			_, err := flow.run(t, owService(t, spy), h, ws, owFlowOpts{})

			// Assert
			if err != nil {
				t.Fatalf("DeployAgents in a fresh %s workspace: %v", h.name, err)
			}
			if got := owPlanItem(t, spy.lastPlan(t), owRunner).Action; got != domain.ActionCreate {
				t.Fatalf("action of %s = %s; want %s in a fresh workspace", owRunner, got, domain.ActionCreate)
			}
			if _, statErr := os.Stat(filepath.Join(ws, filepath.FromSlash(h.rel(owRunner)))); statErr != nil {
				t.Errorf("agent file %s was not written: %v", h.rel(owRunner), statErr)
			}
			entry := owAgentEntry(t, h, ws, owRunner)
			if entry.Owned == nil {
				t.Errorf("manifest entry of %s carries no Owned record", owRunner)
			}
		})
	}
}
