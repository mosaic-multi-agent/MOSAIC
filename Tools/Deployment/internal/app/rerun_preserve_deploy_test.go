package app_test

// rerun_preserve_deploy_test.go verifies, through real deploy runs on all four built-in
// harnesses, what a second deploy to a workspace does to agents that are already deployed and
// have gone stale because their catalog version was bumped: the user's project regions, custom
// regions and tool choices stay, the model comes from the run when it supplies one and
// otherwise stays, infrastructure declarations stay unless the run selects infrastructure
// agents, the workflow set follows the selection, and an agent with no file is created. Every
// test asserts on the content of several rewritten files.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// rpDeployReq is the request of a later deploy over a first deploy: the sample workflow and
// utility agent, no infrastructure agent, model questions skipped.
func rpDeployReq(harness, ws string) app.DeployRequest {
	return app.DeployRequest{
		HarnessID: harness, WorkspacePath: ws, Scope: domain.ScopeProject,
		WorkflowIDs:            []string{"parity-flow"},
		UtilityAgentIDs:        []string{"parity-helper"},
		InfrastructureAgentIDs: []string{},
		HookIDs:                []string{},
		SkipAll:                rpSkipModels(),
		AutoConfirmPlan:        true,
	}
}

// rpRewrittenKeys are the agents a deploy over the first deploy rewrites once all are bumped.
var rpRewrittenKeys = []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"}

func rpRunDeploy(t *testing.T, catRoot string, req app.DeployRequest) domain.RunSummary {
	t.Helper()
	sum, err := rpService(t, catRoot).DeployNew(context.Background(), req)
	if err != nil {
		t.Fatalf("deploy over an existing deployment: %v", err)
	}
	return sum
}

func TestDeployRerun_StaleAgents_KeepRegionsAndToolChoices(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpPlant(t, ws, h, rpRewrittenKeys...)
			rpBumpAll(t, cat)

			rpRunDeploy(t, cat, rpDeployReq(h, ws))

			for _, key := range rpRewrittenKeys {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				rpAssertKept(t, h, key, got)
			}
		})
	}
}

func TestDeployRerun_ModelFromRunWhenSuppliedOtherwiseDeployed(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpBumpAll(t, cat)
			req := rpDeployReq(h, ws)
			req.AgentModels = map[string]string{"parity-runner": rpRunModel}

			sum := rpRunDeploy(t, cat, req)

			for _, key := range rpRewrittenKeys {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				want := rpDeployedModel
				if key == "parity-runner" {
					want = rpRunModel
				}
				rpAssertModel(t, key, got, want)
			}
			if kept := rpModelTodos(sum, rpRewrittenKeys...); len(kept) > 0 {
				t.Errorf("model TODO entries for agents whose model is known: %v", kept)
			}
		})
	}
}

func TestDeployRerun_TierModelOfRunWinsOverDeployedModel(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}})
			rpBumpAll(t, cat)
			req := rpDeployReq(h, ws)
			req.TierModels = map[domain.Tier]string{"LOW": rpTierModel}

			rpRunDeploy(t, cat, req)

			rpAssertModel(t, "parity-helper", rpReadAgent(t, ws, h, "parity-helper"), rpTierModel)
			rpAssertModel(t, "parity-runner", rpReadAgent(t, ws, h, "parity-runner"), rpDeployedModel)
			rpAssertModel(t, "orchestrator", rpReadAgent(t, ws, h, "orchestrator"), rpDeployedModel)
		})
	}
}

// rpInfraCase is one infrastructure selection of a later deploy.
type rpInfraCase struct {
	name    string
	catalog func(t *testing.T, catRoot string) // optional change to the catalog before the later run
	apply   func(*app.DeployRequest)
	expect  func(t *testing.T, before, after string)
}

func rpExpectDeclarationsKept(t *testing.T, before, after string) {
	t.Helper()
	if after != before {
		t.Errorf("declarations changed without an infrastructure selection:\nbefore: %q\nafter:  %q", before, after)
	}
}

func rpInfraCases() []rpInfraCase {
	return []rpInfraCase{
		{
			name: "no request selection and no infrastructure agents in the catalog keeps the declarations",
			catalog: func(t *testing.T, catRoot string) {
				path := filepath.Join(catRoot, "Subagents", "Infrastructure", "parity-checker.md")
				if err := os.Remove(path); err != nil {
					t.Fatalf("remove infrastructure agent from the catalog copy: %v", err)
				}
			},
			apply:  func(r *app.DeployRequest) { r.InfrastructureAgentIDs = nil },
			expect: rpExpectDeclarationsKept,
		},
		{
			name:   "no request selection and an empty interactive answer keeps the declarations",
			apply:  func(r *app.DeployRequest) { r.InfrastructureAgentIDs = nil },
			expect: rpExpectDeclarationsKept,
		},
		{
			name: "no selection keeps the deployed declarations",
			apply: func(r *app.DeployRequest) {
				r.InfrastructureAgentIDs = nil
				r.SkipAll[domain.QInfrastructureAgents] = true
			},
			expect: rpExpectDeclarationsKept,
		},
		{
			name:  "explicitly empty selection clears the declarations",
			apply: func(r *app.DeployRequest) { r.InfrastructureAgentIDs = []string{} },
			expect: func(t *testing.T, _, after string) {
				if after != "" {
					t.Errorf("declarations remain after an explicitly empty selection: %q", after)
				}
			},
		},
		{
			name:  "explicit selection writes the selected declarations",
			apply: func(r *app.DeployRequest) { r.InfrastructureAgentIDs = []string{"parity-checker"} },
			expect: func(t *testing.T, _, after string) {
				if !strings.Contains(after, "parity-checker") {
					t.Errorf("selected infrastructure agent is not declared: %q", after)
				}
			},
		},
	}
}

func TestDeployRerun_InfrastructureDeclarationsFollowTheSelection(t *testing.T) {
	for _, h := range parityHarnesses {
		for _, c := range rpInfraCases() {
			h, c := h, c
			t.Run(h+"/"+c.name, func(t *testing.T) {
				cat := rpCopyCatalog(t)
				ws := rpFirstDeploy(t, h, cat, rpFirst{infra: []string{"parity-checker"}})
				before := map[string]string{}
				for _, key := range []string{"orchestrator", "orchestrator-script"} {
					before[key] = rpManagedRegion(t, rpReadAgent(t, ws, h, key), "InfrastructureAgents")
					if before[key] == "" {
						t.Fatalf("%s: the first deploy declared no infrastructure agent", key)
					}
				}
				rpBumpAll(t, cat)
				if c.catalog != nil {
					c.catalog(t, cat)
				}
				req := rpDeployReq(h, ws)
				req.UtilityAgentIDs = []string{}
				c.apply(&req)

				rpRunDeploy(t, cat, req)

				for _, key := range []string{"orchestrator", "orchestrator-script"} {
					got := rpReadAgent(t, ws, h, key)
					rpAssertRewritten(t, key, got)
					c.expect(t, before[key], rpManagedRegion(t, got, "InfrastructureAgents"))
				}
			})
		}
	}
}

func TestDeployRerun_WorkflowSetIsReplacedBySelection(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{})
			rpAddWorkflow(t, cat, "parity-flow-two")
			req := rpDeployReq(h, ws)
			req.UtilityAgentIDs = []string{}
			req.WorkflowIDs = []string{"parity-flow-two"}

			rpRunDeploy(t, cat, req)

			for _, key := range []string{"orchestrator", "orchestrator-script"} {
				region := rpManagedRegion(t, rpReadAgent(t, ws, h, key), "AvailableWorkflows")
				if !strings.Contains(region, `name="parity-flow-two"`) {
					t.Errorf("%s: selected workflow is missing from the workflow set", key)
				}
				if strings.Contains(region, `name="parity-flow"`) {
					t.Errorf("%s: deselected workflow is still in the workflow set", key)
				}
			}
		})
	}
}

func TestDeployRerun_AgentWithoutFile_IsCreatedBesideRewrittenAgents(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{})
			if rpAgentExists(t, ws, h, "parity-helper") {
				t.Fatalf("the first deploy already wrote the utility agent")
			}
			rpPlant(t, ws, h, "parity-runner", "orchestrator")
			rpBumpAll(t, cat)
			req := rpDeployReq(h, ws)
			req.AgentModels = map[string]string{"parity-helper": rpRunModel}

			rpRunDeploy(t, cat, req)

			created := rpReadAgent(t, ws, h, "parity-helper")
			rpAssertRewritten(t, "parity-helper", created)
			rpAssertModel(t, "parity-helper", created, rpRunModel)
			for _, key := range []string{"parity-runner", "orchestrator"} {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				rpAssertKept(t, h, key, got)
				rpAssertModel(t, key, got, rpDeployedModel)
			}
		})
	}
}

// A fresh deploy that resolves no model still fails under strict models: keeping a deployed
// model must not hide a model that is missing on a new agent.
func TestDeployFresh_StrictModelsWithoutModel_StillFails(t *testing.T) {
	cat := rpCopyCatalog(t)
	req := rpDeployReq(hClaude, t.TempDir())
	req.StrictModels = true

	_, err := rpService(t, cat).DeployNew(context.Background(), req)

	if !errors.Is(err, app.ErrUnresolvedModels) {
		t.Fatalf("error = %v, want one matching ErrUnresolvedModels", err)
	}
}
