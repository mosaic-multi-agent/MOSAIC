package app_test

// unknown_hook_flow_test.go verifies, through the hooks and deploy flows, that an unknown hook
// id fails the run before anything is written and names the searched catalogs, and that a
// known hook the harness does not support is still skipped silently.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// unsupportedHookModule is a harness module that has no target path for any hook bundle.
type unsupportedHookModule struct{ *stubHarnessModule }

func (m unsupportedHookModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	if req.Kind == domain.ArtifactHook {
		return "", errors.New("hooks are not supported by this harness")
	}
	return m.stubHarnessModule.TargetPath(req)
}

func TestDeployHooks_UnknownHookId_FailsNamingCatalogsBeforeAnyWrite(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build(), crStackOpts{})
	exec := &crSpyExecutor{}
	deps.Executor = exec

	_, err := app.New(deps).DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, Scope: domain.ScopeProject,
		HookIDs: []string{"no-such"}, AutoConfirmPlan: true})

	if !errors.Is(err, plan.ErrUnknownHook) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownHook)", err)
	}
	if !strings.Contains(err.Error(), "no-such") {
		t.Errorf("error %q does not name the hook id", err.Error())
	}
	requireSearchedCatalogs(t, err)
	if exec.called {
		t.Error("the executor ran; an unknown hook id must fail before any write")
	}
	if got := crSnapshotDir(t, ws); len(got) != 0 {
		t.Errorf("workspace must stay empty, found %v", got)
	}
}

func TestDeployNew_UnknownHookId_FailsNamingCatalogsBeforeAnyWrite(t *testing.T) {
	deps, _ := crDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build(), crStackOpts{})
	exec := &crSpyExecutor{}
	deps.Executor = exec
	target := filepath.Join(t.TempDir(), "new-workspace")
	req := newDeployRequest(target, "stub-harness", []string{})
	req.HookIDs = []string{"no-such"}

	_, err := app.New(deps).DeployNew(context.Background(), req)

	if !errors.Is(err, plan.ErrUnknownHook) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownHook)", err)
	}
	requireSearchedCatalogs(t, err)
	if exec.called {
		t.Error("the executor ran; an unknown hook id must fail before any write")
	}
	requireNothingCreated(t, target)
}

func TestDeployHooks_KnownHookUnsupportedByHarness_IsSkippedSilently(t *testing.T) {
	deps, ws := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	deps.Catalog = catalogWithHookBundles()
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{minimalHarness},
		modules: map[string]domain.HarnessModule{"stub-harness": unsupportedHookModule{newMinimalModule()}},
	}
	deps.Planner = plan.New()
	exec := &crSpyExecutor{}
	deps.Executor = exec

	_, err := app.New(deps).DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, Scope: domain.ScopeProject,
		HookIDs: []string{minimalHookBundle.Key}, AutoConfirmPlan: true})

	if err != nil {
		t.Fatalf("a known hook the harness does not support must be skipped silently, got: %v", err)
	}
	if !exec.called {
		t.Error("the executor did not run for a known but unsupported hook")
	}
}
