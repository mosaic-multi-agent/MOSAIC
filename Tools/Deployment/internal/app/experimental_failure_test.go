package app_test

// experimental_failure_test.go tests the experimental warning on the results of runs that do not
// complete cleanly. A result the app returns for the frontends to render as a document carries
// the warning for each experimental harness resolved by then: a summary with failed items, and a
// transform result for a declined plan, a cancelled selection or a reverted run. Results that
// are empty because the run stopped with a run-level error carry no warning.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// switchInteraction wraps a scripted stub whose answers can be turned into a decline or a
// cancellation after the setup of a test has run.
type switchInteraction struct {
	domain.Interaction
	declineReview bool
	cancelModel   bool
	cancelTarget  bool
}

func newSwitchInteraction() *switchInteraction {
	return &switchInteraction{Interaction: interactiontest.NewBuilder().Build()}
}

func (s *switchInteraction) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	if s.declineReview {
		return domain.ConfirmAnswer{Status: domain.Answered, Confirm: false}, nil
	}
	return s.Interaction.Review(ctx, p)
}

func (s *switchInteraction) SelectOne(ctx context.Context, q domain.ChoiceQuestion) (domain.ChoiceAnswer, error) {
	if (s.cancelModel && q.ID == domain.QTransformTargetModel) ||
		(s.cancelTarget && q.ID == domain.QTransformTargetHarness) {
		return domain.ChoiceAnswer{Status: domain.Cancelled}, nil
	}
	return s.Interaction.SelectOne(ctx, q)
}

// partialExecutor wraps the executor of a service. Once armed, every run it executes reports a
// failed write (ExecResult.Partial) after the real execution.
type partialExecutor struct {
	deploy.Executor
	armed bool
}

func (p *partialExecutor) Execute(ctx context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	res, err := p.Executor.Execute(ctx, req)
	if p.armed {
		res.Partial = errors.New("simulated write failure")
	}
	return res, err
}

// withPartialExecutor is a Deps option that installs ex around the service's executor.
func withPartialExecutor(ex *partialExecutor) func(*app.Deps) {
	return func(d *app.Deps) {
		ex.Executor = d.Executor
		d.Executor = ex
	}
}

func TestRunSummary_FailedOutcomeOfExperimentalTarget_ReportsHarnessOnce(t *testing.T) {
	for _, flow := range opFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expHarnessID, expReason)
			ex := &partialExecutor{}
			svc := newTestExperimentalService(t, []domain.HarnessModule{mod}, withPartialExecutor(ex))
			ws := seedWorkspace(t, svc, expHarnessID)
			ex.armed = true

			// Act
			sum, err := flow.run(svc, expHarnessID, ws, opFlags{autoConfirm: true})

			// Assert
			if err != nil {
				t.Fatalf("%s: %v", flow.name, err)
			}
			if sum.Outcome != domain.OutcomeFailed {
				t.Fatalf("setup: outcome = %q, want %q", sum.Outcome, domain.OutcomeFailed)
			}
			assertReported(t, sum.ExperimentalHarnesses, mod.Ref())
		})
	}
}

// A declined plan of the summary-returning operations returns the empty summary and the
// sentinel error. The plan review already showed the warning, and no result document is
// rendered, so the summary carries none.
func TestRunSummary_DeclinedPlanOfExperimentalTarget_ReturnsEmptyResultWithoutWarning(t *testing.T) {
	for _, flow := range opFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expHarnessID, expReason)
			in := newSwitchInteraction()
			svc := newTestExperimentalService(t, []domain.HarnessModule{mod}, withInteraction(in))
			ws := seedWorkspace(t, svc, expHarnessID)
			in.declineReview = true

			// Act
			sum, err := flow.run(svc, expHarnessID, ws, opFlags{autoConfirm: false})

			// Assert
			if !errors.Is(err, app.ErrPlanNotConfirmed) {
				t.Fatalf("%s: error = %v, want ErrPlanNotConfirmed", flow.name, err)
			}
			if sum.ExperimentalHarnesses != nil {
				t.Errorf("declined plan reports %+v, want no warning on the empty result", sum.ExperimentalHarnesses)
			}
		})
	}
}

func TestPromoteResult_RunLevelErrorWithExperimentalHarness_ReturnsEmptyResultWithoutWarning(t *testing.T) {
	// Arrange
	deps := newExperimentalPromoteDeps(t, expReason)

	// Act
	res, err := app.New(deps).Promote(context.Background(), app.PromoteRequest{
		FilePath: filepath.Join(t.TempDir(), "missing.toml"), Category: "TestCategory",
		HarnessID: expHarnessID})

	// Assert
	if err == nil {
		t.Fatal("Promote of a missing file returned no error")
	}
	if res.ExperimentalHarnesses != nil {
		t.Errorf("promote error result reports %+v, want no warning", res.ExperimentalHarnesses)
	}
}

// transformFailureSetup deploys with src, arms nothing yet, and returns the service, the
// switchable interaction and the workspace.
func transformFailureSetup(t *testing.T, src, tgt domain.HarnessModule) (app.Service, *switchInteraction, string) {
	t.Helper()
	in := newSwitchInteraction()
	svc := newTestExperimentalService(t, []domain.HarnessModule{src, tgt}, withInteraction(in))
	return svc, in, seedWorkspace(t, svc, src.Ref().ID)
}

func TestTransformResult_DeclinedPlan_ReportsBothExperimentalHarnesses(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID2, expReason2)
	svc, in, ws := transformFailureSetup(t, src, tgt)
	in.declineReview = true

	// Act
	res, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: expHarnessID, TargetHarnessID: expHarnessID2, Workspace: ws, TargetModel: parityModel})

	// Assert
	if !errors.Is(err, app.ErrPlanNotConfirmed) || !res.Cancelled {
		t.Fatalf("setup: error = %v, Cancelled = %v, want a declined plan", err, res.Cancelled)
	}
	assertReported(t, res.ExperimentalHarnesses, src.Ref(), tgt.Ref())
}

func TestTransformResult_DeclinedPlanWithExperimentalSourceOnly_ReportsSource(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expPlainID, "")
	svc, in, ws := transformFailureSetup(t, src, tgt)
	in.declineReview = true

	// Act
	res, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: expHarnessID, TargetHarnessID: expPlainID, Workspace: ws, TargetModel: parityModel})

	// Assert
	if !errors.Is(err, app.ErrPlanNotConfirmed) {
		t.Fatalf("setup: error = %v, want ErrPlanNotConfirmed", err)
	}
	assertReported(t, res.ExperimentalHarnesses, src.Ref())
}

func TestTransformResult_CancelledModelSelection_ReportsBothExperimentalHarnesses(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID2, expReason2)
	svc, in, ws := transformFailureSetup(t, src, tgt)
	in.cancelModel = true

	// Act: no target model is given, so the model question is asked and cancelled
	res, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: expHarnessID, TargetHarnessID: expHarnessID2, Workspace: ws})

	// Assert
	if !errors.Is(err, app.ErrModelSelectionCancelled) || !res.Cancelled {
		t.Fatalf("setup: error = %v, Cancelled = %v, want a cancelled model selection", err, res.Cancelled)
	}
	assertReported(t, res.ExperimentalHarnesses, src.Ref(), tgt.Ref())
}

// A selection cancelled before the target harness is resolved reports the source only.
func TestTransformResult_CancelledTargetSelectionWithExperimentalSource_ReportsSourceOnly(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID2, expReason2)
	svc, in, ws := transformFailureSetup(t, src, tgt)
	in.cancelTarget = true

	// Act
	res, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: expHarnessID, Workspace: ws, TargetModel: parityModel})

	// Assert
	if !errors.Is(err, app.ErrSelectionCancelled) {
		t.Fatalf("setup: error = %v, want ErrSelectionCancelled", err)
	}
	assertReported(t, res.ExperimentalHarnesses, src.Ref())
}

func TestTransformResult_RevertedRun_ReportsBothExperimentalHarnesses(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expHarnessID, expReason)
	tgt := newTestExperimentalModuleOn(t, hClaude, expHarnessID2, expReason2)
	svc, _, ws := transformFailureSetup(t, src, tgt)
	// A non-empty folder where a target agent file must be written cannot be overwritten.
	blocker := filepath.Join(ws, filepath.FromSlash(tgt.Descriptor().Paths.Agents.Project),
		owRunner+tgt.Descriptor().Extensions[domain.ArtifactAgent], "keep.txt")
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatalf("setup blocker: %v", err)
	}
	if err := os.WriteFile(blocker, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("setup blocker: %v", err)
	}

	// Act
	res, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: expHarnessID, TargetHarnessID: expHarnessID2, Workspace: ws,
		TargetModel: parityModel, Overwrite: true})

	// Assert
	if !errors.Is(err, app.ErrRunReverted) {
		t.Fatalf("setup: error = %v, want ErrRunReverted", err)
	}
	assertReported(t, res.ExperimentalHarnesses, src.Ref(), tgt.Ref())
}

// A transform that stops before a run exists returns the zero result, without a warning.
func TestTransformResult_UnresolvableSourceHarness_ReturnsEmptyResultWithoutWarning(t *testing.T) {
	// Arrange
	tgt := newTestExperimentalModule(t, expHarnessID, expReason)
	svc := newTestExperimentalService(t, []domain.HarnessModule{tgt})

	// Act
	res, err := svc.TransformHarness(context.Background(), app.TransformHarnessRequest{
		SourceHarnessID: "no-such-harness", TargetHarnessID: expHarnessID, Workspace: t.TempDir()})

	// Assert
	if err == nil {
		t.Fatal("transform from an unknown source returned no error")
	}
	if res.ExperimentalHarnesses != nil {
		t.Errorf("error result reports %+v, want no warning", res.ExperimentalHarnesses)
	}
}
