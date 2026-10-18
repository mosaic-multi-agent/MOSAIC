package app_test

// stack_persistence_fixtures_test.go holds the fixtures shared by the tests of the catalog stack
// the write runs persist: a fixture with two user catalogs, the stack inputs a run can be built
// with, a service wired to the real executor and manifest store, and readers of the persisted
// value. Nothing in this file is a test itself.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// spFixture is a set of deps over the provenance fixtures with two user catalogs on disk.
type spFixture struct {
	deps  app.Deps
	ws    string
	exec  *recordingExecutor
	store manifest.Store
	userA string // catalog id spIDA
	userB string // catalog id spIDB
}

const (
	spIDA = "layer-a"
	spIDB = "layer-b"
)

func newSPFixture(t *testing.T) *spFixture {
	t.Helper()
	deps, ws, exec := provDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	return &spFixture{
		deps:  deps,
		ws:    ws,
		exec:  exec,
		store: manifest.NewStore(),
		userA: newSysUserLayer(t, sysLayer{id: spIDA}),
		userB: newSysUserLayer(t, sysLayer{id: spIDB}),
	}
}

// withInput returns deps whose stack came from stack input.
func (f *spFixture) withInput(d stackconfig.Description) app.Deps {
	deps := f.deps
	deps.StackDescription = d
	deps.StackFromInput = true
	return deps
}

// withoutInput returns deps whose stack is the default layer alone and did not come from input.
func (f *spFixture) withoutInput() app.Deps {
	deps := f.deps
	deps.StackDescription = stackconfig.Description{IncludeDefault: true}
	deps.StackFromInput = false
	return deps
}

// real returns deps wired to the real executor and manifest store, so a run writes the manifest.
func (f *spFixture) real(deps app.Deps) app.Deps {
	deps.Manifest = f.store
	deps.Executor = deploy.NewExecutor(f.store, &stubLogger{}, deps.Todo)
	return deps
}

func (f *spFixture) descA() stackconfig.Description {
	return stackconfig.Description{UserLayers: []string{f.userA}, IncludeDefault: true}
}

func (f *spFixture) descB() stackconfig.Description {
	return stackconfig.Description{UserLayers: []string{f.userB}}
}

func (f *spFixture) layersA() []domain.PersistedCatalogLayer {
	return []domain.PersistedCatalogLayer{{Path: f.userA, CatalogID: spIDA}, {Default: true}}
}

func (f *spFixture) layersB() []domain.PersistedCatalogLayer {
	return []domain.PersistedCatalogLayer{{Path: f.userB, CatalogID: spIDB}}
}

// persisted returns the stack the manifest of the fixture's workspace holds.
func (f *spFixture) persisted(t *testing.T) []domain.PersistedCatalogLayer {
	t.Helper()
	snap, err := f.store.Load(f.ws)
	if err != nil || snap.State != manifest.StatePresent {
		t.Fatalf("manifest: state %q, err %v / %v", snap.State, snap.Err, err)
	}
	return snap.Manifest.CatalogStack
}

// requirePersisted asserts that the manifest holds exactly want.
func (f *spFixture) requirePersisted(t *testing.T, want []domain.PersistedCatalogLayer) {
	t.Helper()
	got := f.persisted(t)
	if len(got) != len(want) {
		t.Fatalf("persisted stack = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("persisted stack = %+v, want %+v", got, want)
		}
	}
}

// deployWithInput runs a deploy with stack input d through the real executor.
func (f *spFixture) deployWithInput(t *testing.T, d stackconfig.Description) {
	t.Helper()
	ops := provOperations()
	if _, err := ops[0].run(context.Background(), app.New(f.real(f.withInput(d))), f.ws); err != nil {
		t.Fatalf("deploy: %v", err)
	}
}

// opByName returns the write operation with the given name.
func opByName(t *testing.T, name string) provOperation {
	t.Helper()
	for _, op := range provOperations() {
		if op.name == name {
			return op
		}
	}
	t.Fatalf("no write operation %q", name)
	return provOperation{}
}

// revertingExecutor runs the real executor after adding to the plan an item that cannot be
// written, so an atomic run reverses itself.
type revertingExecutor struct {
	inner deploy.Executor
}

func (e revertingExecutor) Execute(ctx context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	blocker := filepath.Join(req.Plan.WorkspacePath, "blocked")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		return deploy.ExecResult{}, err
	}
	late := domain.PlanItem{
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "late-agent"},
		TargetPath: "blocked/late-agent.md",
		Action:     domain.ActionCreate,
	}
	req.Plan.Items = append(append([]domain.PlanItem(nil), req.Plan.Items...), late)
	content := req.Content
	req.Content = func(it domain.PlanItem) ([]byte, error) {
		if it.Ref == late.Ref {
			return []byte("# late"), nil
		}
		return content(it)
	}
	return e.inner.Execute(ctx, req)
}
