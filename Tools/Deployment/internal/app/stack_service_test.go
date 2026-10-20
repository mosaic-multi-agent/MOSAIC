package app_test

// stack_service_test.go verifies the stack service interface of the app service: the four
// persisted-stack operations are reachable through app.Service and behave as the stackconfig
// operations they delegate to (one case each), and the shared no-op implementation that
// service test doubles embed satisfies the interface.
//
// It also verifies that the commands that never write a deployment (render, promote,
// transform) leave the persisted stack alone even when the service was built from stack input.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/app/stackservicetest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

var _ app.StackService = stackservicetest.MockStackService{}

func TestService_ReadPersistedStack_ReturnsTheWorkspacesStack(t *testing.T) {
	f := newSPFixture(t)
	f.deployWithInput(t, f.descA())
	svc := app.New(f.deps)

	got, ok, err := svc.ReadPersistedStack(f.ws)

	if err != nil || !ok {
		t.Fatalf("ReadPersistedStack = (_, %v, %v), want ok and no error", ok, err)
	}
	requireStackEqual(t, got.Layers, f.layersA())
}

func TestService_ReadPersistedStack_FreshWorkspaceReportsNone(t *testing.T) {
	svc := app.New(newSPFixture(t).deps)

	got, ok, err := svc.ReadPersistedStack(t.TempDir())

	if err != nil || ok || len(got.Layers) != 0 {
		t.Errorf("ReadPersistedStack = (%+v, %v, %v), want none and no error", got, ok, err)
	}
}

func TestService_ValidatePersistedStack_UsesTheServicesMosaicRoot(t *testing.T) {
	f := newSPFixture(t)
	svc := app.New(f.deps)
	good := stackconfig.Stored{Layers: f.layersA()}
	bad := stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{
		{Path: filepath.Join(t.TempDir(), "gone"), CatalogID: "cat-gone"}, {Default: true},
	}}

	if err := svc.ValidatePersistedStack(good); err != nil {
		t.Errorf("ValidatePersistedStack(resolvable): %v", err)
	}
	var ule *stackconfig.UnresolvableLayerError
	if err := svc.ValidatePersistedStack(bad); !errors.As(err, &ule) || ule.ExpectedID != "cat-gone" {
		t.Errorf("ValidatePersistedStack(missing folder) = %v, want an UnresolvableLayerError for cat-gone", err)
	}
}

func TestService_EncodeStack_EncodesTheDescription(t *testing.T) {
	f := newSPFixture(t)
	svc := app.New(f.deps)

	got, err := svc.EncodeStack(f.descA())

	if err != nil {
		t.Fatalf("EncodeStack: %v", err)
	}
	requireStackEqual(t, got.Layers, f.layersA())
	if _, err := svc.EncodeStack(stackconfig.Description{}); err == nil {
		t.Error("EncodeStack of an empty description succeeded; want the empty-stack error")
	}
}

func TestService_FindWorkspace_FindsTheWorkspaceOfAPath(t *testing.T) {
	f := newSPFixture(t)
	f.deployWithInput(t, f.descA())
	svc := app.New(f.deps)
	inside := filepath.Join(f.ws, "sub", "file.md")

	got, ok := svc.FindWorkspace(inside)

	if !ok {
		t.Fatalf("FindWorkspace(%q) found no workspace, want %q", inside, f.ws)
	}
	if gi, wi := mustStat(t, got), mustStat(t, f.ws); !os.SameFile(gi, wi) {
		t.Errorf("FindWorkspace = %q, want %q", got, f.ws)
	}
	if got, ok := svc.FindWorkspace(t.TempDir()); ok {
		t.Errorf("FindWorkspace of a directory outside any workspace = %q, want none", got)
	}
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	return fi
}

func TestMockStackService_IsANoOp(t *testing.T) {
	var m stackservicetest.MockStackService

	if s, ok, err := m.ReadPersistedStack("ws"); len(s.Layers) != 0 || ok || err != nil {
		t.Errorf("ReadPersistedStack = (%+v, %v, %v), want none", s, ok, err)
	}
	if err := m.ValidatePersistedStack(stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{{Path: "x", CatalogID: "y"}}}); err != nil {
		t.Errorf("ValidatePersistedStack = %v, want nil", err)
	}
	if s, err := m.EncodeStack(stackconfig.Description{UserLayers: []string{"x"}}); len(s.Layers) != 0 || err != nil {
		t.Errorf("EncodeStack = (%+v, %v), want a zero Stored and no error", s, err)
	}
	if w, ok := m.FindWorkspace("x"); w != "" || ok {
		t.Errorf("FindWorkspace = (%q, %v), want none", w, ok)
	}
	if a := m.ActiveStack(); len(a.Layers) != 0 || a.Origin != "" {
		t.Errorf("ActiveStack = %+v, want the zero stack", a)
	}
}

// ---------------------------------------------------------------------------
// Commands that never write a deployment
// ---------------------------------------------------------------------------

const persistedStackManifest = `schema_version: "1"
harness_id: stub-harness
updated_at: "2025-01-01T00:00:00Z"
entries: []
catalog_stack:
  - {path: 'PATHA', catalog_id: layer-a}
  - {default: true}
`

// writePersistedWorkspace creates a workspace whose manifest persists userA and the default
// layer, and returns the workspace and the manifest's bytes.
func writePersistedWorkspace(t *testing.T, f *spFixture) (string, []byte) {
	t.Helper()
	ws := t.TempDir()
	dir := filepath.Join(ws, manifest.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := []byte(strings.Replace(persistedStackManifest, "PATHA", f.userA, 1))
	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), text, 0o644); err != nil {
		t.Fatal(err)
	}
	return ws, text
}

// nonWriteCommand runs one command that never writes a deployment, against deps whose stack came
// from stack input.
type nonWriteCommand struct {
	name string
	run  func(t *testing.T, f *spFixture, ws string) error
}

func nonWriteCommands() []nonWriteCommand {
	return []nonWriteCommand{
		{name: "render", run: func(t *testing.T, f *spFixture, ws string) error {
			deps := newRenderDeps(t)
			f.adopt(&deps)
			src := writeRenderSourceFile(t, t.TempDir(), "render-test-agent.md", minimalGenericSubagentBytes())
			_, err := app.New(deps).RenderAgent(context.Background(), app.RenderAgentRequest{
				SourcePath: src, TargetHarnessID: renderHarness.ID, WorkspaceRoot: ws,
			})
			return err
		}},
		{name: "transform", run: func(t *testing.T, f *spFixture, ws string) error {
			deps, _ := newTransformDeps(t, interactiontest.NewBuilder().Build())
			f.adopt(&deps)
			path := writeTransformFile(t, t.TempDir(), "my-agent.src.md", matchingAgentBytes())
			_, err := app.New(deps).TransformHarness(context.Background(), newPreAnsweredTransformRequest(path))
			return err
		}},
		{name: "promote", run: func(t *testing.T, f *spFixture, ws string) error {
			mosaicRoot := writeMosaicRoot(t)
			deps, _ := newPromoteDeps(t, interactiontest.NewBuilder().Build(), mosaicRoot)
			f.adopt(&deps)
			src := writeEligibleSourceFile(t, t.TempDir(), "my-harness-agent.md")
			_, err := app.New(deps).Promote(context.Background(), app.PromoteRequest{
				FilePath: src, Category: "TestCategory", HarnessID: "stub-harness",
			})
			return err
		}},
	}
}

// adopt gives deps the fixture's stack input and its recording executor.
func (f *spFixture) adopt(deps *app.Deps) {
	in := f.withInput(f.descB())
	deps.StackDescription = in.StackDescription
	deps.StackFromInput = true
	deps.Executor = f.exec
	deps.Manifest = f.store
}

func TestNonWriteCommands_NeverChangeThePersistedStack(t *testing.T) {
	for _, cmd := range nonWriteCommands() {
		t.Run(cmd.name, func(t *testing.T) {
			f := newSPFixture(t)
			ws, before := writePersistedWorkspace(t, f)

			if err := cmd.run(t, f, ws); err != nil {
				t.Fatalf("%s: %v", cmd.name, err)
			}

			if len(f.exec.reqs) != 0 {
				t.Errorf("%s reached the executor (%d requests); only a write run may persist a stack", cmd.name, len(f.exec.reqs))
			}
			after, err := os.ReadFile(filepath.Join(ws, manifest.Dir, manifest.FileName))
			if err != nil || string(after) != string(before) {
				t.Errorf("manifest changed by %s (read err %v):\n%s", cmd.name, err, after)
			}
			got, ok, err := app.New(f.deps).ReadPersistedStack(ws)
			if err != nil || !ok {
				t.Fatalf("ReadPersistedStack = (_, %v, %v), want the persisted stack", ok, err)
			}
			requireStackEqual(t, got.Layers, f.layersA())
		})
	}
}
