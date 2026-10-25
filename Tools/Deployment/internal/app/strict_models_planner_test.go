package app_test

// strict_models_planner_test.go verifies that the strict-models decision is taken from the
// gaps the real planner produces, and that agents of every kind, or unknown to the catalog,
// are named with their declared tier.
//
// The companion strict_models_test.go injects hand-built gaps through a stub planner. The
// tests here run plan.New() so the whole path plan item -> Plan.Gaps -> strict decision is
// exercised.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// newRealPlannerStrictFixture builds a service using the real planner over the minimal
// catalog (worker "test-runner" tier HIGH, orchestrator without tier). No interaction is
// scripted beyond plan review, so every model must come from the request.
func newRealPlannerStrictFixture(t *testing.T) (app.Service, *countingExecutor, string) {
	t.Helper()
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newBaseDeps(t, stub)
	exec := &countingExecutor{inner: stubExecutor{result: newMinimalExecResult(workspace)}}
	deps.Planner = plan.New()
	deps.Executor = exec
	return app.New(deps), exec, workspace
}

// realPlannerStrictRequest skips the interactive model questions and applies the given
// per-agent models, so any agent not listed stays without a model.
func realPlannerStrictRequest(workspace string, dryRun bool, agentModels map[string]string) app.DeployRequest {
	req := strictRequest(workspace, dryRun)
	req.SkipAll = map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true}
	req.AgentModels = agentModels
	return req
}

// TestDeployNew_StrictModels_RealPlanner_CreatedAgentWithoutModel_FailsNamingAgentAndTier
// verifies that a created agent the real planner leaves without a model fails the run, for
// real and dry runs, naming the agent and the tier from the catalog.
func TestDeployNew_StrictModels_RealPlanner_CreatedAgentWithoutModel_FailsNamingAgentAndTier(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "real"
		if dryRun {
			name = "dry-run"
		}
		t.Run(name, func(t *testing.T) {
			svc, exec, workspace := newRealPlannerStrictFixture(t)
			req := realPlannerStrictRequest(workspace, dryRun, map[string]string{"orchestrator": "model-a"})

			_, err := svc.DeployNew(context.Background(), req)

			ume := requireUnresolvedModels(t, err)
			if len(ume.Agents) != 1 || ume.Agents[0].AgentKey != "test-runner" || ume.Agents[0].Tier != "HIGH" {
				t.Errorf("Agents = %+v, want exactly [{test-runner HIGH}]: the orchestrator has a model", ume.Agents)
			}
			if exec.calls != 0 {
				t.Errorf("executor reached %d time(s), want 0", exec.calls)
			}
		})
	}
}

// TestDeployNew_StrictModels_RealPlanner_NamesEveryUnresolvedAgentSortedByKey verifies that
// with no model for any agent, the real planner's gaps name both agents, sorted by key, with
// the orchestrator's undeclared tier empty.
func TestDeployNew_StrictModels_RealPlanner_NamesEveryUnresolvedAgentSortedByKey(t *testing.T) {
	svc, _, workspace := newRealPlannerStrictFixture(t)
	req := realPlannerStrictRequest(workspace, false, nil)

	_, err := svc.DeployNew(context.Background(), req)

	ume := requireUnresolvedModels(t, err)
	want := []app.UnresolvedAgentModel{
		{AgentKey: "orchestrator", Tier: ""},
		{AgentKey: "test-runner", Tier: "HIGH"},
	}
	if len(ume.Agents) != len(want) {
		t.Fatalf("Agents = %+v, want %+v", ume.Agents, want)
	}
	for i := range want {
		if ume.Agents[i] != want[i] {
			t.Errorf("Agents[%d] = %+v, want %+v", i, ume.Agents[i], want[i])
		}
	}
}

// TestDeployNew_StrictModels_RealPlanner_AllModelsResolved_Succeeds is regression coverage:
// when every created agent has a model, the real planner emits no no-model gap and a strict
// run executes as today.
func TestDeployNew_StrictModels_RealPlanner_AllModelsResolved_Succeeds(t *testing.T) {
	svc, exec, workspace := newRealPlannerStrictFixture(t)
	req := realPlannerStrictRequest(workspace, false, map[string]string{
		"orchestrator": "model-a",
		"test-runner":  "model-b",
	})

	_, err := svc.DeployNew(context.Background(), req)

	if err != nil {
		t.Fatalf("DeployNew: %v, want success when every agent has a model", err)
	}
	if exec.calls != 1 {
		t.Errorf("executor reached %d time(s), want 1", exec.calls)
	}
}

// TestDeployNew_StrictModels_AgentKinds_NamedWithTierOrEmptyWhenUnknown verifies that
// unresolved utility, infrastructure and workflow agents are named with the tier the
// catalog declares, and that an agent key absent from the catalog is named with an empty
// tier instead of being skipped.
func TestDeployNew_StrictModels_AgentKinds_NamedWithTierOrEmptyWhenUnknown(t *testing.T) {
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newBaseDeps(t, stub)
	cat := newMinimalCatalog()
	cat.utilityAgents = []domain.Agent{
		{Key: "util-helper", Version: "1.0", Name: "Util Helper", Role: domain.RoleUtility, RecommendedTier: "LOW"},
	}
	cat.infraAgents = []domain.Agent{
		{Key: "infra-probe", Version: "1.0", Name: "Infra Probe", Role: domain.RoleWorker, RecommendedTier: "MEDIUM", Infrastructure: "probe"},
	}
	deps.Catalog = cat
	p := newMinimalPlan(workspace)
	p.Gaps = []domain.Gap{
		noModelGap("util-helper"),
		noModelGap("infra-probe"),
		noModelGap("ghost-agent"),
		noModelGap("test-runner"),
	}
	deps.Planner = &stubPlanner{plan: p}
	deps.Executor = &countingExecutor{inner: stubExecutor{result: newMinimalExecResult(workspace)}}
	svc := app.New(deps)

	_, err := svc.DeployNew(context.Background(), strictRequest(workspace, false))

	ume := requireUnresolvedModels(t, err)
	want := []app.UnresolvedAgentModel{
		{AgentKey: "ghost-agent", Tier: ""},
		{AgentKey: "infra-probe", Tier: "MEDIUM"},
		{AgentKey: "test-runner", Tier: "HIGH"},
		{AgentKey: "util-helper", Tier: "LOW"},
	}
	if len(ume.Agents) != len(want) {
		t.Fatalf("Agents = %+v, want %+v", ume.Agents, want)
	}
	for i := range want {
		if ume.Agents[i] != want[i] {
			t.Errorf("Agents[%d] = %+v, want %+v", i, ume.Agents[i], want[i])
		}
	}
}

// TestDeployNew_StrictModels_ItemsWithoutGap_AreNeverNamed verifies that plan items whose
// model field is unresolved but for which the plan emits no no-model gap (an unchanged
// item, or an update that keeps the deployed model) are not named: only Plan.Gaps decides.
func TestDeployNew_StrictModels_ItemsWithoutGap_AreNeverNamed(t *testing.T) {
	deps, workspace := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	p := newMinimalPlan(workspace)
	p.Items = append(p.Items,
		domain.PlanItem{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "plan-review"},
			TargetPath: "plan-review.md",
			Action:     domain.ActionUnchanged,
		},
		domain.PlanItem{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "orchestrator"},
			TargetPath: "orchestrator.md",
			Action:     domain.ActionUpdate,
			Stale:      []domain.VersionDelta{{Field: "version", Deployed: "1.0", Source: "2.0"}},
		},
	)
	p.Gaps = []domain.Gap{noModelGap("test-runner")}
	deps.Planner = &stubPlanner{plan: p}
	svc := app.New(deps)

	_, err := svc.DeployNew(context.Background(), strictRequest(workspace, false))

	ume := requireUnresolvedModels(t, err)
	if len(ume.Agents) != 1 || ume.Agents[0].AgentKey != "test-runner" {
		t.Errorf("Agents = %+v, want only test-runner: items without a no-model gap are never named", ume.Agents)
	}
}
