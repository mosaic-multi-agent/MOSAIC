package app_test

// thirdformat_parity_helpers_test.go holds the twin parity harness: a pair of services, one
// over the test-only agent format and one over Markdown, each with its own workspace, and the
// functions that run one flow step on both sides and hand the observations to the comparison
// (thirdformat_parity_compare_test.go). Observation building lives in
// thirdformat_parity_observe_test.go; hand-edit and flow-step helpers in
// thirdformat_parity_steps_test.go.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// twinPair holds both twins, each with its own workspace and service.
type twinPair struct {
	Test, Markdown twinSide
}

type twinSide struct {
	Format       twinFormat
	Module       domain.HarnessModule
	Counterparts []domain.HarnessModule // registered besides the twin; nil from newTestTwinPair
	Workspace    string
	Service      app.Service
	Planner      *twinPlanRecorder // the service's planner; records the plan of the last run
}

// name labels the side in failure messages.
func (s twinSide) name() string {
	if s.Format == twinTestFormat {
		return "test format"
	}
	return "markdown"
}

// twinPlanRecorder is the service planner of a twin side. It delegates every Build to the real
// planner unchanged and records the plan that call returned.
type twinPlanRecorder struct {
	inner plan.Planner
	last  domain.Plan
}

func (p *twinPlanRecorder) Build(ctx context.Context, in plan.Input) (domain.Plan, error) {
	out, err := p.inner.Build(ctx, in)
	p.last = out
	return out, err
}

// Last returns the plan returned by the most recent Build call, or the zero Plan when Build has
// not been called since the recorder was created or reset.
func (p *twinPlanRecorder) Last() domain.Plan { return p.last }

func (p *twinPlanRecorder) reset() { p.last = domain.Plan{} }

// newTestTwinPair builds both sides with fresh workspaces.
func newTestTwinPair(t *testing.T) twinPair { return newTestTwinPairWith(t, nil) }

// newTestTwinPairWith builds both sides with fresh workspaces. counterparts are registered in
// both sides' services and recorded in twinSide.Counterparts. sideOpts are called once per side
// with that side's format and workspace; the Deps options they return apply to that side only.
func newTestTwinPairWith(t *testing.T, counterparts []domain.HarnessModule,
	sideOpts ...func(f twinFormat, workspace string) func(*app.Deps)) twinPair {
	t.Helper()
	build := func(f twinFormat) twinSide {
		mod := newTestTwinHarness(t, f)
		ws := t.TempDir()
		rec := &twinPlanRecorder{}
		install := func(d *app.Deps) { rec.inner, d.Planner = d.Planner, rec }
		opts := []func(*app.Deps){withTwinRegistryModules(counterparts...)}
		for _, so := range sideOpts {
			if o := so(f, ws); o != nil {
				opts = append(opts, o)
			}
		}
		opts = append(opts, install)
		return twinSide{Format: f, Module: mod, Counterparts: counterparts, Workspace: ws,
			Service: newTestTwinService(t, mod, opts...), Planner: rec}
	}
	return twinPair{Test: build(twinTestFormat), Markdown: build(twinMarkdown)}
}

// twinStep runs one flow step on one side and returns its run summary.
type twinStep func(t *testing.T, side twinSide) (domain.RunSummary, error)

// runTwinStep runs step on both sides and returns their observations. A step error on either
// side fails the test.
func runTwinStep(t *testing.T, p twinPair, step twinStep) (test, markdown twinObservation) {
	t.Helper()
	run := func(side twinSide) twinObservation {
		t.Helper()
		side.Planner.reset()
		sum, err := step(t, side)
		if err != nil {
			t.Fatalf("flow step on the %s twin: %v", side.name(), err)
		}
		obs := observeTwinSide(t, side)
		obs.Items = twinPlanItems(side, side.Planner.Last())
		obs.Actions = twinActions(sum)
		obs.OwnedKeyDifferences = sum.OwnedKeyDifferences
		return obs
	}
	return run(p.Test), run(p.Markdown)
}

// twinFlowResult is the report of a flow step that does not return a RunSummary. Exactly one of
// Transform and Promote is set.
type twinFlowResult struct {
	// Transform is the result of Service.TransformHarness.
	Transform *app.TransformHarnessResult
	// Promote is the result of Service.Promote.
	Promote *app.PromoteResult
	// PromotedFile is the content of the generic file Promote wrote, read by the step; nil for a
	// dry run and for transform steps.
	PromotedFile []byte
}

// twinFlowStep runs one flow step whose result is not a RunSummary on one side.
type twinFlowStep func(t *testing.T, side twinSide) (twinFlowResult, error)

// runTwinFlowStep runs step on both sides and returns their observations, with Flow set from
// the step result and Items, Actions and OwnedKeyDifferences empty. A step error on either side
// fails the test.
func runTwinFlowStep(t *testing.T, p twinPair, step twinFlowStep) (test, markdown twinObservation) {
	t.Helper()
	run := func(side twinSide) twinObservation {
		t.Helper()
		res, err := step(t, side)
		if err != nil {
			t.Fatalf("flow step on the %s twin: %v", side.name(), err)
		}
		obs := observeTwinSide(t, side)
		obs.Flow = &res
		return obs
	}
	return run(p.Test), run(p.Markdown)
}
