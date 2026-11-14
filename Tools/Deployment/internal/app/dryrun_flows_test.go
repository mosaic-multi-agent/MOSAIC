package app_test

// dryrun_flows_test.go verifies the dry run of the five write flows (deploy, update, workflows,
// agents, hooks) as a non-interactive caller sees it: no plan confirmation is given, the review
// still happens, nothing is written, and the summary carries the reviewed plan. A real run
// without confirmation keeps refusing.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// drReview is an Interaction that keeps the plans shown for review and answers the review the
// way a non-interactive caller without confirmation does: not confirmed.
type drReview struct {
	*interactiontest.Stub
	answer domain.ConfirmAnswer
	plans  []domain.Plan
}

func newDrReview(status domain.AnswerStatus) *drReview {
	return &drReview{Stub: interactiontest.NewBuilder().Build(), answer: domain.ConfirmAnswer{Status: status}}
}

// drUnconfirmed are the review answers that are not a confirmation: a skipped question (what a
// non-interactive caller without confirmation gets), a cancel and an explicit decline.
var drUnconfirmed = []struct {
	name   string
	status domain.AnswerStatus
}{
	{"skipped", domain.SkippedOne},
	{"cancelled", domain.Cancelled},
	{"declined", domain.Answered},
}

func (r *drReview) Review(_ context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	r.plans = append(r.plans, p)
	return r.answer, nil
}

// drPlan is a plan with a create, an update with a reason, a removal and a replacement.
func drPlan(ws string) domain.Plan {
	p := newMinimalPlan(ws)
	p.Items = append(p.Items, domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "other-agent"},
		TargetPath: "other-agent.md", Action: domain.ActionUpdate, Reasons: []string{"version changed from \"1.0\" to \"2.0\""}})
	p.Removals = []domain.Removal{{Path: "legacy.md", DropEntry: true, Reason: "replaced by test-runner.md"}}
	p.Replacements = []domain.Replacement{{Path: "test-runner.md", Reason: "existing file is replaced by the migrated file"}}
	return p
}

// drDeps returns deps for crOperations over the plan drPlan, with the given review answer and a
// spy executor.
func drDeps(t *testing.T, r *drReview) (app.Deps, string, *crSpyExecutor) {
	t.Helper()
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
	crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")
	deps.Planner = &stubPlanner{plan: drPlan(ws)}
	deps.Interaction = r
	spy := &crSpyExecutor{}
	deps.Executor = spy
	return deps, ws, spy
}

// TestDryRun_WithoutConfirmation_SucceedsAndWritesNothing verifies a dry run needs no plan
// confirmation on any write flow, still shows the plan for review, and changes nothing.
func TestDryRun_WithoutConfirmation_SucceedsAndWritesNothing(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			r := newDrReview(domain.SkippedOne)
			deps, ws, spy := drDeps(t, r)
			before := crSnapshotDir(t, ws)

			_, err := op.run(context.Background(), app.New(deps), ws, crFlags{dryRun: true})

			if err != nil {
				t.Fatalf("%s: dry run without confirmation failed: %v", op.name, err)
			}
			if len(r.plans) != 1 {
				t.Errorf("plan was reviewed %d times, want once", len(r.plans))
			}
			if spy.called && !spy.dryRun {
				t.Error("executor was asked for a real run")
			}
			if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
				t.Error("a dry run changed the workspace")
			}
		})
	}
}

// TestDryRun_CarriesTheReviewedPlanOnTheSummary verifies the summary holds the plan that was
// reviewed, with its items, removals and replacements.
func TestDryRun_CarriesTheReviewedPlanOnTheSummary(t *testing.T) {
	for _, op := range crOperations() {
		for _, autoConfirm := range []bool{false, true} {
			name := op.name + "/without-confirmation"
			if autoConfirm {
				name = op.name + "/auto-confirm"
			}
			t.Run(name, func(t *testing.T) {
				r := newDrReview(domain.SkippedOne)
				deps, ws, _ := drDeps(t, r)

				summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{dryRun: true, autoConfirm: autoConfirm})

				if err != nil {
					t.Fatalf("%s: %v", op.name, err)
				}
				if summary.DryRunPlan == nil {
					t.Fatalf("%s: summary carries no dry-run plan", op.name)
				}
				if len(r.plans) != 1 {
					t.Fatalf("plan was reviewed %d times, want once", len(r.plans))
				}
				got, want := *summary.DryRunPlan, r.plans[0]
				if !reflect.DeepEqual(got.Items, want.Items) || !reflect.DeepEqual(got.Removals, want.Removals) ||
					!reflect.DeepEqual(got.Replacements, want.Replacements) {
					t.Errorf("dry-run plan differs from the reviewed plan:\n got %+v\nwant %+v", got, want)
				}
				if len(got.Items) == 0 {
					t.Error("dry-run plan has no items")
				}
				if len(got.Removals) != 1 || got.Removals[0].Path != "legacy.md" || got.Removals[0].Reason == "" {
					t.Errorf("removals = %+v, want the planned removal with its reason", got.Removals)
				}
				if len(got.Replacements) != 1 || got.Replacements[0].Path != "test-runner.md" || got.Replacements[0].Reason == "" {
					t.Errorf("replacements = %+v, want the planned replacement with its reason", got.Replacements)
				}
			})
		}
	}
}

// TestDryRun_ReviewDeclined_StillSucceeds verifies the refusal belongs to real runs only: a
// dry run whose review answer is a cancel or a decline is not an error.
func TestDryRun_ReviewDeclined_StillSucceeds(t *testing.T) {
	for _, op := range crOperations() {
		for _, status := range drUnconfirmed {
			t.Run(op.name+"/"+status.name, func(t *testing.T) {
				r := newDrReview(status.status)
				deps, ws, _ := drDeps(t, r)

				summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{dryRun: true})

				if err != nil {
					t.Fatalf("%s: dry run failed on an unconfirmed review: %v", op.name, err)
				}
				if summary.DryRunPlan == nil {
					t.Error("summary carries no dry-run plan")
				}
			})
		}
	}
}

// TestRealRun_WithoutConfirmation_StillRefuses verifies a run that is not a dry run and has
// no confirmation is refused before anything is written, and carries no dry-run plan.
func TestRealRun_WithoutConfirmation_StillRefuses(t *testing.T) {
	for _, op := range crOperations() {
		for _, status := range drUnconfirmed {
			t.Run(op.name+"/"+status.name, func(t *testing.T) {
				r := newDrReview(status.status)
				deps, ws, spy := drDeps(t, r)
				before := crSnapshotDir(t, ws)

				summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{})

				if !errors.Is(err, app.ErrPlanNotConfirmed) {
					t.Fatalf("%s: err = %v, want ErrPlanNotConfirmed", op.name, err)
				}
				if spy.called {
					t.Error("executor ran although the plan was not confirmed")
				}
				if summary.DryRunPlan != nil {
					t.Error("a refused real run carries a dry-run plan")
				}
				if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
					t.Error("a refused run changed the workspace")
				}
			})
		}
	}
}

// TestRealRun_Confirmed_CarriesNoDryRunPlan verifies only dry runs carry the plan.
func TestRealRun_Confirmed_CarriesNoDryRunPlan(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			r := newDrReview(domain.SkippedOne)
			deps, ws, _ := drDeps(t, r)

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			if summary.DryRunPlan != nil {
				t.Error("a real run carries a dry-run plan")
			}
		})
	}
}
