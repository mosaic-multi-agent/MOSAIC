package tui

// stack_persisted_helpers_test.go holds the shared fixtures for the persisted-stack flow tests: an
// in-memory store standing in for workspace manifests, a launch service double that reads from
// it, and a rebuilt-service double that persists the way a write run does (only for a stack that
// came from stack input, and only in the modes that persist). Nothing here is a test itself.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// persistStore maps a workspace directory to its persisted stack.
type persistStore struct{ stacks map[string]stackconfig.Stored }

func newPersistStore() *persistStore { return &persistStore{stacks: map[string]stackconfig.Stored{}} }

func (p *persistStore) get(ws string) (stackconfig.Stored, bool) {
	s, ok := p.stacks[filepath.Clean(ws)]
	return s, ok
}

func (p *persistStore) set(ws string, s stackconfig.Stored) { p.stacks[filepath.Clean(ws)] = s }

// storedOf builds a persisted stack: user layers in order, then the default entry when asked.
func storedOf(includeDefault bool, userLayers ...string) stackconfig.Stored {
	var layers []domain.PersistedCatalogLayer
	for _, p := range userLayers {
		layers = append(layers, domain.PersistedCatalogLayer{Path: p, CatalogID: "id-" + filepath.Base(p)})
	}
	if includeDefault {
		layers = append(layers, domain.PersistedCatalogLayer{Default: true})
	}
	return stackconfig.Stored{Layers: layers}
}

// newWorkspaceDir creates a directory holding .mosaic/manifest.yaml and records stored (when it
// has layers) as its persisted stack.
func newWorkspaceDir(t *testing.T, store *persistStore, stored stackconfig.Stored) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".mosaic"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mosaic", "manifest.yaml"), []byte("schema: \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(stored.Layers) > 0 {
		store.set(dir, stored)
	}
	return dir
}

// subDir creates and returns a subdirectory of dir.
func subDir(t *testing.T, dir, name string) string {
	t.Helper()
	sub := filepath.Join(dir, name)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	return sub
}

// persistedSvc is the launch service. It answers the stack-service queries from the store and
// the real file system, like the app service does.
type persistedSvc struct {
	*stackFlowSvc
	store       *persistStore
	active      stackconfig.ActiveStack
	validateErr error
	readErr     error
}

func (s *persistedSvc) ReadPersistedStack(ws string) (stackconfig.Stored, bool, error) {
	if s.readErr != nil {
		return stackconfig.Stored{}, false, s.readErr
	}
	st, ok := s.store.get(ws)
	return st, ok, nil
}

// ValidatePersistedStack reports validateErr when the stack holds a user layer whose folder is
// missing on disk (a stack of existing folders is valid), so removing the bad layer on the stack
// screen yields a valid stack again.
func (s *persistedSvc) ValidatePersistedStack(st stackconfig.Stored) error {
	for _, l := range st.Layers {
		if l.Default {
			continue
		}
		if _, err := os.Stat(l.Path); err != nil {
			return s.validateErr
		}
	}
	return nil
}

func (s *persistedSvc) ActiveStack() stackconfig.ActiveStack { return s.active }

// FindWorkspace returns path when it is a directory holding .mosaic/manifest.yaml, otherwise the
// nearest ancestor that does.
func (s *persistedSvc) FindWorkspace(path string) (string, bool) {
	for dir := filepath.Clean(path); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".mosaic", "manifest.yaml")); err == nil {
			return dir, true
		}
		if filepath.Dir(dir) == dir {
			return "", false
		}
	}
}

// persistingSvc is the rebuilt service. A write run persists the stack it was built with when
// that stack came from stack input; promote and transform never persist.
//
// The persistence rule is implemented here, so tests using this double prove that the TUI starts
// from the persisted stack and passes the right description and origin to the reload capability;
// they cannot prove "never persisted" against a TUI that persists on its own. That rule (only the
// stack input origin is persisted, a different stack replaces the old one, a declined plan writes
// nothing) is verified against the real service in internal/app (stack_persistence_test.go and
// TestWritePaths_PersistOnlyTheStackInputOrigin).
type persistingSvc struct {
	*stackFlowSvc
	store     *persistStore
	workspace string
	desc      stackconfig.Description
	origin    stackconfig.Origin
	declines  bool // the user declines the plan: the run writes nothing
}

func (s *persistingSvc) persist() {
	if s.declines || s.origin != stackconfig.OriginStackInput {
		return
	}
	s.store.set(s.workspace, storedOf(s.desc.IncludeDefault, s.desc.UserLayers...))
}

func (s *persistingSvc) DeployNew(ctx context.Context, r app.DeployRequest) (domain.RunSummary, error) {
	s.persist()
	return s.stackFlowSvc.DeployNew(ctx, r)
}

func (s *persistingSvc) Update(ctx context.Context, r app.UpdateRequest) (domain.RunSummary, error) {
	s.persist()
	return s.stackFlowSvc.Update(ctx, r)
}

func (s *persistingSvc) UpdateWorkflows(ctx context.Context, r app.WorkflowUpdateRequest) (domain.RunSummary, error) {
	s.persist()
	return s.stackFlowSvc.UpdateWorkflows(ctx, r)
}

func (s *persistingSvc) DeployAgents(ctx context.Context, r app.DeployAgentsRequest) (domain.RunSummary, error) {
	s.persist()
	return s.stackFlowSvc.DeployAgents(ctx, r)
}

func (s *persistingSvc) DeployHooks(ctx context.Context, r app.DeployHooksRequest) (domain.RunSummary, error) {
	s.persist()
	return s.stackFlowSvc.DeployHooks(ctx, r)
}

// persistFlowOpts configures newPersistedFlow beyond the Options flow options.
type persistFlowOpts struct {
	active      stackconfig.ActiveStack
	validateErr error
	readErr     error
	declines    bool
}

// newPersistedFlow builds a root model for mode whose launch service reads persisted stacks from
// store and whose path screen is pre-filled with path.
func newPersistedFlow(t *testing.T, mode domain.RunMode, store *persistStore, path string,
	po persistFlowOpts, opts ...flowOption) *stackFlow {
	t.Helper()
	f := &stackFlow{log: &runLog{}, mode: mode, path: path, rec: &reloadRecorder{}}
	base := newStackFlowSvc("original", f.log)
	f.orig = base
	launch := &persistedSvc{stackFlowSvc: base, store: store, active: po.active,
		validateErr: po.validateErr, readErr: po.readErr}
	reload := func(d stackconfig.Description, origin stackconfig.Origin) (app.Service, error) {
		f.rec.calls = append(f.rec.calls, reloadCall{d: d, origin: origin})
		f.reloaded = newStackFlowSvc("reloaded", f.log)
		return &persistingSvc{
			stackFlowSvc: f.reloaded, store: store, workspace: f.m.wsScreen.WorkspacePath(),
			desc: d, origin: origin, declines: po.declines,
		}, nil
	}
	o := Options{
		Theme:          DefaultTheme(),
		ReloadCatalog:  reload,
		LaunchStack:    stackconfig.Description{IncludeDefault: true},
		InitialRequest: &app.DeployRequest{WorkspacePath: path},
	}
	for _, opt := range opts {
		opt(&o)
	}
	f.m = newRootModel(t.Context(), launch, o)
	return f
}

// writeModes are the modes whose run persists the stack with the deployment it writes.
var writeModes = []domain.RunMode{
	domain.ModeDeployWorkspace,
	domain.ModeUpdateWorkspace,
	domain.ModeUpdateWorkflows,
	domain.ModeDeployAgents,
	domain.ModeDeployHooks,
}
