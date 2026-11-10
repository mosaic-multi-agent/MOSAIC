package app_test

// conflict_report_plan_test.go verifies that every write operation (deploy, update, workflows,
// agents, hooks) hands the plan reviewer a plan that already carries the run's conflict report,
// the same report the run summary carries, and that a run whose plan the reviewer declines after
// seeing that report writes nothing. The report is empty when nothing is shadowed.

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
)

// crReviewSpy records the plans handed to Review and delegates the answer to the stub.
type crReviewSpy struct {
	*interactiontest.Stub
	plans []domain.Plan
}

func (r *crReviewSpy) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	r.plans = append(r.plans, p)
	return r.Stub.Review(ctx, p)
}

// onlyReviewedPlan returns the single plan the reviewer received.
func onlyReviewedPlan(t *testing.T, spy *crReviewSpy) domain.Plan {
	t.Helper()
	if len(spy.plans) != 1 {
		t.Fatalf("the reviewer received %d plans, want exactly one", len(spy.plans))
	}
	return spy.plans[0]
}

// TestPlanReview_EveryWritePath_PlanCarriesTheRunsConflictReport verifies that the plan the
// reviewer receives holds the shadowed-agent warning and equals the report of the run summary.
func TestPlanReview_EveryWritePath_PlanCarriesTheRunsConflictReport(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
			spy := &crReviewSpy{Stub: interactiontest.NewBuilder().Build()}
			deps.Interaction = spy

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			plan := onlyReviewedPlan(t, spy)
			line := crLineFor(t, plan.ConflictReport, crAgentItem())
			if line.Severity != domain.ConflictWarning || line.Winner.ID != crUserID {
				t.Errorf("plan line = %+v, want a warning won by %q", line, crUserID)
			}
			if !reflect.DeepEqual(plan.ConflictReport, summary.ConflictReport) {
				t.Errorf("plan report = %+v, want the run summary report %+v", plan.ConflictReport, summary.ConflictReport)
			}
		})
	}
}

// TestPlanReview_DeclaredOverride_PlanLineIsInfo verifies that a declared override reaches the
// reviewer with info severity.
func TestPlanReview_DeclaredOverride_PlanLineIsInfo(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{declareAgent: true})
			spy := &crReviewSpy{Stub: interactiontest.NewBuilder().Build()}
			deps.Interaction = spy

			if _, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true}); err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}

			line := crLineFor(t, onlyReviewedPlan(t, spy).ConflictReport, crAgentItem())
			if line.Severity != domain.ConflictInfo {
				t.Errorf("plan severity = %q, want %q", line.Severity, domain.ConflictInfo)
			}
		})
	}
}

// TestPlanReview_PlanReportNamesTheRunsWorkflows verifies that the affected workflows are on the
// plan's line already at review time, including those only the deployed orchestrator embeds.
func TestPlanReview_PlanReportNamesTheRunsWorkflows(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{
				workflows: []domain.Workflow{crWorkflow("extra-flow", crAgentKey)},
			})
			crWriteDeployed(t, ws, "orchestrator.md", "extra-flow")
			spy := &crReviewSpy{Stub: interactiontest.NewBuilder().Build()}
			deps.Interaction = spy

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			got := crLineFor(t, onlyReviewedPlan(t, spy).ConflictReport, crAgentItem()).AffectedWorkflows
			want := crLineFor(t, summary.ConflictReport, crAgentItem()).AffectedWorkflows
			if len(got) == 0 || !reflect.DeepEqual(got, want) {
				t.Errorf("plan AffectedWorkflows = %v, want the summary's %v (non-empty)", got, want)
			}
		})
	}
}

// TestPlanReview_NothingShadowed_PlanReportIsEmpty verifies that a single-layer stack and a plain
// catalog give the reviewer a plan with an empty report.
func TestPlanReview_NothingShadowed_PlanReportIsEmpty(t *testing.T) {
	setups := map[string]func(t *testing.T) (app.Deps, string){
		"default-only stack": func(t *testing.T) (app.Deps, string) {
			deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
			mosaic := newSysMosaicRoot(t, sysLayer{id: crDefaultID, protocolVersion: "1.0", bundleVersion: "1.0.0"})
			crWriteAgent(t, filepath.Join(mosaic, catalogpaths.CatalogDirName), crAgentKey, "1.0")
			useResolvedStack(t, &deps, mosaic, defaultStackLayer(mosaic))
			return deps, ws
		},
		"plain catalog": func(t *testing.T) (app.Deps, string) {
			return newBaseDeps(t, interactiontest.NewBuilder().Build())
		},
	}
	for name, setup := range setups {
		for _, op := range crOperations() {
			t.Run(name+"/"+op.name, func(t *testing.T) {
				deps, ws := setup(t)
				spy := &crReviewSpy{Stub: interactiontest.NewBuilder().Build()}
				deps.Interaction = spy

				if _, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true}); err != nil {
					t.Fatalf("%s: %v", op.name, err)
				}

				if got := onlyReviewedPlan(t, spy).ConflictReport; len(got.Lines) != 0 {
					t.Errorf("plan report = %+v, want empty", got)
				}
			})
		}
	}
}

// TestPlanReview_DeclinedAfterTheReport_WritesNothing verifies that when the reviewer sees the
// report and declines, the run ends with ErrPlanNotConfirmed, the executor never runs and the
// workspace is byte-identical.
func TestPlanReview_DeclinedAfterTheReport_WritesNothing(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")
			spy := &crReviewSpy{Stub: interactiontest.NewBuilder().AnswerConfirm(domain.QPlanConfirm, "", false).Build()}
			deps.Interaction = spy
			exec := &crSpyExecutor{}
			deps.Executor = exec
			before := crSnapshotDir(t, ws)

			_, err := op.run(context.Background(), app.New(deps), ws, crFlags{})

			if !errors.Is(err, app.ErrPlanNotConfirmed) {
				t.Fatalf("err = %v, want ErrPlanNotConfirmed", err)
			}
			crLineFor(t, onlyReviewedPlan(t, spy).ConflictReport, crAgentItem())
			if exec.called {
				t.Error("executor ran although the plan was declined after the report")
			}
			if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
				t.Error("declining after the report changed the workspace")
			}
		})
	}
}
