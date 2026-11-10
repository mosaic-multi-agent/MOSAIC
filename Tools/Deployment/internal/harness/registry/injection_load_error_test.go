package registry_test

// injection_load_error_test.go verifies how the stack-backed registry reports a built-in whose
// winning catalog layer supplies a HarnessInjections directory that lacks a required file or
// holds an unparseable one. Injections are never merged across layers, so the failure must name
// the supplying layer (id and root), the harness directory and the file, and say the layer has to
// supply a valid file or drop the directory. The harness stays listed but unusable.
//
// Fixtures are layers in t.TempDir() resolved through injections.Resolve, the same way the stack
// builds its result.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/injectionfile"
	"mosaic-deploy/internal/harness/registry"
)

const (
	injLoadValidDoc = "---\nversion: \"1.0.0\"\nharness: claude-code\n---\n\n" +
		"<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"
	injLoadUnparseableDoc = "---\nversion: \"1.0.0\"\nharness: claude-code\n---\n\n" +
		"<HarnessConstraints type=\"managed\">\nnever closed\n"
	injLoadWinnerID = "winning-layer"
)

// injLoadLayer writes a layer whose Claude Code injection directory holds the given files
// (name -> content; an empty map leaves the directory empty) and returns the layer input.
func injLoadLayer(t *testing.T, id string, files map[string]string) stackrec.LayerInput {
	t.Helper()
	root := t.TempDir()
	dir := catalogpaths.HarnessInjectionsDir(root, catalogpaths.HarnessDirNameClaudeCode)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	return stackrec.LayerInput{
		Layer:     domain.CatalogLayer{Identity: domain.CatalogIdentity{ID: id, Version: "1.0.0"}, Root: root},
		Overrides: identity.Overrides{},
	}
}

func injLoadValidFiles() map[string]string {
	return map[string]string{
		injectionfile.SharedFileName:       injLoadValidDoc,
		injectionfile.OrchestratorFileName: injLoadValidDoc,
	}
}

// injLoadStack resolves a stack whose winning layer holds winnerFiles over a default layer with
// complete, valid files, and returns the result with the winning layer.
func injLoadStack(t *testing.T, winnerFiles map[string]string) (injections.Result, domain.CatalogLayer) {
	t.Helper()
	winner := injLoadLayer(t, injLoadWinnerID, winnerFiles)
	fallback := injLoadLayer(t, "default-layer", injLoadValidFiles())
	return injections.Resolve([]stackrec.LayerInput{winner, fallback}), winner.Layer
}

// registerLoadingBuiltin registers a built-in that, like the real ones, loads its harness
// content from the Claude Code injection directory its resolver returns and wraps a load failure.
func registerLoadingBuiltin(t *testing.T) string {
	t.Helper()
	id := uniqueID(t, "inj-load")
	registry.Register(id, func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		dir, err := opts.ResolveContentDir(catalogpaths.HarnessDirNameClaudeCode)
		if err != nil {
			return nil, err
		}
		if _, err := injectionfile.LoadDir(dir); err != nil {
			return nil, fmt.Errorf("%s: load harness content: %w", id, err)
		}
		return stubModule(id), nil
	})
	return id
}

type injLoadCase struct {
	name        string
	files       map[string]string
	wantFile    string
	wantMissing bool
}

func injLoadCases() []injLoadCase {
	return []injLoadCase{
		{"empty directory", map[string]string{}, injectionfile.SharedFileName, true},
		{"orchestrator file missing", map[string]string{injectionfile.SharedFileName: injLoadValidDoc},
			injectionfile.OrchestratorFileName, true},
		{"shared file unparseable", map[string]string{injectionfile.SharedFileName: injLoadUnparseableDoc,
			injectionfile.OrchestratorFileName: injLoadValidDoc}, injectionfile.SharedFileName, false},
		{"orchestrator file unparseable", map[string]string{injectionfile.SharedFileName: injLoadValidDoc,
			injectionfile.OrchestratorFileName: injLoadUnparseableDoc}, injectionfile.OrchestratorFileName, false},
	}
}

// stackBuilders returns the two stack-backed construction paths: discovery for the stack, and
// the rebuild of a registry that was first discovered for a healthy stack.
func stackBuilders(t *testing.T) map[string]func(opts registry.Options, res injections.Result) registry.Registry {
	return map[string]func(opts registry.Options, res injections.Result) registry.Registry{
		"DiscoverForStack": func(opts registry.Options, res injections.Result) registry.Registry {
			reg, err := registry.DiscoverForStack(opts, res)
			if err != nil {
				t.Fatalf("DiscoverForStack: %v", err)
			}
			return reg
		},
		"RebuildForStack": func(opts registry.Options, res injections.Result) registry.Registry {
			healthy := injections.Resolve([]stackrec.LayerInput{injLoadLayer(t, "healthy", injLoadValidFiles())})
			first, err := registry.DiscoverForStack(opts, healthy)
			if err != nil {
				t.Fatalf("DiscoverForStack (healthy): %v", err)
			}
			reg, err := first.(registry.Rebuilder).RebuildForStack(res)
			if err != nil {
				t.Fatalf("RebuildForStack: %v", err)
			}
			return reg
		},
	}
}

func TestResolve_WinningLayerInjectionsIncomplete_ErrorNamesLayerDirectoryFileAndGuidance(t *testing.T) {
	for path, build := range stackBuilders(t) {
		for _, tc := range injLoadCases() {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				id := registerLoadingBuiltin(t)
				res, winner := injLoadStack(t, tc.files)
				reg := build(registry.Options{MosaicRoot: makeRoot(t)}, res)

				_, err := reg.Resolve(id)

				if err == nil {
					t.Fatal("Resolve returned nil; a winning layer with an incomplete injection directory must fail")
				}
				text := err.Error()
				for what, want := range map[string]string{
					"layer id":          winner.Identity.ID,
					"layer root":        winner.Root,
					"harness directory": catalogpaths.HarnessDirNameClaudeCode,
					"file":              tc.wantFile,
					"not-merged note":   "not merged",
					"built-in prefix":   `harness "` + id + `" is not usable: failed to initialise built-in: `,
					"factory text":      "injectionfile: ",
				} {
					if !strings.Contains(text, want) {
						t.Errorf("error %q does not contain the %s %q", text, what, want)
					}
				}
			})
		}
	}
}

func TestResolve_WinningLayerInjectionsIncomplete_ErrorIsTypedAndKeepsItsCauses(t *testing.T) {
	for path, build := range stackBuilders(t) {
		for _, tc := range injLoadCases() {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				id := registerLoadingBuiltin(t)
				res, winner := injLoadStack(t, tc.files)
				reg := build(registry.Options{MosaicRoot: makeRoot(t)}, res)

				_, err := reg.Resolve(id)

				var attributed *registry.InjectionLoadError
				if !errors.As(err, &attributed) {
					t.Fatalf("Resolve error = %v; want an error wrapping *registry.InjectionLoadError", err)
				}
				if attributed.Resolution.Supplier.Identity.ID != winner.Identity.ID ||
					attributed.Resolution.Supplier.Root != winner.Root {
					t.Errorf("InjectionLoadError supplier = %+v, want the winning layer %+v", attributed.Resolution.Supplier, winner)
				}
				if attributed.Resolution.HarnessDir != catalogpaths.HarnessDirNameClaudeCode {
					t.Errorf("InjectionLoadError.Resolution.HarnessDir = %q, want %q",
						attributed.Resolution.HarnessDir, catalogpaths.HarnessDirNameClaudeCode)
				}
				var loadErr *injectionfile.LoadError
				if !errors.As(err, &loadErr) || loadErr.File != tc.wantFile {
					t.Errorf("Resolve error does not reach a *injectionfile.LoadError for %q (got %+v)", tc.wantFile, loadErr)
				}
				if got := errors.Is(err, fs.ErrNotExist); got != tc.wantMissing {
					t.Errorf("errors.Is(err, fs.ErrNotExist) = %v, want %v", got, tc.wantMissing)
				}
			})
		}
	}
}

func TestList_WinningLayerInjectionsIncomplete_HarnessStaysListedUnusableNamingTheLayer(t *testing.T) {
	for path, build := range stackBuilders(t) {
		t.Run(path, func(t *testing.T) {
			id := registerLoadingBuiltin(t)
			res, winner := injLoadStack(t, map[string]string{})
			reg := build(registry.Options{MosaicRoot: makeRoot(t)}, res)

			var ref *domain.HarnessRef
			for _, r := range reg.List() {
				if r.ID == id {
					r := r
					ref = &r
				}
			}

			if ref == nil {
				t.Fatalf("harness %q missing from List(); it must stay listed", id)
			}
			if ref.Usable {
				t.Error("Usable = true, want false")
			}
			if !strings.Contains(ref.UnusableReason, winner.Identity.ID) || !strings.Contains(ref.UnusableReason, "not merged") {
				t.Errorf("UnusableReason = %q, want it to name layer %q and the not-merged note", ref.UnusableReason, winner.Identity.ID)
			}
		})
	}
}

func TestResolve_NonStackResolverInjectionsIncomplete_ErrorStaysUnattributedAndNamesTheFile(t *testing.T) {
	id := registerLoadingBuiltin(t)
	dir := injLoadLayer(t, "plain", map[string]string{}).Layer.Root
	reg, err := registry.Discover(registry.Options{
		MosaicRoot: makeRoot(t),
		BuiltinContentDir: func(harnessDirName string) (string, error) {
			return catalogpaths.HarnessInjectionsDir(dir, harnessDirName), nil
		},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	_, resolveErr := reg.Resolve(id)

	var attributed *registry.InjectionLoadError
	if errors.As(resolveErr, &attributed) {
		t.Errorf("Resolve error = %v; a resolver without a layer must not produce an *InjectionLoadError", resolveErr)
	}
	var loadErr *injectionfile.LoadError
	if !errors.As(resolveErr, &loadErr) || loadErr.File != injectionfile.SharedFileName {
		t.Errorf("Resolve error = %v; want it to reach a *injectionfile.LoadError for %s", resolveErr, injectionfile.SharedFileName)
	}
}
