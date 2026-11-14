package app_test

// codex_experimental_test.go tests, with the real built-in Codex module, that the experimental
// declaration of the Codex descriptor reaches the surfaces the app feeds: the harness question
// marks Codex "Experimental" and carries its reason, the reviewed plan exposes the reason, and
// a writing operation's result reports it.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// codexDeployRequest deploys the parity sample catalog to Codex in ws.
func codexDeployRequest(ws string, dryRun bool) app.DeployRequest {
	h := owHarnesses[1]
	return app.DeployRequest{
		HarnessID: hCodex, WorkspacePath: ws, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
		AgentModels: h.models(), AutoConfirmPlan: true, DryRun: dryRun,
		ConflictDefault: domain.DecisionSkip}
}

// codexRef returns the ref of the real Codex module.
func codexRef(t *testing.T) domain.HarnessRef {
	t.Helper()
	return codexAppModule(t).Ref()
}

func TestCodexDeploy_ResultReportsCodexReasonOnce(t *testing.T) {
	// Arrange
	svc := parityServiceWith(t, withCodexTarget(t))

	// Act
	sum, err := svc.DeployNew(context.Background(), codexDeployRequest(t.TempDir(), false))
	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}

	// Assert
	assertReported(t, sum.ExperimentalHarnesses, codexRef(t))
	got := sum.ExperimentalHarnesses[0]
	if !strings.Contains(got.Reason, ".codex/agents") || !strings.Contains(got.Reason, "mosaic-run") {
		t.Errorf("reported reason %q lacks the deploy target or the mosaic-run reference", got.Reason)
	}
}

func TestCodexDeploy_DryRunResultReportsCodexReason(t *testing.T) {
	// Arrange
	svc := parityServiceWith(t, withCodexTarget(t))

	// Act
	sum, err := svc.DeployNew(context.Background(), codexDeployRequest(t.TempDir(), true))
	if err != nil {
		t.Fatalf("DeployNew dry run: %v", err)
	}

	// Assert
	assertReported(t, sum.ExperimentalHarnesses, codexRef(t))
}

func TestCodexDeploy_ReviewedPlanExposesCodexReason(t *testing.T) {
	// Arrange
	rec := newRecordingInteraction(nil)
	svc := parityServiceWith(t, withCodexTarget(t), withInteraction(rec))

	// Act
	if _, err := svc.DeployNew(context.Background(), codexDeployRequest(t.TempDir(), false)); err != nil {
		t.Fatalf("DeployNew: %v", err)
	}

	// Assert
	assertPlanExperimental(t, rec.lastPlan(t), codexRef(t))
}

func TestCodexDeploy_DoesNotChangeOutcomeOrFailTheRun(t *testing.T) {
	// Arrange
	svc := parityServiceWith(t, withCodexTarget(t))

	// Act
	sum, err := svc.DeployNew(context.Background(), codexDeployRequest(t.TempDir(), false))

	// Assert
	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
	if sum.Outcome == domain.OutcomeFailed {
		t.Errorf("outcome = %q, want a Codex deploy to stay a normal run", sum.Outcome)
	}
}

func TestCodexHarnessQuestion_MarksCodexExperimentalWithReason(t *testing.T) {
	// Arrange
	rec := newRecordingInteraction(map[domain.QuestionID]string{domain.QHarness: hCodex})
	svc := parityServiceWith(t, withCodexTarget(t), withInteraction(rec))
	ref := codexRef(t)

	// Act
	_, _ = svc.DeployNew(context.Background(), app.DeployRequest{
		WorkspacePath: t.TempDir(), Scope: domain.ScopeProject})

	// Assert
	opt := optionByID(t, rec.question(t, domain.QHarness), hCodex)
	assertExperimentalOption(t, opt, ref)
	if opt.Disabled {
		t.Errorf("Codex option disabled (%q), want experimental status to leave it selectable", opt.DisabledReason)
	}
	if !strings.Contains(opt.Label, "Experimental") {
		t.Errorf("Codex option label = %q, want it marked Experimental", opt.Label)
	}
}

func TestCodexHarnessQuestion_OtherHarnessesAreNotMarked(t *testing.T) {
	// Arrange
	rec := newRecordingInteraction(map[domain.QuestionID]string{domain.QHarness: hCodex})
	svc := parityServiceWith(t, withCodexTarget(t), withInteraction(rec))

	// Act
	_, _ = svc.DeployNew(context.Background(), app.DeployRequest{
		WorkspacePath: t.TempDir(), Scope: domain.ScopeProject})

	// Assert
	q := rec.question(t, domain.QHarness)
	for _, id := range []string{hClaude, hOpen, hCLI, hVSCode} {
		if o := optionByID(t, q, id); strings.Contains(o.Label, domain.ExperimentalLabel) {
			t.Errorf("option %q label = %q, want only Codex marked experimental", id, o.Label)
		}
	}
}
