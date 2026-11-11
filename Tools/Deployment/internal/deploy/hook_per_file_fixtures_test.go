package deploy_test

// hook_per_file_fixtures_test.go holds the builders for the per-file hook executor tests: a
// two-file hook bundle whose source files live in a temporary directory, one plan item per
// file, and helpers that run a request and read the hook entries of the result. Nothing in this
// file is a test itself.

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/manifest/manifesttest"
)

const (
	hookBundleKey = "mosaic-logger"
	hookDir       = ".github/hooks"
	hookFlatRel   = ".github/hooks/logger.json"
	hookNestedRel = ".github/hooks/lib/logger.sh"
	hookRegRel    = ".github/settings.json"

	hookFlatSource   = "{\"hook\":1}\n"
	hookNestedSource = "#!/bin/sh\necho hi\n"
	hookOnDiskEdit   = "# hand edited\n"
)

var perFileHookRef = domain.ArtifactRef{Kind: domain.ArtifactHook, Key: hookBundleKey}

// hookBundleFixture is a workspace plus the source files of the two-file bundle.
type hookBundleFixture struct {
	workspace string
	flatSrc   string
	nestedSrc string
}

func newHookBundleFixture(t *testing.T) hookBundleFixture {
	t.Helper()
	src := t.TempDir()
	flat := filepath.Join(src, "logger.json")
	nested := filepath.Join(src, "lib", "logger.sh")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(flat, []byte(hookFlatSource), 0o644); err != nil {
		t.Fatalf("write flat source: %v", err)
	}
	if err := os.WriteFile(nested, []byte(hookNestedSource), 0o644); err != nil {
		t.Fatalf("write nested source: %v", err)
	}
	return hookBundleFixture{workspace: t.TempDir(), flatSrc: flat, nestedSrc: nested}
}

// flatItem and nestedItem are the plan items of the two files with the given action.
func (f hookBundleFixture) flatItem(action domain.PlanAction) domain.PlanItem {
	return domain.PlanItem{Ref: perFileHookRef, SourcePath: f.flatSrc, TargetPath: hookFlatRel, Action: action}
}

func (f hookBundleFixture) nestedItem(action domain.PlanAction) domain.PlanItem {
	return domain.PlanItem{Ref: perFileHookRef, SourcePath: f.nestedSrc, TargetPath: hookNestedRel, Action: action}
}

// registration is the hook plan of the bundle with one performable registration step. Its
// Files are the same files the plan items name; the executor writes hook files from the items.
func (f hookBundleFixture) registration() []domain.HookPlan {
	return []domain.HookPlan{{
		Supported: true,
		TargetDir: hookDir,
		Files: []domain.HookFile{
			{SourcePath: f.flatSrc, TargetName: "logger.json"},
			{SourcePath: f.nestedSrc, TargetName: "lib/logger.sh"},
		},
		Registration: []domain.RegistrationStep{registrationStep(hookRegRel, "{\"registered\":true}")},
	}}
}

// hookContent fails for hook items: hook files come from their own source file, never through
// the content callback.
func hookContent(it domain.PlanItem) ([]byte, error) {
	if it.Ref.Kind == domain.ArtifactHook {
		return nil, errors.New("hook item routed through the content callback")
	}
	return []byte("# agent"), nil
}

// executeHooks runs the plan against the fixture workspace with the default store.
func (f hookBundleFixture) executeHooks(t *testing.T, items []domain.PlanItem, mutate func(*deploy.ExecRequest)) deploy.ExecResult {
	t.Helper()
	req := deploy.ExecRequest{
		Plan:       newPlan(f.workspace, items...),
		MosaicRoot: t.TempDir(),
		Content:    hookContent,
	}
	if mutate != nil {
		mutate(&req)
	}
	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

// hookEntriesByPath returns the hook entries of the manifest by target path.
func hookEntriesByPath(m domain.Manifest) map[string]domain.ManifestEntry {
	out := map[string]domain.ManifestEntry{}
	for _, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactHook {
			out[e.TargetPath] = e
		}
	}
	return out
}

// seedHookEntries saves a manifest holding the given hook entries under the run's harness.
func seedHookEntries(t *testing.T, workspace string, entries ...domain.ManifestEntry) {
	t.Helper()
	m := domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Harnesses:     []domain.HarnessSection{{HarnessID: "test-harness"}},
		Entries:       manifesttest.WithHarness("test-harness", entries),
	}
	if err := manifest.NewStore().Save(workspace, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
}

// hookItemsFor returns the plan items that write the files of the supported hook plans: one item
// per file, an update when the file already exists in the workspace and a create otherwise.
// Hook files are written from plan items, so tests that deploy a hook plan pass these items.
func hookItemsFor(workspace string, hooks ...domain.HookPlan) []domain.PlanItem {
	var items []domain.PlanItem
	for _, hp := range hooks {
		if !hp.Supported {
			continue
		}
		for _, hf := range hp.Files {
			target := path.Join(strings.ReplaceAll(hp.TargetDir, "\\", "/"), hf.TargetName)
			action := domain.ActionCreate
			if _, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(target))); err == nil {
				action = domain.ActionUpdate
			}
			items = append(items, domain.PlanItem{
				Ref:        domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "hook-bundle"},
				SourcePath: hf.SourcePath,
				TargetPath: target,
				Action:     action,
			})
		}
	}
	return items
}

// withHookItems returns p with the hook items of the hook plans appended to its items.
func withHookItems(p domain.Plan, hooks []domain.HookPlan) domain.Plan {
	p.Items = append(append([]domain.PlanItem{}, p.Items...), hookItemsFor(p.WorkspacePath, hooks...)...)
	return p
}

// bundleLevelHookEntry is the entry recorded before per-file hook entries.
func bundleLevelHookEntry() domain.ManifestEntry {
	return domain.ManifestEntry{
		Ref:         perFileHookRef,
		TargetPath:  hookDir + "/" + hookBundleKey,
		Version:     "1.0",
		ContentHash: domain.EmptyContentHash,
	}
}
