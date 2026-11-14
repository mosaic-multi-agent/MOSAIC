package plan_test

// hook_per_file_test.go specifies the per-file planning of hook bundles: one plan item per
// deployed hook file, edit detection on the whole file, unrecorded files, bundle-level records
// from before per-file records, and damaged bundles.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// pfBothRecorded returns the per-file entries of both files with records r1 and r2.
func pfBothRecorded() []domain.ManifestEntry {
	return []domain.ManifestEntry{
		pfEntry(pfFlatPath, ownedRecord("sha256:r1")),
		pfEntry(pfNestedPath, ownedRecord("sha256:r2")),
	}
}

func pfBothDeployed(flat, nested *domain.OwnedContent) map[string]domain.DeployedArtifactState {
	return map[string]domain.DeployedArtifactState{
		pfFlatPath:   pfDeployed(flat),
		pfNestedPath: pfDeployed(nested),
	}
}

func TestBuildHook_OneItemPerDeployedFile(t *testing.T) {
	p := pfBuild(t, absentSnapshot(), nil, pfHookVersion)

	items := pfHookItems(p)
	if len(items) != 2 {
		t.Fatalf("hook items = %d (%v); want one per file", len(items), items)
	}
	for _, path := range []string{pfFlatPath, pfNestedPath} {
		it, ok := items[path]
		if !ok {
			t.Fatalf("no hook item at %q", path)
		}
		if it.Ref != hookRef(pfHookKey) {
			t.Errorf("%s: Ref = %v; want the bundle ref", path, it.Ref)
		}
	}
	if got := items[pfNestedPath].SourcePath; got != "/fake/hooks/"+pfHookKey+"/lib/logger.sh" {
		t.Errorf("nested file SourcePath = %q; want the file's own source", got)
	}
}

func TestBuildHook_RegistrationPlannedOncePerBundle(t *testing.T) {
	p := pfBuild(t, absentSnapshot(), nil, pfHookVersion)

	if len(p.Registrations) != 1 {
		t.Errorf("Registrations = %d; want 1 for a two-file bundle", len(p.Registrations))
	}
}

func TestBuildHook_FileAbsent_CreateWithNewFileReason(t *testing.T) {
	p := pfBuild(t, pfSnap(), nil, pfHookVersion)

	for path, it := range pfHookItems(p) {
		if it.Action != domain.ActionCreate || !hasExactReason(it, pfNewFileReason) {
			t.Errorf("%s: Action = %v, Reasons = %q; want create with %q", path, it.Action, it.Reasons, pfNewFileReason)
		}
	}
}

func TestBuildHook_RecordsMatch_Unchanged(t *testing.T) {
	snap := pfSnap(pfBothRecorded()...)
	deployed := pfBothDeployed(ownedRecord("sha256:r1"), ownedRecord("sha256:r2"))

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	if len(items) != 2 {
		t.Fatalf("hook items = %d; want one per file", len(items))
	}
	for path, it := range items {
		if it.Action != domain.ActionUnchanged || it.BackupBeforeWrite {
			t.Errorf("%s: Action = %v, backup = %v; want unchanged", path, it.Action, it.BackupBeforeWrite)
		}
	}
}

func TestBuildHook_EditedFile_UpdateWithReasonAndBackup(t *testing.T) {
	snap := pfSnap(pfBothRecorded()...)
	deployed := pfBothDeployed(ownedRecord("sha256:r1"), ownedRecord("sha256:edited"))

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	edited := items[pfNestedPath]
	if edited.Action != domain.ActionUpdate || !hasExactReason(edited, editReason) || !edited.BackupBeforeWrite {
		t.Errorf("edited file: Action = %v, Reasons = %q, backup = %v; want update with edit reason and backup",
			edited.Action, edited.Reasons, edited.BackupBeforeWrite)
	}
	if edited.Conflict != nil {
		t.Errorf("edited file carries a conflict %+v; an edit of a recorded file is an update", edited.Conflict)
	}
	if other := items[pfFlatPath]; other.Action != domain.ActionUnchanged || other.BackupBeforeWrite {
		t.Errorf("untouched file: Action = %v, backup = %v; want unchanged without backup", other.Action, other.BackupBeforeWrite)
	}
}

func TestBuildHook_EditedFileWhoseHashDiffersFromWriteHash_NoLongerConflict(t *testing.T) {
	// The whole-file hash of the deployed file differs from the recorded content hash, as it does
	// after any edit; only the Deploy-owned record decides.
	snap := pfSnap(pfBothRecorded()...)
	deployed := pfBothDeployed(ownedRecord("sha256:r1"), ownedRecord("sha256:r2"))

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	for path, it := range items {
		if it.Action != domain.ActionUnchanged {
			t.Errorf("%s: Action = %v; want unchanged when the Deploy-owned records match", path, it.Action)
		}
	}
}

func TestBuildHook_EditedAndStaleVersion_BothReasonsAndBackup(t *testing.T) {
	snap := pfSnap(pfBothRecorded()...)
	deployed := pfBothDeployed(ownedRecord("sha256:edited"), ownedRecord("sha256:r2"))

	it := pfHookItems(pfBuild(t, snap, deployed, "2.0"))[pfFlatPath]

	if it.Action != domain.ActionUpdate || !hasExactReason(it, editReason) || len(it.Reasons) < 2 || !it.BackupBeforeWrite {
		t.Errorf("Action = %v, Reasons = %q, backup = %v; want update with the edit reason, a version reason and backup",
			it.Action, it.Reasons, it.BackupBeforeWrite)
	}
}

func TestBuildHook_UneditedFileWithStaleVersion_UpdateWithoutBackup(t *testing.T) {
	snap := pfSnap(pfBothRecorded()...)
	deployed := pfBothDeployed(ownedRecord("sha256:r1"), ownedRecord("sha256:r2"))

	items := pfHookItems(pfBuild(t, snap, deployed, "2.0"))

	for path, it := range items {
		if it.Action != domain.ActionUpdate || it.BackupBeforeWrite || hasExactReason(it, editReason) {
			t.Errorf("%s: Action = %v, backup = %v, Reasons = %q; want a plain version update", path, it.Action, it.BackupBeforeWrite, it.Reasons)
		}
	}
}

func TestBuildHook_EntryRecordedWithoutContentRecord_NoEditCheck(t *testing.T) {
	snap := pfSnap(pfEntry(pfFlatPath, nil), pfEntry(pfNestedPath, nil))
	deployed := pfBothDeployed(ownedRecord("sha256:anything"), ownedRecord("sha256:anything"))

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	for path, it := range items {
		if it.Action != domain.ActionUnchanged || it.BackupBeforeWrite {
			t.Errorf("%s: Action = %v, backup = %v; want unchanged without an edit check", path, it.Action, it.BackupBeforeWrite)
		}
	}
}

func TestBuildHook_UnrecordedFile_ConflictNotEditUpdate(t *testing.T) {
	// A manifest exists and records the nested file only; the flat file is on disk unrecorded.
	snap := pfSnap(pfEntry(pfNestedPath, ownedRecord("sha256:r2")))
	deployed := pfBothDeployed(ownedRecord("sha256:whatever"), ownedRecord("sha256:r2"))

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	flat := items[pfFlatPath]
	if flat.Action != domain.ActionConflict || flat.Conflict == nil || !flat.Conflict.ManifestMissing {
		t.Fatalf("unrecorded file: Action = %v, Conflict = %+v; want conflict with ManifestMissing", flat.Action, flat.Conflict)
	}
	if hasExactReason(flat, editReason) {
		t.Error("unrecorded file given the edit reason")
	}
	if nested := items[pfNestedPath]; nested.Action != domain.ActionUnchanged {
		t.Errorf("recorded file: Action = %v; want unchanged", nested.Action)
	}
}

func TestBuildHook_FilePresentNoUsableManifest_Conflict(t *testing.T) {
	deployed := map[string]domain.DeployedArtifactState{pfFlatPath: pfDeployed(ownedRecord("sha256:x"))}

	items := pfHookItems(pfBuild(t, absentSnapshot(), deployed, pfHookVersion))

	flat := items[pfFlatPath]
	if flat.Action != domain.ActionConflict || flat.Conflict == nil || !flat.Conflict.ManifestMissing {
		t.Errorf("Action = %v, Conflict = %+v; want conflict with ManifestMissing", flat.Action, flat.Conflict)
	}
	if nested := items[pfNestedPath]; nested.Action != domain.ActionCreate {
		t.Errorf("absent file: Action = %v; want create", nested.Action)
	}
}

func TestBuildHook_BundleLevelEntry_FilesOverwrittenWithoutConflictOrBackup(t *testing.T) {
	snap := pfSnap(pfBundleEntry())
	deployed := pfBothDeployed(ownedRecord("sha256:any1"), ownedRecord("sha256:any2"))

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	if len(items) != 2 {
		t.Fatalf("hook items = %d; want one per file", len(items))
	}
	for path, it := range items {
		if it.Action != domain.ActionUpdate {
			t.Errorf("%s: Action = %v; want update", path, it.Action)
		}
		if it.Conflict != nil || it.BackupBeforeWrite {
			t.Errorf("%s: Conflict = %+v, backup = %v; want neither for a pre-change record", path, it.Conflict, it.BackupBeforeWrite)
		}
		if !hasExactReason(it, pfLegacyReason) {
			t.Errorf("%s: Reasons = %q; want %q", path, it.Reasons, pfLegacyReason)
		}
		if hasExactReason(it, editReason) {
			t.Errorf("%s: pre-change record must not be edit-checked", path)
		}
	}
}

func TestBuildHook_BundleLevelEntry_AbsentFileIsCreate(t *testing.T) {
	snap := pfSnap(pfBundleEntry())
	deployed := map[string]domain.DeployedArtifactState{pfFlatPath: pfDeployed(ownedRecord("sha256:any"))}

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	if it := items[pfNestedPath]; it.Action != domain.ActionCreate {
		t.Errorf("absent file: Action = %v; want create", it.Action)
	}
	if it := items[pfFlatPath]; it.Action != domain.ActionUpdate || it.Conflict != nil {
		t.Errorf("present file: Action = %v, Conflict = %+v; want update without conflict", it.Action, it.Conflict)
	}
}

func TestBuildHook_DamagedBundle_MissingFileRewritten(t *testing.T) {
	snap := pfSnap(pfBothRecorded()...)
	deployed := map[string]domain.DeployedArtifactState{pfFlatPath: pfDeployed(ownedRecord("sha256:r1"))}

	items := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))

	missing := items[pfNestedPath]
	if missing.Action != domain.ActionCreate || !hasExactReason(missing, pfDamagedReason) {
		t.Errorf("missing file: Action = %v, Reasons = %q; want create with %q", missing.Action, missing.Reasons, pfDamagedReason)
	}
	if present := items[pfFlatPath]; present.Action != domain.ActionUnchanged {
		t.Errorf("intact file: Action = %v; want unchanged", present.Action)
	}
}

func TestBuildHook_NoEntryAnywhereForAbsentFile_NewFileReasonNotDamaged(t *testing.T) {
	snap := pfSnap(pfEntry(pfFlatPath, ownedRecord("sha256:r1")))
	deployed := map[string]domain.DeployedArtifactState{pfFlatPath: pfDeployed(ownedRecord("sha256:r1"))}

	it := pfHookItems(pfBuild(t, snap, deployed, pfHookVersion))[pfNestedPath]

	if it.Action != domain.ActionCreate || !hasExactReason(it, pfNewFileReason) || hasExactReason(it, pfDamagedReason) {
		t.Errorf("Action = %v, Reasons = %q; want create with %q only", it.Action, it.Reasons, pfNewFileReason)
	}
}

func TestEnumerateTargetPaths_Hook_OnePathPerHookFileInFilesOrder(t *testing.T) {
	set := plan.ArtifactSet{Hooks: []domain.HookBundle{makeHookBundle(pfHookKey, pfHookVersion)}}

	paths, err := plan.EnumerateTargetPaths(set, pfModule(), domain.ScopeProject, "linux")

	must(t, err)
	got := paths.PathsFor(hookRef(pfHookKey))
	want := []string{pfFlatPath, pfNestedPath}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("PathsFor = %v; want %v", got, want)
	}
	if first, ok := paths.Path(hookRef(pfHookKey)); !ok || first != pfFlatPath {
		t.Errorf("Path = %q, %v; want the first file %q", first, ok, pfFlatPath)
	}
}

func TestEnumerateTargetPaths_Hook_UnsupportedHookPlanOmitted(t *testing.T) {
	set := plan.ArtifactSet{Hooks: []domain.HookBundle{makeHookBundle(pfHookKey, pfHookVersion)}}
	mod := pfModule()
	mod.HookPlanFn = func(domain.HookPlanRequest) (domain.HookPlan, error) {
		return domain.HookPlan{Supported: false}, nil
	}

	paths, err := plan.EnumerateTargetPaths(set, mod, domain.ScopeProject, "linux")

	must(t, err)
	if len(paths) != 0 {
		t.Errorf("PlannedPaths = %v; want none for an unsupported bundle", paths)
	}
}
