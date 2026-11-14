package app_test

// experimental_results_test.go tests that the result of every operation that writes to a
// harness workspace reports, as a field of the result, each experimental harness the operation
// involves, exactly once per operation: deploy, update, workflows, agents and hooks (the target),
// promote (the harness the file was interpreted against) and transform (the target, or the
// source alone). Dry runs report it too. Behaviour is driven by the descriptor declaration only.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// opFlags are the request flags a summary-returning flow runs with.
type opFlags struct {
	dryRun      bool
	autoConfirm bool
}

// opFlow runs one summary-returning operation against harnessID in workspace ws.
type opFlow struct {
	name string
	run  func(svc app.Service, harnessID, ws string, f opFlags) (domain.RunSummary, error)
}

// opFlows lists every operation that returns a domain.RunSummary. Flows other than deploy run
// over a workspace that deploy already filled.
var opFlows = []opFlow{
	{"deploy", func(svc app.Service, id, ws string, f opFlags) (domain.RunSummary, error) {
		return svc.DeployNew(context.Background(), app.DeployRequest{
			HarnessID: id, WorkspacePath: ws, Scope: domain.ScopeProject,
			WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
			AgentModels: twinModels(), AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun,
			ConflictDefault: domain.DecisionSkip})
	}},
	{"update", func(svc app.Service, id, ws string, f opFlags) (domain.RunSummary, error) {
		return svc.Update(context.Background(), app.UpdateRequest{
			HarnessID: id, WorkspacePath: ws, AgentModels: twinModels(),
			AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun, ConflictDefault: domain.DecisionSkip})
	}},
	{"workflows", func(svc app.Service, id, ws string, f opFlags) (domain.RunSummary, error) {
		return svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
			HarnessID: id, WorkspacePath: ws, WorkflowIDs: []string{"parity-flow"},
			AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun, ConflictDefault: domain.DecisionSkip})
	}},
	{"agents", func(svc app.Service, id, ws string, f opFlags) (domain.RunSummary, error) {
		return svc.DeployAgents(context.Background(), app.DeployAgentsRequest{
			HarnessID: id, WorkspacePath: ws, Scope: domain.ScopeProject,
			SubagentIDs: []string{owRunner}, UtilityAgentIDs: []string{}, InfrastructureAgentIDs: []string{},
			StandaloneAgentIDs: []string{}, AgentModels: twinModels(), AutoConfirmPlan: f.autoConfirm,
			DryRun: f.dryRun, ConflictDefault: domain.DecisionSkip})
	}},
	{"hooks", func(svc app.Service, id, ws string, f opFlags) (domain.RunSummary, error) {
		return svc.DeployHooks(context.Background(), app.DeployHooksRequest{
			HarnessID: id, WorkspacePath: ws, Scope: domain.ScopeProject, HookIDs: []string{},
			AutoConfirmPlan: f.autoConfirm, DryRun: f.dryRun, ConflictDefault: domain.DecisionSkip})
	}},
}

// seedWorkspace deploys the parity catalog to a new workspace through svc so that the flows
// other than deploy have something to act on.
func seedWorkspace(t *testing.T, svc app.Service, harnessID string) string {
	t.Helper()
	ws := t.TempDir()
	if _, err := opFlows[0].run(svc, harnessID, ws, opFlags{autoConfirm: true}); err != nil {
		t.Fatalf("setup deploy: %v", err)
	}
	return ws
}

// runOpFlow runs the flow against a freshly seeded workspace (deploy runs over an empty one).
func runOpFlow(t *testing.T, flow opFlow, svc app.Service, harnessID string, f opFlags) (domain.RunSummary, error) {
	t.Helper()
	ws := t.TempDir()
	if flow.name != "deploy" {
		ws = seedWorkspace(t, svc, harnessID)
	}
	return flow.run(svc, harnessID, ws, f)
}

// assertReported checks the result reports exactly the given harnesses, once each, in order,
// with the identity and reason of each; an empty want requires that nothing is reported.
func assertReported(t *testing.T, got []domain.ExperimentalHarness, want ...domain.HarnessRef) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("result reports %d experimental harnesses (%+v), want %d", len(got), got, len(want))
	}
	for i, ref := range want {
		if got[i].HarnessID != ref.ID || got[i].Reason != ref.ExperimentalReason || got[i].DisplayName != ref.DisplayName {
			t.Errorf("ExperimentalHarnesses[%d] = %+v, want harness %q (%q) with reason %q",
				i, got[i], ref.ID, ref.DisplayName, ref.ExperimentalReason)
		}
	}
	if len(want) == 0 && got != nil {
		t.Errorf("ExperimentalHarnesses = %#v, want nil when no harness is experimental", got)
	}
}

func TestRunSummary_ExperimentalTarget_ReportsHarnessOnceInEveryFlow(t *testing.T) {
	for _, flow := range opFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expHarnessID, expReason)
			svc := newTestExperimentalService(t, []domain.HarnessModule{mod})

			// Act
			sum, err := runOpFlow(t, flow, svc, expHarnessID, opFlags{autoConfirm: true})

			// Assert
			if err != nil {
				t.Fatalf("%s: %v", flow.name, err)
			}
			assertReported(t, sum.ExperimentalHarnesses, mod.Ref())
		})
	}
}

func TestRunSummary_DryRunOfExperimentalTarget_ReportsHarnessInEveryFlow(t *testing.T) {
	for _, flow := range opFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expHarnessID, expReason)
			svc := newTestExperimentalService(t, []domain.HarnessModule{mod})

			// Act
			sum, err := runOpFlow(t, flow, svc, expHarnessID, opFlags{dryRun: true, autoConfirm: true})

			// Assert
			if err != nil {
				t.Fatalf("%s dry run: %v", flow.name, err)
			}
			assertReported(t, sum.ExperimentalHarnesses, mod.Ref())
		})
	}
}

func TestRunSummary_PlainTarget_ReportsNothingInEveryFlow(t *testing.T) {
	for _, flow := range opFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expPlainID, "")
			svc := newTestExperimentalService(t, []domain.HarnessModule{mod})

			// Act
			sum, err := runOpFlow(t, flow, svc, expPlainID, opFlags{autoConfirm: true})

			// Assert
			if err != nil {
				t.Fatalf("%s: %v", flow.name, err)
			}
			assertReported(t, sum.ExperimentalHarnesses)
		})
	}
}

// A deploy that writes several agents (subagents, a utility agent, the orchestrator pair) is
// still one operation: the warning is reported once, not once per agent.
func TestRunSummary_MultiAgentDeployToExperimentalTarget_ReportsHarnessOnce(t *testing.T) {
	// Arrange
	mod := newTestExperimentalModule(t, expHarnessID, expReason)
	svc := newTestExperimentalService(t, []domain.HarnessModule{mod})

	// Act
	sum, err := opFlows[0].run(svc, expHarnessID, t.TempDir(), opFlags{autoConfirm: true})

	// Assert
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	agents := 0
	for _, a := range sum.Actions {
		if a.Ref.Kind == domain.ArtifactAgent {
			agents++
		}
	}
	if agents < 2 {
		t.Fatalf("setup: run wrote %d agents, want a multi-agent run", agents)
	}
	assertReported(t, sum.ExperimentalHarnesses, mod.Ref())
}

// Only the harness the run involves is reported, even when the registry holds another
// experimental harness.
func TestRunSummary_OtherExperimentalHarnessInRegistry_IsNotReported(t *testing.T) {
	// Arrange
	target := newTestExperimentalModule(t, expPlainID, "")
	other := newTestExperimentalModuleOn(t, hClaude, expHarnessID2, expReason2)
	svc := newTestExperimentalService(t, []domain.HarnessModule{target, other})

	// Act
	sum, err := opFlows[0].run(svc, expPlainID, t.TempDir(), opFlags{autoConfirm: true})

	// Assert
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	assertReported(t, sum.ExperimentalHarnesses)
}

func TestRunSummary_ExperimentalTarget_DoesNotChangeOutcomeTodosOrNotices(t *testing.T) {
	// Arrange
	plain := newTestExperimentalModule(t, expHarnessID, "")
	exp := newTestExperimentalModule(t, expHarnessID, expReason)
	stub := interactiontest.NewBuilder().Build()
	svcPlain := newTestExperimentalService(t, []domain.HarnessModule{plain}, withInteraction(stub))
	stubExp := interactiontest.NewBuilder().Build()
	svcExp := newTestExperimentalService(t, []domain.HarnessModule{exp}, withInteraction(stubExp))

	// Act
	want, err1 := opFlows[0].run(svcPlain, expHarnessID, t.TempDir(), opFlags{autoConfirm: true})
	got, err2 := opFlows[0].run(svcExp, expHarnessID, t.TempDir(), opFlags{autoConfirm: true})

	// Assert
	if err1 != nil || err2 != nil {
		t.Fatalf("deploy errors: plain %v, experimental %v", err1, err2)
	}
	assertReported(t, got.ExperimentalHarnesses, exp.Ref())
	if got.Outcome != want.Outcome {
		t.Errorf("Outcome = %q, want %q (same as without the declaration)", got.Outcome, want.Outcome)
	}
	if len(got.Todos) != len(want.Todos) {
		t.Errorf("run has %d todo items, want %d: the warning must not be recorded as a gap",
			len(got.Todos), len(want.Todos))
	}
	if n := len(stubExp.Notices()); n != len(stub.Notices()) {
		t.Errorf("app sent %d notices, want %d: the warning must not travel through Notify", n, len(stub.Notices()))
	}
}

// newExperimentalPromoteDeps returns promote dependencies whose registry holds one harness,
// expHarnessID, declared experimental with the given reason (empty: not experimental).
func newExperimentalPromoteDeps(t *testing.T, reason string) app.Deps {
	t.Helper()
	deps, _ := newCodexPromoteDeps(t, interactiontest.NewBuilder().Build(), writeMosaicRoot(t))
	mod := newCodexPromoteModule()
	mod.ref = domain.HarnessRef{ID: expHarnessID, DisplayName: "Display " + expHarnessID,
		Usable: true, ExperimentalReason: reason}
	deps.Registry = &stubRegistry{list: []domain.HarnessRef{mod.ref},
		modules: map[string]domain.HarnessModule{expHarnessID: mod}}
	return deps
}

// promoteResultFor promotes an eligible harness-only file interpreted against a harness with
// the given reason (empty: not experimental).
func promoteResultFor(t *testing.T, reason string, dryRun bool) app.PromoteResult {
	t.Helper()
	src := writeEligibleCodexTomlFile(t, t.TempDir(), "my-codex-agent.toml")
	deps := newExperimentalPromoteDeps(t, reason)

	res, err := app.New(deps).Promote(context.Background(), app.PromoteRequest{
		FilePath: src, Category: "TestCategory", HarnessID: expHarnessID, DryRun: dryRun})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	return res
}

func TestPromoteResult_ExperimentalHarness_ReportsHarnessOnce(t *testing.T) {
	// Act
	res := promoteResultFor(t, expReason, false)

	// Assert
	assertReported(t, res.ExperimentalHarnesses, domain.HarnessRef{
		ID: expHarnessID, DisplayName: "Display " + expHarnessID, ExperimentalReason: expReason})
}

func TestPromoteResult_DryRunWithExperimentalHarness_ReportsHarness(t *testing.T) {
	// Act
	res := promoteResultFor(t, expReason, true)

	// Assert
	if !res.DryRun {
		t.Fatal("setup: result is not a dry run")
	}
	assertReported(t, res.ExperimentalHarnesses, domain.HarnessRef{
		ID: expHarnessID, DisplayName: "Display " + expHarnessID, ExperimentalReason: expReason})
}

func TestPromoteResult_PlainHarness_ReportsNothing(t *testing.T) {
	// Act
	res := promoteResultFor(t, "", false)

	// Assert
	assertReported(t, res.ExperimentalHarnesses)
}

// transformResult deploys with src and transforms to tgt, returning the result.
func transformResult(t *testing.T, src, tgt domain.HarnessModule, dryRun bool) app.TransformHarnessResult {
	t.Helper()
	svc := newTestExperimentalService(t, []domain.HarnessModule{src, tgt})
	ws := seedWorkspace(t, svc, src.Ref().ID)
	res, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: src.Ref().ID, TargetHarnessID: tgt.Ref().ID, Workspace: ws,
		TargetModel: parityModel, DryRun: dryRun})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	return res
}

func TestTransformResult_ExperimentalTarget_ReportsTargetOnce(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expPlainID, "")
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID, expReason)

	// Act
	res := transformResult(t, src, tgt, false)

	// Assert
	assertReported(t, res.ExperimentalHarnesses, tgt.Ref())
}

func TestTransformResult_ExperimentalSourceOnly_ReportsSource(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expPlainID, "")

	// Act
	res := transformResult(t, src, tgt, false)

	// Assert
	assertReported(t, res.ExperimentalHarnesses, src.Ref())
}

func TestTransformResult_BothExperimental_ReportsSourceThenTarget(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID2, expReason2)

	// Act
	res := transformResult(t, src, tgt, false)

	// Assert
	assertReported(t, res.ExperimentalHarnesses, src.Ref(), tgt.Ref())
}

func TestTransformResult_DryRunWithExperimentalSource_ReportsSource(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expPlainID, "")

	// Act
	res := transformResult(t, src, tgt, true)

	// Assert
	if !res.DryRun {
		t.Fatal("setup: result is not a dry run")
	}
	assertReported(t, res.ExperimentalHarnesses, src.Ref())
}

func TestTransformResult_PlainHarnesses_ReportsNothing(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expPlainID, "")
	tgt := newTestExperimentalModuleOn(t, hClaude, "other-plain", "")

	// Act
	res := transformResult(t, src, tgt, false)

	// Assert
	assertReported(t, res.ExperimentalHarnesses)
}
