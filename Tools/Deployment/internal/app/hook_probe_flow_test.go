package app_test

// hook_probe_flow_test.go verifies, through the DeployHooks service, that the deployed-hook
// state reaching the planner reports each hook file the deploy writes (flat or nested in the
// hooks directory) with its own path and the hash and Deploy-owned record of its real bytes, and
// that the real planner then plans one item per file from the per-file manifest entries.
//
// Fixtures are temp-dir workspaces and stub harness modules; nothing reads Catalog/.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
	"mosaic-deploy/internal/transform"
)

const (
	flowHooksDir   = ".claude/hooks"
	flowHookKey    = "git-hooks"
	flowHookTarget = flowHooksDir + "/" + flowHookKey

	flowHookFlatPath   = flowHooksDir + "/logger.sh"
	flowHookNestedPath = flowHooksDir + "/lib/util.sh"
	flowHookFlatText   = "#!/bin/sh\necho logger\n"
	flowHookNestedText = "#!/bin/sh\necho util\n"
)

// flatHookModule is a harness module whose hook bundles deploy flat into flowHooksDir
// (target path <hooks dir>/<key>, files written as <hooks dir>/<TargetName>).
type flatHookModule struct {
	*stubHarnessModule
}

func (m flatHookModule) TargetPath(req domain.TargetPathRequest) (string, error) {
	if req.Kind == domain.ArtifactHook {
		return flowHooksDir + "/" + req.Key, nil
	}
	return m.stubHarnessModule.TargetPath(req)
}

func (m flatHookModule) HookPlan(_ domain.HookPlanRequest) (domain.HookPlan, error) {
	return domain.HookPlan{
		Supported: true,
		TargetDir: flowHooksDir,
		Files: []domain.HookFile{
			{SourcePath: "/src/logger.sh", TargetName: "logger.sh"},
			{SourcePath: "/src/lib/util.sh", TargetName: "lib/util.sh"},
		},
	}, nil
}

// writeFlowHookFile writes one hook file of the workspace.
func writeFlowHookFile(t *testing.T, workspace, rel, text string) {
	t.Helper()
	full := filepath.Join(workspace, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(text), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// newFlatHookDeps returns deps whose catalog holds one hook bundle at catalogVersion, whose
// harness deploys it flat, and whose workspace already contains both deployed files when
// deployed is true.
func newFlatHookDeps(t *testing.T, catalogVersion string, deployed bool) (app.Deps, string) {
	t.Helper()
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newBaseDeps(t, stub)

	cat := newMinimalCatalog()
	cat.hooks = []domain.HookBundle{{Key: flowHookKey, Version: catalogVersion}}
	deps.Catalog = cat

	module := flatHookModule{stubHarnessModule: newMinimalModule()}
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{minimalHarness},
		modules: map[string]domain.HarnessModule{"stub-harness": module},
	}

	if deployed {
		// Only the nested file: the bundle is partly deployed (damaged).
		writeFlowHookFile(t, workspace, flowHookNestedPath, flowHookNestedText)
	}
	return deps, workspace
}

// newDeployedHookDeps is newFlatHookDeps with both files of the bundle on disk.
func newDeployedHookDeps(t *testing.T, catalogVersion string) (app.Deps, string) {
	t.Helper()
	deps, workspace := newFlatHookDeps(t, catalogVersion, true)
	writeFlowHookFile(t, workspace, flowHookFlatPath, flowHookFlatText)
	return deps, workspace
}

// flowHookEntry is the per-file entry of a hook file deployed with text at recordedVersion.
func flowHookEntry(t *testing.T, path, text, recordedVersion string) domain.ManifestEntry {
	t.Helper()
	owned, err := transform.OwnedContentOf(domain.ArtifactHook, []byte(text))
	if err != nil {
		t.Fatalf("OwnedContentOf: %v", err)
	}
	return domain.ManifestEntry{
		Harness:     "stub-harness",
		Ref:         domain.ArtifactRef{Kind: domain.ArtifactHook, Key: flowHookKey},
		TargetPath:  path,
		Version:     recordedVersion,
		ContentHash: manifest.Hash([]byte(text)),
		Owned:       &owned,
	}
}

// manifestWithHook returns a usable manifest snapshot recording both hook files at
// recordedVersion, as the executor records them after a deploy.
func manifestWithHook(t *testing.T, recordedVersion string) *stubManifestStore {
	t.Helper()
	return manifestWithEntries(
		flowHookEntry(t, flowHookFlatPath, flowHookFlatText, recordedVersion),
		flowHookEntry(t, flowHookNestedPath, flowHookNestedText, recordedVersion),
	)
}

// manifestWithEntries returns a usable manifest snapshot holding the entries.
func manifestWithEntries(entries ...domain.ManifestEntry) *stubManifestStore {
	return &stubManifestStore{snap: manifest.Snapshot{
		State:    manifest.StatePresent,
		Manifest: domain.Manifest{Entries: entries},
	}}
}

func runFlatDeployHooks(t *testing.T, deps app.Deps, workspace string) {
	t.Helper()
	_, err := app.New(deps).DeployHooks(context.Background(), app.DeployHooksRequest{
		HarnessID:       "stub-harness",
		WorkspacePath:   workspace,
		Scope:           domain.ScopeProject,
		HookIDs:         []string{flowHookKey},
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("DeployHooks: %v", err)
	}
}

// TestDeployHooks_DeployedFiles_PlannerSeesEachFileWithItsRealContent verifies that every hook
// file on disk reaches the planner as present under its own path, carrying the hash and the
// Deploy-owned record of the bytes on disk.
func TestDeployHooks_DeployedFiles_PlannerSeesEachFileWithItsRealContent(t *testing.T) {
	deps, workspace := newDeployedHookDeps(t, "1.0")
	capPlan := &capturingPlanner{result: hooksPlan(workspace)}
	deps.Planner = capPlan

	runFlatDeployHooks(t, deps, workspace)

	if capPlan.capturedInput == nil {
		t.Fatal("planner was not called")
	}
	for path, text := range map[string]string{flowHookFlatPath: flowHookFlatText, flowHookNestedPath: flowHookNestedText} {
		state, ok := capPlan.capturedInput.DeployedState[path]
		if !ok || !state.Present {
			t.Fatalf("DeployedState[%q] = %+v, present %v; want a present file", path, state, ok)
		}
		if want := manifest.Hash([]byte(text)); state.ContentHash != want {
			t.Errorf("DeployedState[%q].ContentHash = %q; want the hash of the real bytes %q", path, state.ContentHash, want)
		}
		owned, err := transform.OwnedContentOf(domain.ArtifactHook, []byte(text))
		if err != nil {
			t.Fatalf("OwnedContentOf: %v", err)
		}
		if state.Owned == nil || state.Owned.Hash != owned.Hash {
			t.Errorf("DeployedState[%q].Owned = %+v; want the whole-file record %q", path, state.Owned, owned.Hash)
		}
	}
}

// TestDeployHooks_OnlyOneFileOnDisk_OtherFileReportedAbsent verifies that presence is decided
// per file: a bundle with one file missing reports that file absent and the other present.
func TestDeployHooks_OnlyOneFileOnDisk_OtherFileReportedAbsent(t *testing.T) {
	deps, workspace := newFlatHookDeps(t, "1.0", false)
	writeFlowHookFile(t, workspace, flowHookNestedPath, flowHookNestedText)
	capPlan := &capturingPlanner{result: hooksPlan(workspace)}
	deps.Planner = capPlan

	runFlatDeployHooks(t, deps, workspace)

	got := capPlan.capturedInput.DeployedState
	if !got[flowHookNestedPath].Present {
		t.Errorf("DeployedState[%q].Present = false for a file on disk", flowHookNestedPath)
	}
	if got[flowHookFlatPath].Present {
		t.Errorf("DeployedState[%q].Present = true for a file that is not on disk", flowHookFlatPath)
	}
}

// TestDeployHooks_NoFilesOnDisk_PlannerSeesEveryFileAbsent verifies a bundle with no deployed
// files reaches the planner as absent, file by file.
func TestDeployHooks_NoFilesOnDisk_PlannerSeesEveryFileAbsent(t *testing.T) {
	deps, workspace := newFlatHookDeps(t, "1.0", false)
	capPlan := &capturingPlanner{result: hooksPlan(workspace)}
	deps.Planner = capPlan

	runFlatDeployHooks(t, deps, workspace)

	if capPlan.capturedInput == nil {
		t.Fatal("planner was not called")
	}
	for _, path := range []string{flowHookFlatPath, flowHookNestedPath} {
		state, ok := capPlan.capturedInput.DeployedState[path]
		if !ok {
			t.Errorf("DeployedState has no key for %q; each hook file must be reported", path)
		}
		if state.Present {
			t.Errorf("DeployedState[%q].Present = true with no files on disk", path)
		}
	}
}

// capturePlannedHookItems runs DeployHooks with the real planner and returns the hook plan items
// by target path.
func capturePlannedHookItems(t *testing.T, deps app.Deps, workspace string) map[string]domain.PlanItem {
	t.Helper()
	capExec := &planCapturingExecutor{result: newMinimalExecResult(workspace)}
	deps.Planner = plan.New()
	deps.Executor = capExec

	runFlatDeployHooks(t, deps, workspace)

	if capExec.capturedPlan == nil {
		t.Fatal("executor was not called; no plan captured")
	}
	items := map[string]domain.PlanItem{}
	for _, it := range capExec.capturedPlan.Items {
		if it.Ref.Kind == domain.ArtifactHook && it.Ref.Key == flowHookKey {
			items[it.TargetPath] = it
		}
	}
	if len(items) != 2 {
		t.Fatalf("hook items = %v; want one per hook file", items)
	}
	return items
}

// TestDeployHooks_DeployedBundleOlderRecordedVersion_EveryFileUpdated verifies that with the
// bundle's files on disk and an older manifest-recorded version, the real planner reports an
// update with a version delta for each file, not a new hook bundle.
func TestDeployHooks_DeployedBundleOlderRecordedVersion_EveryFileUpdated(t *testing.T) {
	deps, workspace := newDeployedHookDeps(t, "2.0")
	deps.Manifest = manifestWithHook(t, "1.0")

	for path, item := range capturePlannedHookItems(t, deps, workspace) {
		if item.Action != domain.ActionUpdate || len(item.Stale) == 0 {
			t.Errorf("%s: action = %v (reason %q), stale = %v; want ActionUpdate with a version delta", path, item.Action, reasonText(item), item.Stale)
		}
	}
}

// TestDeployHooks_DeployedBundleSameRecordedVersion_EveryFileUnchanged verifies that with the
// bundle's files on disk and an equal manifest-recorded version, every file is unchanged.
func TestDeployHooks_DeployedBundleSameRecordedVersion_EveryFileUnchanged(t *testing.T) {
	deps, workspace := newDeployedHookDeps(t, "1.0")
	deps.Manifest = manifestWithHook(t, "1.0")

	for path, item := range capturePlannedHookItems(t, deps, workspace) {
		if item.Action != domain.ActionUnchanged {
			t.Errorf("%s: action = %v (reason %q), want ActionUnchanged", path, item.Action, reasonText(item))
		}
	}
}

// TestDeployHooks_EditedHookFile_UpdateWithBackupMarker verifies the edit rule end to end
// through the probe: a file whose bytes no longer reproduce its record is an update that backs
// the file up first.
func TestDeployHooks_EditedHookFile_UpdateWithBackupMarker(t *testing.T) {
	deps, workspace := newDeployedHookDeps(t, "1.0")
	deps.Manifest = manifestWithHook(t, "1.0")
	writeFlowHookFile(t, workspace, flowHookFlatPath, "#!/bin/sh\necho edited\n")

	items := capturePlannedHookItems(t, deps, workspace)

	edited := items[flowHookFlatPath]
	if edited.Action != domain.ActionUpdate || !edited.BackupBeforeWrite {
		t.Errorf("edited file: action = %v, backup = %v; want update with backup", edited.Action, edited.BackupBeforeWrite)
	}
	if other := items[flowHookNestedPath]; other.Action != domain.ActionUnchanged {
		t.Errorf("untouched file: action = %v; want unchanged", other.Action)
	}
}

// TestDeployHooks_PartlyMissingBundle_MissingFileCreatedPresentFileUnchanged verifies a damaged
// bundle: both files are recorded, one is gone from disk. The missing file is created and the
// present, unedited file is left unchanged.
func TestDeployHooks_PartlyMissingBundle_MissingFileCreatedPresentFileUnchanged(t *testing.T) {
	deps, workspace := newFlatHookDeps(t, "1.0", true) // only the nested file is on disk
	deps.Manifest = manifestWithHook(t, "1.0")

	items := capturePlannedHookItems(t, deps, workspace)

	if got := items[flowHookFlatPath]; got.Action != domain.ActionCreate {
		t.Errorf("missing file: action = %v (reason %q), want ActionCreate", got.Action, reasonText(got))
	}
	if got := items[flowHookNestedPath]; got.Action != domain.ActionUnchanged {
		t.Errorf("present file: action = %v (reason %q), want ActionUnchanged", got.Action, reasonText(got))
	}
}

// TestDeployHooks_PreChangeBundleEntryWithAbsentFile_FileCreatedWithoutConflict verifies that a
// bundle-level record from before per-file records does not stop an absent file being created.
func TestDeployHooks_PreChangeBundleEntryWithAbsentFile_FileCreatedWithoutConflict(t *testing.T) {
	deps, workspace := newFlatHookDeps(t, "1.0", true) // only the nested file is on disk
	deps.Manifest = manifestWithEntries(domain.ManifestEntry{
		Harness:     "stub-harness",
		Ref:         domain.ArtifactRef{Kind: domain.ArtifactHook, Key: flowHookKey},
		TargetPath:  flowHookTarget,
		Version:     "1.0",
		ContentHash: manifest.Hash(nil),
	})

	items := capturePlannedHookItems(t, deps, workspace)

	got := items[flowHookFlatPath]
	if got.Action != domain.ActionCreate || got.Conflict != nil {
		t.Errorf("absent file: action = %v, conflict = %+v; want a plain create", got.Action, got.Conflict)
	}
}

// TestDeployHooks_AbsentBundleWithStaleManifestEntries_EveryFileCreated verifies that manifest
// entries alone do not make a bundle deployed: with no files on disk every file is created.
func TestDeployHooks_AbsentBundleWithStaleManifestEntries_EveryFileCreated(t *testing.T) {
	deps, workspace := newFlatHookDeps(t, "2.0", false)
	deps.Manifest = manifestWithHook(t, "1.0")

	for path, item := range capturePlannedHookItems(t, deps, workspace) {
		if item.Action != domain.ActionCreate {
			t.Errorf("%s: action = %v (reason %q), want ActionCreate", path, item.Action, reasonText(item))
		}
	}
}

// TestDeployHooks_PreChangeBundleEntry_FilesOverwrittenWithoutConflict verifies that a manifest
// holding only the bundle-level record from before per-file records plans the files on disk as
// updates, not as conflicts, and without a backup.
func TestDeployHooks_PreChangeBundleEntry_FilesOverwrittenWithoutConflict(t *testing.T) {
	deps, workspace := newDeployedHookDeps(t, "1.0")
	bundle := domain.ManifestEntry{
		Harness:     "stub-harness",
		Ref:         domain.ArtifactRef{Kind: domain.ArtifactHook, Key: flowHookKey},
		TargetPath:  flowHookTarget,
		Version:     "1.0",
		ContentHash: manifest.Hash(nil),
	}
	deps.Manifest = manifestWithEntries(bundle)

	for path, item := range capturePlannedHookItems(t, deps, workspace) {
		if item.Action != domain.ActionUpdate || item.Conflict != nil || item.BackupBeforeWrite {
			t.Errorf("%s: action = %v, conflict = %+v, backup = %v; want a plain update", path, item.Action, item.Conflict, item.BackupBeforeWrite)
		}
	}
}
