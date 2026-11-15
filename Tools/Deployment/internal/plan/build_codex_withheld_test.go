package plan_test

// build_codex_withheld_test.go builds plans with the real Codex module and checks the deploy
// report entry for web_fetch: a read-only agent that asks for it is told it gets no network
// access (entry names the agent and the tool), a write-capable agent is not.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/codex"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/plan"
)

// buildRealCodexPlan builds a create plan for one worker agent with the given generic tools.
func buildRealCodexPlan(t *testing.T, tools ...string) domain.Plan {
	t.Helper()
	frozen, err := filepath.Abs(filepath.Join("..", "..", "testdata", "frozen-catalog", "Catalog"))
	if err != nil {
		t.Fatalf("resolve frozen catalog: %v", err)
	}
	mod, err := codex.New(registry.BuiltinOptions{ContentDir: func(dir string) (string, error) {
		return catalogpaths.HarnessInjectionsDir(frozen, dir), nil
	}})
	if err != nil {
		t.Fatalf("codex.New: %v", err)
	}
	agent := makeAgent("net-agent", "1.0")
	agent.Tools = tools
	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workers:      []domain.Agent{agent},
		workflows:    []domain.Workflow{makeWorkflow("net-wf", agent.Key)},
	}
	p, err := plan.New().Build(context.Background(), plan.Input{
		Catalog:       cat,
		Module:        mod,
		Mode:          domain.ModeDeployWorkspace,
		WorkspacePath: "/fake/workspace",
		Scope:         domain.ScopeProject,
		GOOS:          "linux",
		Manifest:      absentSnapshot(),
		WorkflowIDs:   []string{"net-wf"},
		Models: map[string]domain.ModelSelection{
			agent.Key:      {ModelID: "gpt-5.6-sol", Origin: domain.OriginHarnessList},
			"orchestrator": {ModelID: "gpt-5.6-sol", Origin: domain.OriginHarnessList},
		},
	})
	if err != nil {
		t.Fatalf("plan.Build: %v", err)
	}
	return p
}

func TestBuild_CodexReadOnlyAgentWithWebFetch_ReportsNoNetworkAccess(t *testing.T) {
	// Arrange / Act
	p := buildRealCodexPlan(t, "file_read", "web_fetch")

	// Assert
	gap, ok := findGapForSubject(p.Gaps, domain.GapToolCapabilityWithheld, "web_fetch")
	if !ok {
		t.Fatalf("plan gaps lack a withheld-capability entry for web_fetch: %+v", p.Gaps)
	}
	if gap.Owner != "net-agent" {
		t.Errorf("gap Owner = %q, want net-agent", gap.Owner)
	}
}

func TestBuild_CodexWriteCapableAgentWithWebFetch_ReportsNothing(t *testing.T) {
	for _, tools := range [][]string{
		{"file_write", "web_fetch"},
		{"terminal", "web_fetch"},
	} {
		// Arrange / Act
		p := buildRealCodexPlan(t, tools...)

		// Assert
		if gaps := withheldGapsOf(p); len(gaps) != 0 {
			t.Errorf("tools %v: withheld-capability gaps %+v, want none", tools, gaps)
		}
	}
}

func TestBuild_CodexTerminalAlone_ReportsNothing(t *testing.T) {
	// Arrange / Act
	p := buildRealCodexPlan(t, "terminal")

	// Assert
	if gaps := withheldGapsOf(p); len(gaps) != 0 {
		t.Errorf("withheld-capability gaps %+v, want none", gaps)
	}
}
