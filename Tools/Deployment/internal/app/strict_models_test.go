package app_test

// strict_models_test.go verifies the strict-models mode of Service.DeployNew.
//
// Under DeployRequest.StrictModels, a run whose built plan carries a no-model gap fails with
// *UnresolvedModelsError before review and before the executor is reached, for real runs and
// dry runs. The error names every affected agent and its tier. Without the option, or when
// no no-model gap exists, the run behaves exactly as before.
//
// The stub planner returns a pre-built plan, so the tests pin the decision to Plan.Gaps of
// kind no-model and the tier lookup to the catalog's agents.

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// countingExecutor records how often Execute is reached.
type countingExecutor struct {
	inner stubExecutor
	calls int
}

func (e *countingExecutor) Execute(ctx context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.calls++
	return e.inner.Execute(ctx, req)
}

// strictFixture bundles the collaborators of one strict-models scenario.
type strictFixture struct {
	svc       app.Service
	stub      *interactiontest.Stub
	spy       *spyTodo
	exec      *countingExecutor
	workspace string
}

// newTestStrictFixture builds a service whose catalog holds agents "test-runner" (tier HIGH),
// "plan-review" (tier LOW) and an orchestrator that declares no tier, and whose planner
// returns a plan carrying the given gaps.
func newTestStrictFixture(t *testing.T, gaps []domain.Gap) *strictFixture {
	t.Helper()
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newBaseDeps(t, stub)

	cat := newMinimalCatalog()
	cat.agents = []domain.Agent{
		minimalAgent,
		{Key: "plan-review", Version: "1.0", Name: "Plan Review", Role: domain.RoleWorker, RecommendedTier: "LOW"},
	}
	deps.Catalog = cat

	plan := newMinimalPlan(workspace)
	plan.Gaps = gaps
	deps.Planner = &stubPlanner{plan: plan}

	spy := &spyTodo{}
	deps.Todo = spy
	exec := &countingExecutor{inner: stubExecutor{result: newMinimalExecResult(workspace)}}
	deps.Executor = exec

	return &strictFixture{svc: app.New(deps), stub: stub, spy: spy, exec: exec, workspace: workspace}
}

func noModelGap(agentKey string) domain.Gap {
	return domain.Gap{Kind: domain.GapNoModel, Subject: agentKey, Detail: "no model has been selected for agent " + agentKey}
}

func strictRequest(workspace string, dryRun bool) app.DeployRequest {
	req := newDeployRequest(workspace, "stub-harness", []string{"quick-fix"})
	req.StrictModels = true
	req.DryRun = dryRun
	return req
}

// requireUnresolvedModels asserts err is a *UnresolvedModelsError and returns it.
func requireUnresolvedModels(t *testing.T, err error) *app.UnresolvedModelsError {
	t.Helper()
	if !errors.Is(err, app.ErrUnresolvedModels) {
		t.Fatalf("error = %v, want one matching errors.Is(err, ErrUnresolvedModels)", err)
	}
	var ume *app.UnresolvedModelsError
	if !errors.As(err, &ume) {
		t.Fatalf("error = %T %v, want *UnresolvedModelsError", err, err)
	}
	return ume
}

// ---------------------------------------------------------------------------
// Strict mode: unresolved model fails the run
// ---------------------------------------------------------------------------

// TestDeployNew_StrictModels_UnresolvedAgent_FailsNamingAgentAndTier verifies a real run with
// one no-model gap fails with the typed error naming that agent and its tier.
func TestDeployNew_StrictModels_UnresolvedAgent_FailsNamingAgentAndTier(t *testing.T) {
	f := newTestStrictFixture(t, []domain.Gap{noModelGap("test-runner")})

	_, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, false))

	ume := requireUnresolvedModels(t, err)
	if len(ume.Agents) != 1 || ume.Agents[0].AgentKey != "test-runner" || ume.Agents[0].Tier != "HIGH" {
		t.Errorf("Agents = %+v, want exactly [{test-runner HIGH}]", ume.Agents)
	}
	msg := err.Error()
	if !strings.Contains(msg, "test-runner") || !strings.Contains(msg, "HIGH") {
		t.Errorf("error text %q must name the agent \"test-runner\" and its tier \"HIGH\"", msg)
	}
}

// TestDeployNew_StrictModels_DryRun_FailsNamingAgentAndTier verifies the same failure on a
// dry run.
func TestDeployNew_StrictModels_DryRun_FailsNamingAgentAndTier(t *testing.T) {
	f := newTestStrictFixture(t, []domain.Gap{noModelGap("test-runner")})

	_, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, true))

	ume := requireUnresolvedModels(t, err)
	if len(ume.Agents) != 1 || ume.Agents[0].AgentKey != "test-runner" || ume.Agents[0].Tier != "HIGH" {
		t.Errorf("Agents = %+v, want exactly [{test-runner HIGH}]", ume.Agents)
	}
	if msg := err.Error(); !strings.Contains(msg, "test-runner") || !strings.Contains(msg, "HIGH") {
		t.Errorf("error text %q must name the agent \"test-runner\" and its tier \"HIGH\"", msg)
	}
}

// TestDeployNew_StrictModels_UnresolvedAgent_NothingWrittenAndNoReview verifies the decision
// precedes review and execution: the executor is never reached and the plan is never shown.
func TestDeployNew_StrictModels_UnresolvedAgent_NothingWrittenAndNoReview(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "real"
		if dryRun {
			name = "dry-run"
		}
		t.Run(name, func(t *testing.T) {
			f := newTestStrictFixture(t, []domain.Gap{noModelGap("test-runner")})

			_, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, dryRun))

			requireUnresolvedModels(t, err)
			if f.exec.calls != 0 {
				t.Errorf("executor reached %d time(s), want 0: a strict failure must precede any write", f.exec.calls)
			}
			if n := f.stub.CountAsked(domain.QPlanConfirm, ""); n != 0 {
				t.Errorf("plan review shown %d time(s), want 0: a strict failure must precede review", n)
			}
			// The TODO file is written by the executor from the collected items; nothing may
			// be recorded for it and nothing may appear in the workspace.
			if len(f.spy.items) != 0 {
				t.Errorf("TODO items recorded = %+v, want none on a strict failure", f.spy.items)
			}
			entries, rerr := os.ReadDir(f.workspace)
			if rerr != nil {
				t.Fatalf("ReadDir workspace: %v", rerr)
			}
			if len(entries) != 0 {
				t.Errorf("workspace holds %d entr(ies) after a strict failure, want none (no agent, manifest or TODO file)", len(entries))
			}
		})
	}
}

// TestDeployNew_StrictModels_ReturnsZeroSummary verifies the failure returns the zero-value
// summary, as no conflict report has been computed at that point.
func TestDeployNew_StrictModels_ReturnsZeroSummary(t *testing.T) {
	f := newTestStrictFixture(t, []domain.Gap{noModelGap("test-runner")})

	summary, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, false))

	requireUnresolvedModels(t, err)
	if summary.Outcome != "" || summary.WorkspacePath != "" || len(summary.Actions) != 0 {
		t.Errorf("summary = %+v, want the zero RunSummary", summary)
	}
}

// TestDeployNew_StrictModels_MultipleUnresolved_NamesEveryAgentSortedByKey verifies every
// unresolved agent is named, ordered by key, with an empty tier written as <none>.
func TestDeployNew_StrictModels_MultipleUnresolved_NamesEveryAgentSortedByKey(t *testing.T) {
	// Gaps are given out of key order on purpose.
	f := newTestStrictFixture(t, []domain.Gap{
		noModelGap("test-runner"),
		noModelGap("orchestrator"),
		noModelGap("plan-review"),
	})

	_, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, false))

	ume := requireUnresolvedModels(t, err)
	want := []app.UnresolvedAgentModel{
		{AgentKey: "orchestrator", Tier: ""},
		{AgentKey: "plan-review", Tier: "LOW"},
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
	msg := err.Error()
	for _, frag := range []string{"orchestrator", "plan-review", "LOW", "test-runner", "HIGH", "<none>"} {
		if !strings.Contains(msg, frag) {
			t.Errorf("error text %q does not contain %q", msg, frag)
		}
	}
}

// TestDeployNew_StrictModels_OnlyNoModelGapsDecide verifies that gaps of other kinds do not
// name an agent: only the agent with a no-model gap is reported.
func TestDeployNew_StrictModels_OnlyNoModelGapsDecide(t *testing.T) {
	f := newTestStrictFixture(t, []domain.Gap{
		{Kind: domain.GapSkippedFile, Subject: "plan-review", Detail: "skipped"},
		noModelGap("test-runner"),
	})

	_, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, false))

	ume := requireUnresolvedModels(t, err)
	if len(ume.Agents) != 1 || ume.Agents[0].AgentKey != "test-runner" {
		t.Errorf("Agents = %+v, want only test-runner (plan-review has no no-model gap)", ume.Agents)
	}
}

// TestDeployNew_StrictModels_OtherGapKindsOnly_Succeeds verifies a plan whose only gaps are
// not no-model gaps does not trip strict mode.
func TestDeployNew_StrictModels_OtherGapKindsOnly_Succeeds(t *testing.T) {
	f := newTestStrictFixture(t, []domain.Gap{
		{Kind: domain.GapSkippedFile, Subject: "plan-review", Detail: "skipped"},
	})

	_, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, false))

	if err != nil {
		t.Fatalf("DeployNew: %v, want success: only no-model gaps are fatal under strict mode", err)
	}
	if f.exec.calls != 1 {
		t.Errorf("executor reached %d time(s), want 1", f.exec.calls)
	}
}

// ---------------------------------------------------------------------------
// Strict mode: everything resolved behaves as today
// ---------------------------------------------------------------------------

// TestDeployNew_StrictModels_AllResolved_SucceedsAsToday verifies a strict run without any
// gap succeeds, executes, and produces the same outcome as the same run without strict mode.
func TestDeployNew_StrictModels_AllResolved_SucceedsAsToday(t *testing.T) {
	strict := newTestStrictFixture(t, nil)
	plain := newTestStrictFixture(t, nil)

	strictSummary, strictErr := strict.svc.DeployNew(context.Background(), strictRequest(strict.workspace, false))
	plainReq := newDeployRequest(plain.workspace, "stub-harness", []string{"quick-fix"})
	plainSummary, plainErr := plain.svc.DeployNew(context.Background(), plainReq)

	if strictErr != nil || plainErr != nil {
		t.Fatalf("DeployNew errors: strict=%v plain=%v, want both nil", strictErr, plainErr)
	}
	if strictSummary.Outcome != plainSummary.Outcome {
		t.Errorf("strict outcome = %q, plain outcome = %q, want equal", strictSummary.Outcome, plainSummary.Outcome)
	}
	if strict.exec.calls != 1 {
		t.Errorf("executor reached %d time(s) under strict mode, want 1", strict.exec.calls)
	}
}

// TestDeployNew_StrictModels_AllResolved_DryRunSucceeds verifies the dry-run counterpart.
func TestDeployNew_StrictModels_AllResolved_DryRunSucceeds(t *testing.T) {
	f := newTestStrictFixture(t, nil)

	_, err := f.svc.DeployNew(context.Background(), strictRequest(f.workspace, true))

	if err != nil {
		t.Fatalf("DeployNew dry run: %v, want success when every model is resolved", err)
	}
}

// ---------------------------------------------------------------------------
// Default mode: unchanged
// ---------------------------------------------------------------------------

// TestDeployNew_NoStrictModels_UnresolvedAgent_GapGoesToTodoAndRunSucceeds is the regression
// test for the default behaviour: the same no-model plan without StrictModels records the
// gap in the TODO collector and still executes without error.
func TestDeployNew_NoStrictModels_UnresolvedAgent_GapGoesToTodoAndRunSucceeds(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "real"
		if dryRun {
			name = "dry-run"
		}
		t.Run(name, func(t *testing.T) {
			f := newTestStrictFixture(t, []domain.Gap{noModelGap("test-runner")})
			req := newDeployRequest(f.workspace, "stub-harness", []string{"quick-fix"})
			req.DryRun = dryRun

			_, err := f.svc.DeployNew(context.Background(), req)

			if err != nil {
				t.Fatalf("DeployNew: %v, want success without strict mode", err)
			}
			if n := countNoModelGaps(f.spy, "test-runner"); n != 1 {
				t.Errorf("collected %d no-model gaps for \"test-runner\", want exactly 1", n)
			}
			if f.exec.calls != 1 {
				t.Errorf("executor reached %d time(s), want 1", f.exec.calls)
			}
		})
	}
}
