package app_test

// reconcile_damaged_hooks_flow_test.go verifies that the damaged-bundle mark reaches planning
// through the deploy-hooks flow: the planner input carries the damaged bundle of the run harness
// and not that of another harness. It reuses the workspace of reconcile_hooks_flow_test.go.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

const rdOtherPresentFile = ".opencode/plugins/logger-present.ts"

// rdRecordingPlanner delegates to a real planner and keeps the input of its last Build.
type rdRecordingPlanner struct {
	inner plan.Planner
	input *plan.Input
}

func (p *rdRecordingPlanner) Build(ctx context.Context, in plan.Input) (domain.Plan, error) {
	cp := in
	p.input = &cp
	return p.inner.Build(ctx, in)
}

// rdSeedOtherHarnessDamagedBundle adds a damaged bundle with the same key for another harness:
// one file missing, one on disk.
func rdSeedOtherHarnessDamagedBundle(t *testing.T, ws string) {
	t.Helper()
	m := gamManifest(t, ws)
	m.Entries = append(m.Entries,
		rhHookEntry(rhOtherHarness, rhDamagedBundle, ".opencode/plugins/logger-missing.ts"),
		rhHookEntry(rhOtherHarness, rhDamagedBundle, rdOtherPresentFile))
	gamSaveManifest(t, ws, m)
	p := filepath.Join(ws, filepath.FromSlash(rdOtherPresentFile))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileHooksFlow_PlannerInputCarriesOnlyTheRunHarnessDamagedBundle(t *testing.T) {
	deps, ws, _ := rhWorkspace(t)
	rdSeedOtherHarnessDamagedBundle(t, ws)
	rec := &rdRecordingPlanner{inner: deps.Planner}
	deps.Planner = rec

	rhRun(t, deps, ws, true)

	if rec.input == nil {
		t.Fatal("planner.Build was never called")
	}
	want := []manifest.DamagedBundle{{Harness: rhRunHarness, Key: rhDamagedBundle}}
	if got := rec.input.Manifest.DamagedHooks; !reflect.DeepEqual(got, want) {
		t.Errorf("planner manifest DamagedHooks = %+v, want only the run harness bundle %+v", got, want)
	}
}

func TestReconcileHooksFlow_PlannerInputHasNoDamagedBundleWhenTheRunHarnessBundleIsComplete(t *testing.T) {
	deps, ws, _ := rhWorkspace(t)
	rdSeedOtherHarnessDamagedBundle(t, ws)
	if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(rhMissingFile)), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := &rdRecordingPlanner{inner: deps.Planner}
	deps.Planner = rec

	rhRun(t, deps, ws, true)

	if rec.input == nil {
		t.Fatal("planner.Build was never called")
	}
	if got := rec.input.Manifest.DamagedHooks; len(got) != 0 {
		t.Errorf("planner manifest DamagedHooks = %+v, want none: another harness's damage is not the run's", got)
	}
}
