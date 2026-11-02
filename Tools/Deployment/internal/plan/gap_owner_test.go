package plan_test

// gap_owner_test.go verifies that the planner's unmapped-tool gap names the owning agent in
// Owner, so the TODO collector can tell the same tool on two agents apart and recognise the
// same tool on the same agent reported by the transform.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// TestBuild_UnmappedToolGap_OwnerIsTheAgentKey verifies that two agents declaring the same
// unmapped tool each get a gap whose Subject is the tool and whose Owner is that agent.
func TestBuild_UnmappedToolGap_OwnerIsTheAgentKey(t *testing.T) {
	first := makeAgent("agent-one", "1.0")
	first.Tools = []string{"bash-shell"}
	second := makeAgent("agent-two", "1.0")
	second.Tools = []string{"bash-shell"}

	cat := &fakeCatalog{
		orchestrator: makeOrchestrator(),
		workers:      []domain.Agent{first, second},
		workflows:    []domain.Workflow{makeWorkflow("test-wf", first.Key, second.Key)},
	}
	module := newFakeModule()
	module.ToolsFn = func(req domain.ToolRequest) (domain.ToolResult, error) {
		res := make([]domain.ToolResolution, len(req.Generic))
		for i, g := range req.Generic {
			res[i] = domain.ToolResolution{Generic: g, Outcome: domain.ToolUnmapped}
		}
		return domain.ToolResult{Resolutions: res}, nil
	}
	input := plan.Input{
		Catalog:       cat,
		Module:        module,
		Mode:          domain.ModeDeployWorkspace,
		WorkspacePath: "/fake/workspace",
		Scope:         domain.ScopeProject,
		GOOS:          "linux",
		Manifest:      absentSnapshot(),
		WorkflowIDs:   []string{"test-wf"},
		Models: map[string]domain.ModelSelection{
			first.Key:      {ModelID: "test-model", Origin: domain.OriginHarnessList},
			second.Key:     {ModelID: "test-model", Origin: domain.OriginHarnessList},
			"orchestrator": {ModelID: "test-model", Origin: domain.OriginHarnessList},
		},
	}

	p, err := plan.New().Build(context.Background(), input)
	must(t, err)

	owners := map[string]bool{}
	for _, g := range p.Gaps {
		if g.Kind == domain.GapUnmappedTool && g.Subject == "bash-shell" {
			owners[g.Owner] = true
		}
	}
	for _, want := range []string{first.Key, second.Key} {
		if !owners[want] {
			t.Errorf("no GapUnmappedTool for tool %q with Owner %q; owners found: %v", "bash-shell", want, owners)
		}
	}
}
