package app_test

// deployhooks_registration_regression_test.go pins the existing deploy-hooks behaviour for an
// already-deployed hook that is at the catalog version: its registration gap is still reported
// in the plan and in the TODO when the registration target exists. The Update hook refresh
// filters registration artefacts of hooks it does not write; deploy-hooks must not.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/manifest/manifesttest"
	"mosaic-deploy/internal/plan"
)

// uhPerFileEntries records every file of files as a hook file at the catalog version.
func uhPerFileEntries(files map[string]string) []domain.ManifestEntry {
	var entries []domain.ManifestEntry
	for target, content := range files {
		entries = append(entries, domain.ManifestEntry{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactHook, Key: uhHookKey},
			TargetPath: target, Version: uhCatalogVersion, ContentHash: manifest.Hash([]byte(content)),
		})
	}
	return entries
}

func TestDeployHooks_UnchangedDeployedHookWithExistingTarget_RegistrationGapStillReported(t *testing.T) {
	module := newTestHookModule(uhHarnessID, newTestHookPlan(t, uhFlat))
	deployed := uhFixtureFiles(t, uhFlat)
	review := &uhReviewSpy{Stub: interactiontest.NewBuilder().AnswerReview(true).Build()}
	deps, workspace := newBaseDeps(t, review.Stub)
	deps.Interaction = review
	cat := newMinimalCatalog()
	cat.hooks = []domain.HookBundle{{Key: uhHookKey, Version: uhCatalogVersion}}
	deps.Catalog = cat
	deps.Planner = &uhPlannerSpy{inner: plan.New(), workspace: workspace}
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{module.Ref()},
		modules: map[string]domain.HarnessModule{uhHarnessID: module},
	}
	deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{State: manifest.StatePresent, Manifest: domain.Manifest{
		SchemaVersion: manifest.SchemaVersion, Harnesses: []domain.HarnessSection{{HarnessID: uhHarnessID}},
		Entries: manifesttest.WithHarness(uhHarnessID, uhPerFileEntries(deployed)),
	}}}
	deps.Executor = deploy.NewExecutor(deps.Manifest, deps.Logger, deps.Todo)
	deployed[uhRegTarget] = uhExistingSettings
	uhSeedFiles(t, workspace, deployed)

	_, err := app.New(deps).DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID:       uhHarnessID,
		WorkspacePath:   workspace,
		Scope:           domain.ScopeProject,
		HookIDs:         []string{uhHookKey},
		AutoConfirmPlan: true,
	})

	if err != nil {
		t.Fatalf("DeployHooks: %v", err)
	}
	if review.reviewed == nil {
		t.Fatal("no plan was reviewed")
	}
	hookItems := 0
	for _, item := range review.reviewed.Items {
		if item.Ref.Key != uhHookKey {
			continue
		}
		hookItems++
		if item.Action != domain.ActionUnchanged {
			t.Errorf("hook file item %s = %q, want ActionUnchanged", item.TargetPath, item.Action)
		}
	}
	if hookItems == 0 {
		t.Fatal("no hook file plan items were reviewed")
	}
	if !uhHasRegistrationGap(review.reviewed.Gaps) {
		t.Errorf("deploy-hooks plan lacks the registration gap of an unchanged hook: %+v", review.reviewed.Gaps)
	}
	if !uhHasRegistrationStep(review.reviewed.Registrations) {
		t.Errorf("deploy-hooks plan lacks the registration step of an unchanged hook: %+v", review.reviewed.Registrations)
	}
	if !deps.Todo.(*spyTodo).hasGapKind(domain.GapHookRegistration) {
		t.Errorf("deploy-hooks TODO lacks the hook-registration gap: %+v", deps.Todo.(*spyTodo).gaps)
	}
}
