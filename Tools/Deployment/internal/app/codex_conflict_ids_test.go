package app

// codex_conflict_ids_test.go specifies that the workflow ids the conflict report takes from the
// workspace's deployed orchestrator files include those embedded in a deployed Codex
// orchestrator (TOML), as they do for a Markdown harness.

import (
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// orchestratorOnlyCatalog answers only the orchestrator lookups deployedWorkflowIDs makes.
type orchestratorOnlyCatalog struct {
	catalog.Catalog
	orch domain.Agent
}

func (c orchestratorOnlyCatalog) Orchestrator() domain.Agent { return c.orch }

func (orchestratorOnlyCatalog) OrchestratorScript() (domain.Agent, bool) {
	return domain.Agent{}, false
}

// writeCodexOrchestratorWithWorkflows writes a deployed Codex orchestrator whose
// developer_instructions hold the given workflow blocks.
func writeCodexOrchestratorWithWorkflows(t *testing.T, ws string, blocks ...string) {
	t.Helper()
	body := "<AvailableWorkflows type=\"managed\">\n"
	for _, b := range blocks {
		body += b
	}
	body += "</AvailableWorkflows>\n"
	doc, err := toml.Marshal(map[string]any{"sandbox_mode": "workspace-write", "developer_instructions": body})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	dir := filepath.Join(ws, ".codex", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orchestrator.toml"), append([]byte("# mosaic_version: 1.0\n\n"), doc...), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func TestDeployedWorkflowIDs_CodexOrchestrator_ReportsEmbeddedWorkflowIds(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	writeCodexOrchestratorWithWorkflows(t, ws,
		"<Workflow type=\"managed\" name=\"alpha-flow\" version=\"1.0\">\n## Alpha\nBODY\n</Workflow>\n",
		"<Workflow type=\"managed\" name=\"beta-flow\" version=\"2.0\">\n## Beta\nBODY\n</Workflow>\n")
	svc := &service{deps: Deps{
		Catalog: orchestratorOnlyCatalog{orch: domain.Agent{Key: "orchestrator", Role: domain.RoleOrchestrator}},
		GOOS:    "linux",
	}}

	// Act
	ids := svc.deployedWorkflowIDs(priorBytesCodexModule{}, ws, domain.ScopeProject, true)

	// Assert
	if len(ids) != 2 || ids[0] != "alpha-flow" || ids[1] != "beta-flow" {
		t.Errorf("deployedWorkflowIDs = %v, want [alpha-flow beta-flow] from the deployed Codex orchestrator", ids)
	}
}
