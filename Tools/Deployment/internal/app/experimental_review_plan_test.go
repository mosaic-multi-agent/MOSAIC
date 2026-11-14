package app_test

// experimental_review_plan_test.go tests that every plan the app sends to Interaction.Review
// lets the review frontend show, as a warning, the reason of each experimental harness the
// operation involves: the target for deploy, update, workflows, agents and hooks, and both the
// source and the target for transform. A run that involves no experimental harness exposes
// none. Behaviour is driven by the descriptor declaration only.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// reviewFlow runs one review-calling operation against harnessID in workspace ws.
type reviewFlow struct {
	name string
	run  func(svc app.Service, harnessID, ws string) (domain.RunSummary, error)
}

func expDeploy(svc app.Service, harnessID, ws string, dryRun bool) (domain.RunSummary, error) {
	return svc.DeployNew(context.Background(), app.DeployRequest{
		HarnessID: harnessID, WorkspacePath: ws, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
		AgentModels: twinModels(), AutoConfirmPlan: true, DryRun: dryRun,
		ConflictDefault: domain.DecisionSkip})
}

// reviewFlows lists every flow that calls Review. Flows other than deploy run over a workspace
// that deploy already filled.
var reviewFlows = []reviewFlow{
	{"deploy", func(svc app.Service, id, ws string) (domain.RunSummary, error) {
		return expDeploy(svc, id, ws, false)
	}},
	{"update", func(svc app.Service, id, ws string) (domain.RunSummary, error) {
		return svc.Update(context.Background(), app.UpdateRequest{
			HarnessID: id, WorkspacePath: ws, AgentModels: twinModels(),
			AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
	}},
	{"workflows", func(svc app.Service, id, ws string) (domain.RunSummary, error) {
		return svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
			HarnessID: id, WorkspacePath: ws, WorkflowIDs: []string{"parity-flow"},
			AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
	}},
	{"agents", func(svc app.Service, id, ws string) (domain.RunSummary, error) {
		return svc.DeployAgents(context.Background(), app.DeployAgentsRequest{
			HarnessID: id, WorkspacePath: ws, Scope: domain.ScopeProject,
			SubagentIDs: []string{owRunner}, UtilityAgentIDs: []string{}, InfrastructureAgentIDs: []string{},
			StandaloneAgentIDs: []string{}, AgentModels: twinModels(), AutoConfirmPlan: true,
			ConflictDefault: domain.DecisionSkip})
	}},
	{"hooks", func(svc app.Service, id, ws string) (domain.RunSummary, error) {
		return svc.DeployHooks(context.Background(), app.DeployHooksRequest{
			HarnessID: id, WorkspacePath: ws, Scope: domain.ScopeProject, HookIDs: []string{},
			AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
	}},
}

// reviewedPlanOf runs the flow for the module's harness and returns the plan the flow sent to
// Review. Flows other than deploy are preceded by a deploy whose plan is discarded.
func reviewedPlanOf(t *testing.T, flow reviewFlow, mod domain.HarnessModule) domain.Plan {
	t.Helper()
	rec := newRecordingInteraction(nil)
	svc := newTestExperimentalService(t, []domain.HarnessModule{mod}, withInteraction(rec))
	ws := t.TempDir()
	id := mod.Ref().ID
	if flow.name != "deploy" {
		if _, err := expDeploy(svc, id, ws, false); err != nil {
			t.Fatalf("setup deploy: %v", err)
		}
		rec.reset()
	}
	if _, err := flow.run(svc, id, ws); err != nil {
		t.Fatalf("%s flow: %v", flow.name, err)
	}
	return rec.lastPlan(t)
}

// assertPlanExperimental checks the plan exposes exactly the given harnesses, in order, with
// the reason of each; an empty want requires that the plan exposes none.
func assertPlanExperimental(t *testing.T, p domain.Plan, want ...domain.HarnessRef) {
	t.Helper()
	got := p.ExperimentalHarnesses
	if len(got) != len(want) {
		t.Fatalf("plan exposes %d experimental harnesses (%+v), want %d", len(got), got, len(want))
	}
	for i, ref := range want {
		if got[i].HarnessID != ref.ID || got[i].Reason != ref.ExperimentalReason {
			t.Errorf("ExperimentalHarnesses[%d] = %+v, want harness %q with reason %q",
				i, got[i], ref.ID, ref.ExperimentalReason)
		}
	}
	if len(want) == 0 && got != nil {
		t.Errorf("ExperimentalHarnesses = %#v, want nil when no harness is experimental", got)
	}
}

func TestReviewPlan_ExperimentalTarget_ExposesReasonInEveryFlow(t *testing.T) {
	for _, flow := range reviewFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expHarnessID, expReason)

			// Act
			p := reviewedPlanOf(t, flow, mod)

			// Assert
			assertPlanExperimental(t, p, mod.Ref())
		})
	}
}

func TestReviewPlan_NoExperimentalHarness_ExposesNoneInEveryFlow(t *testing.T) {
	for _, flow := range reviewFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expPlainID, "")

			// Act
			p := reviewedPlanOf(t, flow, mod)

			// Assert
			assertPlanExperimental(t, p)
		})
	}
}

func TestReviewPlan_DryRunDeployOfExperimentalHarness_ExposesReason(t *testing.T) {
	// Arrange
	mod := newTestExperimentalModule(t, expHarnessID, expReason)
	rec := newRecordingInteraction(nil)
	svc := newTestExperimentalService(t, []domain.HarnessModule{mod}, withInteraction(rec))

	// Act
	if _, err := expDeploy(svc, expHarnessID, t.TempDir(), true); err != nil {
		t.Fatalf("dry-run deploy: %v", err)
	}

	// Assert
	assertPlanExperimental(t, rec.lastPlan(t), mod.Ref())
}

// transformReviewedPlan deploys the parity catalog with the source module, transforms the
// workspace to the target module and returns the plan the transform sent to Review.
func transformReviewedPlan(t *testing.T, src, tgt domain.HarnessModule) domain.Plan {
	t.Helper()
	rec := newRecordingInteraction(nil)
	svc := newTestExperimentalService(t, []domain.HarnessModule{src, tgt}, withInteraction(rec))
	ws := t.TempDir()
	if _, err := expDeploy(svc, src.Ref().ID, ws, false); err != nil {
		t.Fatalf("setup deploy to source: %v", err)
	}
	rec.reset()
	_, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: src.Ref().ID, TargetHarnessID: tgt.Ref().ID, Workspace: ws,
		TargetModel: parityModel})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	return rec.lastPlan(t)
}

func TestReviewPlan_TransformToExperimentalTarget_ExposesTargetReason(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expPlainID, "")
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID, expReason)

	// Act
	p := transformReviewedPlan(t, src, tgt)

	// Assert
	assertPlanExperimental(t, p, tgt.Ref())
	if p.Harness.ID != tgt.Ref().ID {
		t.Errorf("plan.Harness = %q, want the target %q", p.Harness.ID, tgt.Ref().ID)
	}
}

func TestReviewPlan_TransformFromExperimentalSourceToPlainTarget_ExposesSourceReason(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expPlainID, "")

	// Act
	p := transformReviewedPlan(t, src, tgt)

	// Assert
	assertPlanExperimental(t, p, src.Ref())
	if p.Harness.ID != tgt.Ref().ID {
		t.Errorf("plan.Harness = %q, want the target %q", p.Harness.ID, tgt.Ref().ID)
	}
}

func TestReviewPlan_TransformBetweenTwoExperimentalHarnesses_ExposesSourceThenTarget(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID2, expReason2)

	// Act
	p := transformReviewedPlan(t, src, tgt)

	// Assert
	assertPlanExperimental(t, p, src.Ref(), tgt.Ref())
}

func TestReviewPlan_TransformBetweenPlainHarnesses_ExposesNone(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expPlainID, "")
	tgt := newTestExperimentalModuleOn(t, hClaude, "other-plain", "")

	// Act
	p := transformReviewedPlan(t, src, tgt)

	// Assert
	assertPlanExperimental(t, p)
}
