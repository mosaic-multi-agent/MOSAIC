package app_test

// rerun_preserve_agents_test.go verifies, through real agents runs on all four built-in
// harnesses, what deploying agents over an existing deployment does to agents that are already
// deployed and have gone stale because their catalog version was bumped: project regions,
// custom regions and tool choices stay, the deployed model stays unless the run explicitly
// supplies one for that agent, a kept model leaves no model TODO entry, and an agent with no
// file is created. Every test asserts on the content of several rewritten files.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// rpAgentsRewrittenKeys are the agents an agents run over the first deploy rewrites.
var rpAgentsRewrittenKeys = []string{"parity-runner", "parity-helper"}

// rpAgentsReq is the request of an agents run over a first deploy: the sample subagent and
// utility agent, model questions skipped.
func rpAgentsReq(harness, ws string) app.DeployAgentsRequest {
	return app.DeployAgentsRequest{
		HarnessID: harness, WorkspacePath: ws, Scope: domain.ScopeProject,
		SubagentIDs:            []string{"parity-runner"},
		UtilityAgentIDs:        []string{"parity-helper"},
		InfrastructureAgentIDs: []string{},
		StandaloneAgentIDs:     []string{},
		SkipAll:                rpSkipModels(),
		AutoConfirmPlan:        true,
	}
}

func rpRunAgents(t *testing.T, catRoot string, req app.DeployAgentsRequest) domain.RunSummary {
	t.Helper()
	sum, err := rpService(t, catRoot).DeployAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("agents over an existing deployment: %v", err)
	}
	return sum
}

func TestAgentsRerun_StaleAgents_KeepRegionsAndToolChoices(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpPlant(t, ws, h, rpAgentsRewrittenKeys...)
			rpBumpAll(t, cat)

			rpRunAgents(t, cat, rpAgentsReq(h, ws))

			for _, key := range rpAgentsRewrittenKeys {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				rpAssertKept(t, h, key, got)
			}
		})
	}
}

// rpAgentsModelCase is one way an agents run supplies, or does not supply, models.
type rpAgentsModelCase struct {
	name  string
	apply func(*app.DeployAgentsRequest)
	want  map[string]string // agent key -> model expected in the rewritten file
}

func rpAgentsModelCases() []rpAgentsModelCase {
	return []rpAgentsModelCase{
		{
			name:  "nothing supplied keeps the deployed model of every agent",
			apply: func(*app.DeployAgentsRequest) {},
			want:  map[string]string{"parity-runner": rpDeployedModel, "parity-helper": rpDeployedModel},
		},
		{
			name: "an agent model replaces only that agent's model",
			apply: func(r *app.DeployAgentsRequest) {
				r.AgentModels = map[string]string{"parity-runner": rpRunModel}
			},
			want: map[string]string{"parity-runner": rpRunModel, "parity-helper": rpDeployedModel},
		},
		{
			name: "a tier model replaces the model of that tier's agents only",
			apply: func(r *app.DeployAgentsRequest) {
				r.TierModels = map[domain.Tier]string{"LOW": rpTierModel}
			},
			want: map[string]string{"parity-runner": rpDeployedModel, "parity-helper": rpTierModel},
		},
	}
}

func TestAgentsRerun_ModelKeptUnlessRunSuppliesOne(t *testing.T) {
	for _, h := range parityHarnesses {
		for _, c := range rpAgentsModelCases() {
			h, c := h, c
			t.Run(h+"/"+c.name, func(t *testing.T) {
				cat := rpCopyCatalog(t)
				ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
				rpBumpAll(t, cat)
				req := rpAgentsReq(h, ws)
				c.apply(&req)

				sum := rpRunAgents(t, cat, req)

				for _, key := range rpAgentsRewrittenKeys {
					got := rpReadAgent(t, ws, h, key)
					rpAssertRewritten(t, key, got)
					rpAssertModel(t, key, got, c.want[key])
				}
				if gaps := rpModelTodos(sum, rpAgentsRewrittenKeys...); len(gaps) > 0 {
					t.Errorf("model TODO entries for agents whose model is known: %v", gaps)
				}
			})
		}
	}
}

// rpOrchestratorKeys are the orchestrator-role agents an infrastructure selection makes an
// agents run rewrite.
var rpOrchestratorKeys = []string{"orchestrator", "orchestrator-script"}

// The first deploy declares no infrastructure agent, so selecting one in the agents run makes the
// orchestrators drift and be rewritten. This path already preserves state today; the test is
// regression coverage that moving it onto the shared rule loses nothing.
func TestAgentsRerun_StaleOrchestrators_KeepRegionsToolsModelAndFollowSelection(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpPlant(t, ws, h, rpOrchestratorKeys...)
			rpBumpAll(t, cat)
			req := rpAgentsReq(h, ws)
			req.InfrastructureAgentIDs = []string{"parity-checker"}

			sum := rpRunAgents(t, cat, req)

			for _, key := range rpOrchestratorKeys {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				rpAssertKept(t, h, key, got)
				rpAssertModel(t, key, got, rpDeployedModel)
				if decl := rpManagedRegion(t, got, "InfrastructureAgents"); !strings.Contains(decl, "parity-checker") {
					t.Errorf("%s: selected infrastructure agent is not declared: %q", key, decl)
				}
			}
			if gaps := rpModelTodos(sum, rpOrchestratorKeys...); len(gaps) > 0 {
				t.Errorf("model TODO entries for orchestrators whose model is known: %v", gaps)
			}
		})
	}
}

func TestAgentsRerun_AgentWithoutFile_IsCreatedBesideRewrittenAgents(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{})
			if rpAgentExists(t, ws, h, "parity-helper") {
				t.Fatalf("the first deploy already wrote the utility agent")
			}
			rpPlant(t, ws, h, "parity-runner")
			rpBumpAll(t, cat)
			req := rpAgentsReq(h, ws)
			req.AgentModels = map[string]string{"parity-helper": rpRunModel}

			rpRunAgents(t, cat, req)

			created := rpReadAgent(t, ws, h, "parity-helper")
			rpAssertRewritten(t, "parity-helper", created)
			rpAssertModel(t, "parity-helper", created, rpRunModel)
			runner := rpReadAgent(t, ws, h, "parity-runner")
			rpAssertRewritten(t, "parity-runner", runner)
			rpAssertKept(t, h, "parity-runner", runner)
			rpAssertModel(t, "parity-runner", runner, rpDeployedModel)
		})
	}
}
