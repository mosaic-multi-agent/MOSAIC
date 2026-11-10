package main

// tui_reload_origin_test.go verifies that the service the TUI reload rebuilds carries the stack
// origin it was given, and that the write runs act on it: only a stack of origin "stack input"
// is persisted to the workspace manifest, any other origin leaves the persisted stack as it was.
//
// Write runs are performed in process with the rebuilt service (hook deployment, which is a
// write run that needs no workflow selection), and the persisted stack is read back from the
// workspace manifest.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/app/stackconfig"
)

// originFixture is a MOSAIC root with the default catalogue (hook "default-hook"), two user
// catalogues that each carry a hook of their own, and the reload capability.
type originFixture struct {
	mosaicRoot string
	userA      string
	userB      string
	reload     func(stackconfig.Description, stackconfig.Origin) (app.Service, error)
}

func newOriginFixture(t *testing.T) originFixture {
	t.Helper()
	mosaicRoot := newStackMosaicRoot(t)
	deps := buildTestDeps(t, mosaicRoot, filepath.Join(mosaicRoot, "Catalog"))
	deps.Interaction = interactiontest.NewBuilder().Build()
	userA := newStackUserCatalog(t, "origin-a", "2.0.0")
	writeStackHook(t, userA, "a-hook")
	userB := newStackUserCatalog(t, "origin-b", "2.0.0")
	writeStackHook(t, userB, "b-hook")
	return originFixture{mosaicRoot: mosaicRoot, userA: userA, userB: userB, reload: buildReloadFunc(mosaicRoot, deps)}
}

// writeRun rebuilds the service from d and origin and deploys hook into ws with it.
func (fx originFixture) writeRun(t *testing.T, ws, hook string, d stackconfig.Description, origin stackconfig.Origin) {
	t.Helper()
	svc, err := fx.reload(d, origin)
	if err != nil {
		t.Fatalf("reload(%+v, %q): %v", d, origin, err)
	}
	_, err = svc.DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID:       "claude-code",
		WorkspacePath:   ws,
		HookIDs:         []string{hook},
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("hook deployment of %s with stack %+v (origin %q): %v", hook, d, origin, err)
	}
}

func TestReloadOrigin_RebuiltServiceReportsTheOriginItWasGiven(t *testing.T) {
	origins := []stackconfig.Origin{stackconfig.OriginStackInput, stackconfig.OriginPersisted, stackconfig.OriginDefault}
	for _, origin := range origins {
		t.Run(string(origin), func(t *testing.T) {
			fx := newOriginFixture(t)

			svc, err := fx.reload(stackconfig.Description{UserLayers: []string{fx.userA}, IncludeDefault: true}, origin)

			if err != nil {
				t.Fatalf("reload: %v", err)
			}
			if got := svc.ActiveStack().Origin; got != origin {
				t.Errorf("active stack origin = %q, want the origin the reload was given, %q", got, origin)
			}
		})
	}
}

func TestReloadOrigin_RebuiltServiceHoldsExactlyTheGivenStack(t *testing.T) {
	fx := newOriginFixture(t)

	svc, err := fx.reload(stackconfig.Description{UserLayers: []string{fx.userA, fx.userB}, IncludeDefault: true}, stackconfig.OriginStackInput)

	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	layers := svc.ActiveStack().Layers
	if len(layers) != 3 || layers[0].Identity.ID != "origin-a" || layers[1].Identity.ID != "origin-b" || !layers[2].Default {
		t.Errorf("active layers = %+v, want origin-a, origin-b, then the default layer", layers)
	}
}

func TestReloadOrigin_WriteRunAfterStackInput_PersistsTheConfirmedStack(t *testing.T) {
	fx := newOriginFixture(t)
	ws := t.TempDir()

	fx.writeRun(t, ws, "a-hook", stackconfig.Description{UserLayers: []string{fx.userA}}, stackconfig.OriginStackInput)

	assertPersistedUserLayers(t, ws, persistedLayer{Path: fx.userA, CatalogID: "origin-a"})
}

func TestReloadOrigin_WriteRunAfterAChange_ReplacesThePersistedStack(t *testing.T) {
	fx := newOriginFixture(t)
	ws := t.TempDir()
	fx.writeRun(t, ws, "a-hook", stackconfig.Description{UserLayers: []string{fx.userA}}, stackconfig.OriginStackInput)

	fx.writeRun(t, ws, "b-hook", stackconfig.Description{UserLayers: []string{fx.userB}}, stackconfig.OriginStackInput)

	assertPersistedUserLayers(t, ws, persistedLayer{Path: fx.userB, CatalogID: "origin-b"})
}

func TestReloadOrigin_WriteRunAfterUnchangedDefaultStart_LeavesThePersistedStackUnchanged(t *testing.T) {
	fx := newOriginFixture(t)
	ws := t.TempDir()
	fx.writeRun(t, ws, "a-hook", stackconfig.Description{UserLayers: []string{fx.userA}}, stackconfig.OriginStackInput)

	fx.writeRun(t, ws, "default-hook", stackconfig.Description{IncludeDefault: true}, stackconfig.OriginDefault)

	assertPersistedUserLayers(t, ws, persistedLayer{Path: fx.userA, CatalogID: "origin-a"})
}

func TestReloadOrigin_WriteRunWithPersistedOrigin_LeavesThePersistedStackUnchanged(t *testing.T) {
	fx := newOriginFixture(t)
	ws := t.TempDir()
	fx.writeRun(t, ws, "a-hook", stackconfig.Description{UserLayers: []string{fx.userA}}, stackconfig.OriginStackInput)

	// Only the origin decides what is persisted: a run that is not "stack input" never rewrites it.
	fx.writeRun(t, ws, "b-hook", stackconfig.Description{UserLayers: []string{fx.userB}}, stackconfig.OriginPersisted)

	assertPersistedUserLayers(t, ws, persistedLayer{Path: fx.userA, CatalogID: "origin-a"})
}

func TestReloadOrigin_WriteRunWithDefaultOriginIntoAFreshWorkspace_PersistsNothing(t *testing.T) {
	fx := newOriginFixture(t)
	ws := t.TempDir()

	fx.writeRun(t, ws, "default-hook", stackconfig.Description{IncludeDefault: true}, stackconfig.OriginDefault)

	if got := readPersistedLayers(t, ws); len(got) != 0 {
		t.Errorf("persisted stack = %+v after a run with the default origin, want none", got)
	}
}

func TestReloadOrigin_WriteRunAfterAChangeToDefaultAlone_PersistsTheDefaultOnlyStack(t *testing.T) {
	fx := newOriginFixture(t)
	ws := t.TempDir()
	fx.writeRun(t, ws, "a-hook", stackconfig.Description{UserLayers: []string{fx.userA}}, stackconfig.OriginStackInput)

	fx.writeRun(t, ws, "default-hook", stackconfig.Description{IncludeDefault: true}, stackconfig.OriginStackInput)

	assertPersistedDefaultOnly(t, ws)
}
