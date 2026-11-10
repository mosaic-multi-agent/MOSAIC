package app_test

// stack_injection_check_test.go verifies the selected-harness injection check of the stack
// service interface: given a harness id, it reports whether the active stack supplies that
// harness's injections, returning the error the registry raises for the harness's injection
// item (an error wrapping *injections.NotSuppliedError) when no layer supplies it.
//
// Fixtures are temp-dir catalogs built here. The harnesses are test built-ins that ask the
// registry's content-directory resolver for one harness directory, the way the real built-ins do.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// injectionCheckHarnesses are the ids of the two test built-ins: one reads the Claude Code
// injection directory, the other the OpenCode one.
type injectionCheckHarnesses struct{ claude, opencode string }

// registerInjectionHarness registers a built-in under id that resolves its content directory
// from harnessDir and fails construction when the resolver fails.
func registerInjectionHarness(id, harnessDir string) {
	registry.Register(id, func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		if _, err := opts.ResolveContentDir(harnessDir); err != nil {
			return nil, err
		}
		return &appTestModule{desc: &domain.HarnessDescriptor{ID: id}}, nil
	})
}

// newInjectionCheckService builds a service over a real stack of the given user layers
// over a default catalog. The default catalog supplies the OpenCode injections; the optional
// user layer on top supplies the Claude Code injections.
func newInjectionCheckService(t *testing.T, withUserLayer bool) (app.Service, injectionCheckHarnesses) {
	t.Helper()
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	mosaic := newSysMosaicRoot(t, sysLayer{id: "inj-default"})
	writeSysFile(t, filepath.Join(mosaic, catalogpaths.MosaicRelHarnessInjectionsDir,
		catalogpaths.HarnessDirNameOpenCode, "injection.md"), "default opencode injection\n")
	layers := []stack.Layer{defaultStackLayer(mosaic)}
	if withUserLayer {
		user := newSysUserLayer(t, sysLayer{id: "inj-user"})
		writeSysFile(t, filepath.Join(catalogpaths.HarnessInjectionsDir(user, catalogpaths.HarnessDirNameClaudeCode),
			"injection.md"), "user claude injection\n")
		layers = append([]stack.Layer{{Root: user}}, layers...)
	}
	useResolvedStack(t, &deps, mosaic, layers...)

	h := injectionCheckHarnesses{
		claude:   "inj-check-claude-" + filepath.Base(t.TempDir()),
		opencode: "inj-check-opencode-" + filepath.Base(t.TempDir()),
	}
	registerInjectionHarness(h.claude, catalogpaths.HarnessDirNameClaudeCode)
	registerInjectionHarness(h.opencode, catalogpaths.HarnessDirNameOpenCode)
	hostRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hostRoot, "MosaicDeploy", "harnesses"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	res := deps.Catalog.(stack.Catalog).HarnessInjections()
	reg, err := registry.DiscoverForStack(registry.Options{MosaicRoot: hostRoot}, res)
	if err != nil {
		t.Fatalf("DiscoverForStack: %v", err)
	}
	deps.Registry = reg
	return app.New(deps), h
}

func TestCheckHarnessInjections_LayerSuppliesTheHarness_Passes(t *testing.T) {
	svc, h := newInjectionCheckService(t, true)

	err := svc.CheckHarnessInjections(h.claude)

	if err != nil {
		t.Errorf("CheckHarnessInjections(%q) = %v, want nil: the user layer supplies the Claude Code injections", h.claude, err)
	}
}

func TestCheckHarnessInjections_LowerLayerSuppliesTheHarness_Passes(t *testing.T) {
	svc, h := newInjectionCheckService(t, true)

	err := svc.CheckHarnessInjections(h.opencode)

	if err != nil {
		t.Errorf("CheckHarnessInjections(%q) = %v, want nil: the default layer supplies the OpenCode injections", h.opencode, err)
	}
}

func TestCheckHarnessInjections_NoLayerSuppliesTheHarness_ReturnsTheNamedInjectionError(t *testing.T) {
	svc, h := newInjectionCheckService(t, false)

	err := svc.CheckHarnessInjections(h.claude)

	var notSupplied *injections.NotSuppliedError
	if !errors.As(err, &notSupplied) {
		t.Fatalf("CheckHarnessInjections(%q) = %v, want an error wrapping *injections.NotSuppliedError", h.claude, err)
	}
	if notSupplied.HarnessDir != catalogpaths.HarnessDirNameClaudeCode {
		t.Errorf("NotSuppliedError.HarnessDir = %q, want %q", notSupplied.HarnessDir, catalogpaths.HarnessDirNameClaudeCode)
	}
}

func TestCheckHarnessInjections_OtherHarnessIsUnaffectedByTheMissingOne(t *testing.T) {
	svc, h := newInjectionCheckService(t, false)

	err := svc.CheckHarnessInjections(h.opencode)

	if err != nil {
		t.Errorf("CheckHarnessInjections(%q) = %v, want nil: the default layer supplies the OpenCode injections", h.opencode, err)
	}
}

func TestCheckHarnessInjections_UnknownHarness_ReturnsTheResolveError(t *testing.T) {
	svc, _ := newInjectionCheckService(t, true)

	err := svc.CheckHarnessInjections("no-such-harness")

	if !errors.Is(err, registry.ErrHarnessNotFound) {
		t.Errorf("CheckHarnessInjections(unknown) = %v, want an error wrapping registry.ErrHarnessNotFound", err)
	}
}
