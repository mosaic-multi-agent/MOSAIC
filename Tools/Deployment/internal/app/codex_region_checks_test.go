package app

// codex_region_checks_test.go is regression coverage that the orchestrator-region facts and
// the region checks give the same result for a Codex harness as for a Markdown harness given
// equivalent content:
//
//   - the deployed orchestrator injection version and region presence recognised by the
//     deployed-state probe,
//   - the role-region check of a harness-only agent read from a deployed Codex file,
//   - the orchestrator-region-undeliverable decision for a catalog source.

import (
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

const codexRegionBody = "<Identity type=\"core\">\nAgent body.\n</Identity>\n\n" +
	"<OrchestratorHarnessConstraints type=\"managed\" version=\"1.2.1\">\norchestrator text\n" +
	"</OrchestratorHarnessConstraints>\n"

// writeCodexRegionAgent writes a Codex TOML agent whose developer_instructions hold body.
func writeCodexRegionAgent(t *testing.T, ws, relPath, body string) {
	t.Helper()
	doc, err := toml.Marshal(map[string]any{"sandbox_mode": "read-only", "developer_instructions": body})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	full := filepath.Join(ws, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, append([]byte("# mosaic_version: 1.0\n\n"), doc...), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func TestProbeDeployedArtifact_CodexOrchestratorRegion_VersionAndPresenceRecognised(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	writeCodexRegionAgent(t, ws, ".codex/agents/orchestrator.toml", codexRegionBody)

	// Act
	state := probeDeployedArtifact(ws, ".codex/agents/orchestrator.toml", "model", domain.ArtifactAgent, codexProbeDesc())

	// Assert
	if !state.HasOrchestratorInjectionRegion {
		t.Error("HasOrchestratorInjectionRegion = false, want true for a Codex file carrying the region")
	}
	if state.OrchestratorInjectionsVersion != "1.2.1" {
		t.Errorf("OrchestratorInjectionsVersion = %q, want 1.2.1", state.OrchestratorInjectionsVersion)
	}
}

func TestProbeDeployedArtifact_CodexWithoutOrchestratorRegion_NotPresent(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	writeCodexRegionAgent(t, ws, ".codex/agents/orchestrator.toml", "<Identity type=\"core\">\nAgent body.\n</Identity>\n")

	// Act
	state := probeDeployedArtifact(ws, ".codex/agents/orchestrator.toml", "model", domain.ArtifactAgent, codexProbeDesc())

	// Assert
	if state.HasOrchestratorInjectionRegion || state.OrchestratorInjectionsVersion != "" {
		t.Errorf("region = %v / version = %q, want absent and empty", state.HasOrchestratorInjectionRegion,
			state.OrchestratorInjectionsVersion)
	}
}

// buildHarnessOnlyRoleGaps runs the harness-only route of buildContent over a deployed
// harness-only agent file and returns the kinds of the gaps recorded for it.
func buildHarnessOnlyRoleGaps(t *testing.T, module domain.HarnessModule, ws, relPath string) []domain.TodoItem {
	t.Helper()
	collector := todo.NewCollector()
	svc := &service{deps: Deps{Todo: collector, GOOS: "linux"}}
	agent := HarnessOnlyAgent{TargetPath: relPath, FileName: filepath.Base(relPath), Key: "helper", Role: domain.RoleSubagent}
	reader := newDeployedBytes(ws, nil).funnelReader(module.Descriptor())
	content := svc.buildContent(module, nil, nil, nil, nil, nil, nil, domain.ScopeProject, reader, "",
		domain.ProtocolContent{}, domain.BundleContent{},
		map[string]harnessOnlyContentPlan{relPath: {Agent: agent, Scope: RefreshProtocolOnly}}, nil)
	item := domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "helper"},
		TargetPath: relPath,
		Action:     domain.ActionUpdate,
	}

	_, _ = content(item) // the refresh outcome is not under test; the role-region report is
	return collector.Items()
}

func countRoleForbiddenRegion(items []domain.TodoItem) int {
	n := 0
	for _, it := range items {
		if it.Kind == domain.GapRoleForbiddenRegion {
			n++
		}
	}
	return n
}

func TestBuildContent_HarnessOnlyCodexAgent_RoleRegionCheckReportsOrchestratorRegionOnSubagent(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	writeCodexRegionAgent(t, ws, ".codex/agents/helper.toml", codexRegionBody)

	// Act
	items := buildHarnessOnlyRoleGaps(t, priorBytesCodexModule{}, ws, ".codex/agents/helper.toml")

	// Assert
	if got := countRoleForbiddenRegion(items); got != 1 {
		t.Errorf("role-forbidden-region items = %d (%+v), want 1 as for a Markdown harness", got, items)
	}
}

func TestBuildContent_HarnessOnlyMarkdownAgent_RoleRegionCheckReportsOrchestratorRegionOnSubagent(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	doc := "---\nrole: subagent\nversion: \"1.0\"\n---\n" + codexRegionBody
	if err := os.WriteFile(filepath.Join(ws, "helper.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Act
	items := buildHarnessOnlyRoleGaps(t, seamReplayStubModule{}, ws, "helper.md")

	// Assert
	if got := countRoleForbiddenRegion(items); got != 1 {
		t.Errorf("role-forbidden-region items = %d (%+v), want 1", got, items)
	}
}

// sourceOnlyCatalog serves one catalog source for the undeliverable check.
type sourceOnlyCatalog struct {
	catalog.Catalog
	sources map[string][]byte
}

func (c sourceOnlyCatalog) ReadSource(path string) ([]byte, error) {
	if b, ok := c.sources[path]; ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}

func TestOrchestratorRegionUndeliverable_SourceWithoutMarker_SameForCodexAndMarkdownHarness(t *testing.T) {
	// Arrange: a catalog orchestrator without the region, harnesses without orchestrator content.
	cat := sourceOnlyCatalog{sources: map[string][]byte{
		"orch.md": []byte("---\nrole: orchestrator\n---\n<Identity type=\"core\">\nBody.\n</Identity>\n"),
	}}
	agents := []domain.Agent{{Key: "orchestrator", Role: domain.RoleOrchestrator, SourcePath: "orch.md"}}

	for name, module := range map[string]domain.HarnessModule{
		"codex": priorBytesCodexModule{}, "markdown": seamReplayStubModule{},
	} {
		t.Run(name, func(t *testing.T) {
			// Act
			got := orchestratorRegionUndeliverable(cat, module, agents)

			// Assert
			if !got["orchestrator"] {
				t.Errorf("undeliverable = %v, want the orchestrator marked", got)
			}
		})
	}
}

func TestOrchestratorRegionUndeliverable_SourceWithMarker_NotMarkedForCodexHarness(t *testing.T) {
	// Arrange
	cat := sourceOnlyCatalog{sources: map[string][]byte{
		"orch.md": []byte("---\nrole: orchestrator\n---\n<Identity type=\"core\">\nBody.\n</Identity>\n\n" +
			"<OrchestratorHarnessConstraints type=\"managed\">\n</OrchestratorHarnessConstraints>\n"),
	}}
	agents := []domain.Agent{{Key: "orchestrator", Role: domain.RoleOrchestrator, SourcePath: "orch.md"}}

	// Act
	got := orchestratorRegionUndeliverable(cat, priorBytesCodexModule{}, agents)

	// Assert
	if len(got) != 0 {
		t.Errorf("undeliverable = %v, want none for a source carrying the region", got)
	}
}
