package app_test

// injection_load_error_test.go verifies what the app service reports when the winning catalog
// layer supplies an incomplete or unparseable HarnessInjections directory for the selected
// harness: the stack-screen check and a deploy both fail with an error naming the layer (id and
// root), the harness directory and the file and saying injections are not merged across layers,
// and the deploy executes nothing and leaves the workspace empty.
//
// Fixtures are temp-dir catalogs: the default layer supplies complete, valid injections, the
// user layer on top wins the harness directory with whatever incomplete content a test gives it.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/injectionfile"
	"mosaic-deploy/internal/harness/registry"
)

const (
	incompleteInjValidDoc = "---\nversion: \"1.0.0\"\nharness: claude-code\n---\n\n" +
		"<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"
	incompleteInjBrokenDoc = "---\nversion: \"1.0.0\"\nharness: claude-code\n---\n\n" +
		"<HarnessConstraints type=\"managed\">\nnever closed\n"
	incompleteInjWinnerID = "inj-winner"
)

// incompleteInjCase is one way the winning layer's Claude Code injection directory is defective.
type incompleteInjCase struct {
	name     string
	files    map[string]string
	wantFile string
}

func incompleteInjCases() []incompleteInjCase {
	return []incompleteInjCase{
		{"empty directory", map[string]string{}, injectionfile.SharedFileName},
		{"orchestrator file missing", map[string]string{injectionfile.SharedFileName: incompleteInjValidDoc},
			injectionfile.OrchestratorFileName},
		{"shared file unparseable", map[string]string{injectionfile.SharedFileName: incompleteInjBrokenDoc,
			injectionfile.OrchestratorFileName: incompleteInjValidDoc}, injectionfile.SharedFileName},
	}
}

// newIncompleteInjService builds a service over a real two-layer stack whose winning user layer
// holds the given Claude Code injection files over a default layer with complete, valid files.
// It returns the service, the registered harness id, the winning layer root, the workspace and
// the executor that records every write attempt.
func newIncompleteInjService(t *testing.T, files map[string]string) (app.Service, string, string, string, *recordingExecutor) {
	t.Helper()
	deps, workspace := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	mosaic := newSysMosaicRoot(t, sysLayer{id: "inj-default"})
	for _, name := range []string{injectionfile.SharedFileName, injectionfile.OrchestratorFileName} {
		writeSysFile(t, filepath.Join(catalogpaths.HarnessInjectionsDir(filepath.Join(mosaic, catalogpaths.CatalogDirName),
			catalogpaths.HarnessDirNameClaudeCode), name), incompleteInjValidDoc)
	}
	user := newSysUserLayer(t, sysLayer{id: incompleteInjWinnerID})
	userDir := catalogpaths.HarnessInjectionsDir(user, catalogpaths.HarnessDirNameClaudeCode)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", userDir, err)
	}
	for name, content := range files {
		writeSysFile(t, filepath.Join(userDir, name), content)
	}
	useResolvedStack(t, &deps, mosaic, stack.Layer{Root: user}, defaultStackLayer(mosaic))

	id := "inj-incomplete-" + filepath.Base(t.TempDir())
	registry.Register(id, func(opts registry.BuiltinOptions) (domain.HarnessModule, error) {
		dir, err := opts.ResolveContentDir(catalogpaths.HarnessDirNameClaudeCode)
		if err != nil {
			return nil, err
		}
		if _, err := injectionfile.LoadDir(dir); err != nil {
			return nil, fmt.Errorf("%s: load harness content: %w", id, err)
		}
		return &appTestModule{desc: &domain.HarnessDescriptor{ID: id}}, nil
	})
	hostRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hostRoot, "MosaicDeploy", "harnesses"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reg, err := registry.DiscoverForStack(registry.Options{MosaicRoot: hostRoot}, deps.Catalog.(stack.Catalog).HarnessInjections())
	if err != nil {
		t.Fatalf("DiscoverForStack: %v", err)
	}
	exec := &recordingExecutor{}
	deps.Registry = reg
	deps.Executor = exec
	return app.New(deps), id, user, workspace, exec
}

// assertAttributedInjectionError fails unless the error text carries the layer, directory,
// file and not-merged guidance.
func assertAttributedInjectionError(t *testing.T, err error, layerRoot, file string) {
	t.Helper()
	if err == nil {
		t.Fatal("got nil error; an incomplete winning injection directory must fail")
	}
	text := err.Error()
	for what, want := range map[string]string{
		"layer id":          incompleteInjWinnerID,
		"layer root":        layerRoot,
		"harness directory": catalogpaths.HarnessDirNameClaudeCode,
		"file":              file,
		"not-merged note":   "not merged",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("error %q does not contain the %s %q", text, what, want)
		}
	}
	var attributed *registry.InjectionLoadError
	if !errors.As(err, &attributed) {
		t.Errorf("error %v does not wrap *registry.InjectionLoadError", err)
	}
}

func TestCheckHarnessInjections_WinningLayerIncomplete_ErrorNamesLayerDirectoryAndFile(t *testing.T) {
	for _, tc := range incompleteInjCases() {
		t.Run(tc.name, func(t *testing.T) {
			svc, id, userRoot, _, _ := newIncompleteInjService(t, tc.files)

			err := svc.CheckHarnessInjections(id)

			assertAttributedInjectionError(t, err, userRoot, tc.wantFile)
		})
	}
}

func TestDeployNew_WinningLayerInjectionsIncomplete_FailsNamingLayerAndWritesNothing(t *testing.T) {
	for _, tc := range incompleteInjCases() {
		t.Run(tc.name, func(t *testing.T) {
			svc, id, userRoot, workspace, exec := newIncompleteInjService(t, tc.files)

			_, err := svc.DeployNew(context.Background(), app.DeployRequest{
				HarnessID:       id,
				WorkspacePath:   workspace,
				WorkflowIDs:     []string{"quick-fix"},
				AutoConfirmPlan: true,
			})

			assertAttributedInjectionError(t, err, userRoot, tc.wantFile)
			if len(exec.reqs) != 0 {
				t.Errorf("executor was called %d time(s); a failed injection load must write nothing", len(exec.reqs))
			}
			if entries, readErr := os.ReadDir(workspace); readErr != nil || len(entries) != 0 {
				t.Errorf("workspace holds %d entries (err %v); a failed injection load must leave it empty", len(entries), readErr)
			}
		})
	}
}

func TestCheckHarnessInjections_WinningLayerIncomplete_CheckAndDeployBothAttributed(t *testing.T) {
	svc, id, userRoot, workspace, _ := newIncompleteInjService(t, map[string]string{})

	checkErr := svc.CheckHarnessInjections(id)
	_, deployErr := svc.DeployNew(context.Background(), app.DeployRequest{
		HarnessID: id, WorkspacePath: workspace, WorkflowIDs: []string{"quick-fix"}, AutoConfirmPlan: true,
	})

	assertAttributedInjectionError(t, checkErr, userRoot, injectionfile.SharedFileName)
	assertAttributedInjectionError(t, deployErr, userRoot, injectionfile.SharedFileName)
}
