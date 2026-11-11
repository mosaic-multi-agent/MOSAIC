package app_test

// reconcile_hooks_flow_test.go drives the deploy-hooks flow through the real planner, executor
// and manifest store over a workspace whose manifest records hook files that are not on disk:
// the whole bundle of another harness, and one file of a bundle of the run harness whose other
// file exists (a damaged bundle). Only the first is dropped; nothing is written back by a dry
// run, and an executed run saves the reconciled manifest.

import (
	"bytes"
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

const (
	rhGhostPath      = ".opencode/plugins/ghost-hook.ts"
	rhMissingFile    = flowHooksDir + "/logger.sh"
	rhPresentFile    = flowHooksDir + "/lib/util.sh"
	rhOtherHarness   = "opencode"
	rhGhostBundleKey = "ghost-hook"
	rhRunHarness     = "stub-harness"
	rhDamagedBundle  = flowHookKey
)

// rhNoFileHookModule deploys the bundle without copying any file, so an executed run needs no
// hook sources.
type rhNoFileHookModule struct{ flatHookModule }

func (m rhNoFileHookModule) HookPlan(_ domain.HookPlanRequest) (domain.HookPlan, error) {
	return domain.HookPlan{Supported: true, TargetDir: flowHooksDir}, nil
}

func rhHookEntry(harness, key, path string) domain.ManifestEntry {
	e := rfEntry(harness, key, path)
	e.Ref.Kind = domain.ArtifactHook
	return e
}

// rhWorkspace seeds the real manifest store with a damaged bundle of the run harness (one file
// missing, util.sh present on disk) and a bundle of another harness whose only file is missing.
func rhWorkspace(t *testing.T) (deps app.Deps, ws string, c *gamReviewCapture) {
	t.Helper()
	deps, ws = newFlatHookDeps(t, "1.0", true)
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{minimalHarness},
		modules: map[string]domain.HarnessModule{rhRunHarness: rhNoFileHookModule{flatHookModule{newMinimalModule()}}},
	}
	store := manifest.NewStore()
	deps.Manifest = store
	deps.Planner = plan.New()
	deps.Executor = deploy.NewExecutor(store, deps.Logger, deps.Todo)
	c = gamNewReviewCapture(ws)
	deps.Interaction = c
	m := domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     []domain.HarnessSection{{HarnessID: rhRunHarness}, {HarnessID: rhOtherHarness}},
		Entries: []domain.ManifestEntry{
			rhHookEntry(rhRunHarness, rhDamagedBundle, rhMissingFile),
			rhHookEntry(rhRunHarness, rhDamagedBundle, rhPresentFile),
			rhHookEntry(rhOtherHarness, rhGhostBundleKey, rhGhostPath),
		},
	}
	gamSaveManifest(t, ws, m)
	return deps, ws, c
}

func rhRun(t *testing.T, deps app.Deps, ws string, dry bool) {
	t.Helper()
	_, err := app.New(deps).DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID: rhRunHarness, WorkspacePath: ws, Scope: domain.ScopeProject,
		HookIDs: []string{flowHookKey}, AutoConfirmPlan: true, DryRun: dry})
	if err != nil {
		t.Fatalf("DeployHooks: %v", err)
	}
}

func TestReconcileHooksFlow_DryRunListsTheGhostBundleOfAnotherHarnessAndKeepsTheDamagedOne(t *testing.T) {
	deps, ws, c := rhWorkspace(t)
	before := rrManifestBytes(t, ws)

	rhRun(t, deps, ws, true)

	dropped := rrDroppedPaths(gplReviewedPlan(t, c))
	if !rrHasPath(dropped, rhGhostPath) {
		t.Errorf("dropped entries %v lack the missing hook file %s of %s", dropped, rhGhostPath, rhOtherHarness)
	}
	for _, kept := range []string{rhMissingFile, rhPresentFile} {
		if rrHasPath(dropped, kept) {
			t.Errorf("%s belongs to a bundle with a file on disk and must be kept, but is dropped", kept)
		}
	}
	if !bytes.Equal(before, rrManifestBytes(t, ws)) {
		t.Error("a dry run changed the manifest file")
	}
}

func TestReconcileHooksFlow_ExecutedRunSavesTheManifestWithoutTheGhostBundle(t *testing.T) {
	deps, ws, _ := rhWorkspace(t)

	rhRun(t, deps, ws, false)

	m := gamManifest(t, ws)
	for _, e := range m.Entries {
		if rrHasPath([]string{e.TargetPath}, rhGhostPath) {
			t.Errorf("the saved manifest still records the missing hook file %s", rhGhostPath)
		}
	}
	if m.Holds(rhOtherHarness) {
		t.Errorf("harness %s, whose entries were all dropped, is still held", rhOtherHarness)
	}
	if !m.Holds(rhRunHarness) {
		t.Errorf("the run harness %s lost its entries", rhRunHarness)
	}
	var kept []string
	for _, e := range m.Entries {
		if e.Harness == rhRunHarness && e.Ref.Key == rhDamagedBundle {
			kept = append(kept, e.TargetPath)
		}
	}
	for _, want := range []string{rhMissingFile, rhPresentFile} {
		if !rrHasPath(kept, want) {
			t.Errorf("the saved manifest lost %s of the damaged bundle; bundle entries = %v", want, kept)
		}
	}
}
