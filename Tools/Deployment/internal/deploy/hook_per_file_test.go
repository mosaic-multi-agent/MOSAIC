package deploy_test

// hook_per_file_test.go specifies how the executor records and writes hook bundles: hook files
// are written from the plan items (one per file), each with its own manifest entry holding the
// real path and the hash of the bytes written; registration files get no entry; an edited or
// unrecorded hook file is backed up before it is overwritten.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

func TestExecute_HookCreate_WritesEachFileFromItsOwnSource(t *testing.T) {
	f := newHookBundleFixture(t)

	f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate), f.nestedItem(domain.ActionCreate)}, nil)

	assertFileHolds(t, f.workspace, hookFlatRel, hookFlatSource)
	assertFileHolds(t, f.workspace, hookNestedRel, hookNestedSource)
}

func TestExecute_HookCreate_OneEntryPerFileWithRealPathAndHash(t *testing.T) {
	f := newHookBundleFixture(t)

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate), f.nestedItem(domain.ActionCreate)}, nil)

	entries := hookEntriesByPath(res.Manifest)
	if len(entries) != 2 {
		t.Fatalf("hook entries = %v; want one per written file", entries)
	}
	for path, content := range map[string]string{hookFlatRel: hookFlatSource, hookNestedRel: hookNestedSource} {
		e, ok := entries[path]
		if !ok {
			t.Fatalf("no entry for %q (entries: %v)", path, entries)
		}
		if e.Ref != perFileHookRef {
			t.Errorf("%s: Ref = %v; want the bundle ref", path, e.Ref)
		}
		if want := manifest.Hash([]byte(content)); e.ContentHash != want {
			t.Errorf("%s: ContentHash = %q; want the hash of the written bytes %q", path, e.ContentHash, want)
		}
		if e.ContentHash == domain.EmptyContentHash {
			t.Errorf("%s: entry carries the empty-content hash", path)
		}
		owned, err := transform.OwnedContentOf(domain.ArtifactHook, []byte(content))
		if err != nil {
			t.Fatalf("OwnedContentOf: %v", err)
		}
		if e.Owned == nil || e.Owned.Hash != owned.Hash {
			t.Errorf("%s: Owned = %+v; want the whole-file record %q", path, e.Owned, owned.Hash)
		}
	}
}

func TestExecute_HookCreate_EntryCarriesStampVersionAndEveryEntryPathExists(t *testing.T) {
	f := newHookBundleFixture(t)

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate), f.nestedItem(domain.ActionCreate)}, func(r *deploy.ExecRequest) {
		r.VersionStamps = map[string]domain.VersionStamp{
			hookFlatRel:   {Version: "3.1"},
			hookNestedRel: {Version: "3.1"},
		}
	})

	for path, e := range hookEntriesByPath(res.Manifest) {
		if e.Version != "3.1" {
			t.Errorf("%s: Version = %q; want the bundle version", path, e.Version)
		}
		if !fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(e.TargetPath))) {
			t.Errorf("entry path %q does not exist on disk", e.TargetPath)
		}
	}
}

func TestExecute_HookCreate_OneActionRecordPerFile(t *testing.T) {
	f := newHookBundleFixture(t)

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate), f.nestedItem(domain.ActionCreate)}, nil)

	var created int
	for _, a := range res.Actions {
		if a.Ref == perFileHookRef && a.Taken == domain.TakenCreated {
			created++
		}
	}
	if created != 2 {
		t.Errorf("created hook records = %d (%+v); want one per file", created, res.Actions)
	}
}

func TestExecute_HookRegistration_FileWrittenButNoEntry(t *testing.T) {
	f := newHookBundleFixture(t)

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate), f.nestedItem(domain.ActionCreate)}, func(r *deploy.ExecRequest) {
		r.Hooks = f.registration()
	})

	assertFileHolds(t, f.workspace, hookRegRel, "{\"registered\":true}")
	entries := hookEntriesByPath(res.Manifest)
	if _, ok := entries[hookRegRel]; ok {
		t.Error("the registration file has a manifest entry; Deploy only merges into it")
	}
	if len(entries) != 2 {
		t.Errorf("hook entries = %v; want exactly the two hook files", entries)
	}
}

func TestExecute_HookFilesOfExecRequestHooks_AreNotWritten(t *testing.T) {
	f := newHookBundleFixture(t)

	// The plan names only the flat file; the nested file is listed in the hook plan alone.
	f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate)}, func(r *deploy.ExecRequest) {
		r.Hooks = f.registration()
	})

	if fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(hookNestedRel))) {
		t.Error("a hook file with no plan item was written; plan items are the single source of hook file writes")
	}
}

func TestExecute_HookRegistration_NotPerformedWhenNoFileIsWritten(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookFlatSource)
	writeWorkspaceFile(t, f.workspace, hookNestedRel, hookNestedSource)

	f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionUnchanged), f.nestedItem(domain.ActionUnchanged)}, func(r *deploy.ExecRequest) {
		r.Hooks = f.registration()
	})

	if fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(hookRegRel))) {
		t.Error("registration was performed although no file of the bundle was written")
	}
}

func TestExecute_HookUnchanged_WritesNothingAndKeepsEntry(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	prior := domain.ManifestEntry{Ref: perFileHookRef, TargetPath: hookFlatRel, Version: "1.0", ContentHash: "sha256:prior"}
	seedHookEntries(t, f.workspace, prior)

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionUnchanged)}, nil)

	assertFileHolds(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	if e, ok := hookEntriesByPath(res.Manifest)[hookFlatRel]; !ok || e.ContentHash != "sha256:prior" {
		t.Errorf("entry = %+v, %v; want the prior entry unchanged", e, ok)
	}
}

func TestExecute_HookEditedUpdate_BacksUpThenOverwritesAndRerecords(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	item := f.flatItem(domain.ActionUpdate)
	item.BackupBeforeWrite = true

	res := f.executeHooks(t, []domain.PlanItem{item}, nil)

	assertFileHolds(t, f.workspace, hookFlatRel, hookFlatSource)
	backup := assertBackupHolds(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	if len(res.Actions) != 1 || res.Actions[0].Taken != domain.TakenBackedUp || res.Actions[0].BackupPath != backup {
		t.Errorf("Actions = %+v; want one TakenBackedUp record naming %q", res.Actions, backup)
	}
	if e := hookEntriesByPath(res.Manifest)[hookFlatRel]; e.ContentHash != manifest.Hash([]byte(hookFlatSource)) {
		t.Errorf("ContentHash = %q; want the hash of the bytes now on disk", e.ContentHash)
	}
}

func TestExecute_HookUpdateWithoutBackupMarker_WritesWithoutBackup(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, "# older deploy\n")

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionUpdate)}, nil)

	assertFileHolds(t, f.workspace, hookFlatRel, hookFlatSource)
	if backups := backupFilesFor(t, f.workspace, hookFlatRel); len(backups) != 0 {
		t.Errorf("backups = %v; an ordinary update makes none", backups)
	}
	if res.Actions[0].Taken != domain.TakenUpdated {
		t.Errorf("Taken = %q; want %q", res.Actions[0].Taken, domain.TakenUpdated)
	}
}

func unrecordedHookConflict(f hookBundleFixture) domain.PlanItem {
	item := f.flatItem(domain.ActionConflict)
	item.Conflict = &domain.LocalModification{CurrentHash: "sha256:on-disk", ManifestMissing: true}
	return item
}

func TestExecute_HookConflictBackupThenOverwrite_BacksUpTheFile(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)

	res := f.executeHooks(t, []domain.PlanItem{unrecordedHookConflict(f)}, func(r *deploy.ExecRequest) {
		r.Conflicts = map[string]domain.ConflictDecision{hookFlatRel: domain.DecisionBackupThenOverwrite}
	})

	assertFileHolds(t, f.workspace, hookFlatRel, hookFlatSource)
	assertBackupHolds(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	if res.Actions[0].Taken != domain.TakenBackedUp || res.Actions[0].BackupPath == "" {
		t.Errorf("record = %+v; want TakenBackedUp with a backup path", res.Actions[0])
	}
}

func TestExecute_HookConflictOverwrite_WritesWithoutBackup(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)

	res := f.executeHooks(t, []domain.PlanItem{unrecordedHookConflict(f)}, func(r *deploy.ExecRequest) {
		r.Conflicts = map[string]domain.ConflictDecision{hookFlatRel: domain.DecisionOverwrite}
	})

	assertFileHolds(t, f.workspace, hookFlatRel, hookFlatSource)
	if backups := backupFilesFor(t, f.workspace, hookFlatRel); len(backups) != 0 {
		t.Errorf("backups = %v; the overwrite decision makes none", backups)
	}
	if res.Actions[0].Taken != domain.TakenUpdated {
		t.Errorf("Taken = %q; want %q", res.Actions[0].Taken, domain.TakenUpdated)
	}
}

func TestExecute_HookConflictSkip_KeepsFileAndWritesNoEntry(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)

	res := f.executeHooks(t, []domain.PlanItem{unrecordedHookConflict(f)}, func(r *deploy.ExecRequest) {
		r.Conflicts = map[string]domain.ConflictDecision{hookFlatRel: domain.DecisionSkip}
	})

	assertFileHolds(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	if _, ok := hookEntriesByPath(res.Manifest)[hookFlatRel]; ok {
		t.Error("a skipped hook file received an entry for bytes Deploy did not write")
	}
}

func TestExecute_HookConflictOverwrite_RecordsTheWrittenBytes(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)

	res := f.executeHooks(t, []domain.PlanItem{unrecordedHookConflict(f)}, func(r *deploy.ExecRequest) {
		r.Conflicts = map[string]domain.ConflictDecision{hookFlatRel: domain.DecisionOverwrite}
	})

	if e := hookEntriesByPath(res.Manifest)[hookFlatRel]; e.ContentHash != manifest.Hash([]byte(hookFlatSource)) || e.Owned == nil {
		t.Errorf("entry = %+v; want the hash and record of the bytes written", e)
	}
}

func TestExecute_BundleLevelEntry_ReplacedByPerFileEntries(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, "# older deploy\n")
	writeWorkspaceFile(t, f.workspace, hookNestedRel, "# older deploy\n")
	seedHookEntries(t, f.workspace, bundleLevelHookEntry())

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionUpdate), f.nestedItem(domain.ActionUpdate)}, nil)

	entries := hookEntriesByPath(res.Manifest)
	if len(entries) != 2 {
		t.Fatalf("hook entries = %v; want the two per-file entries only", entries)
	}
	for path, e := range entries {
		if e.ContentHash == domain.EmptyContentHash {
			t.Errorf("%s: bundle-level entry survived: %+v", path, e)
		}
	}
	// The replacement is saved, not only reported.
	snap, err := manifest.NewStore().Load(f.workspace)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved := hookEntriesByPath(snap.Manifest); len(saved) != 2 {
		t.Errorf("saved hook entries = %v; want the two per-file entries only", saved)
	}
}

func TestExecute_BundleLevelEntryOfOtherBundle_Kept(t *testing.T) {
	f := newHookBundleFixture(t)
	other := bundleLevelHookEntry()
	other.Ref = domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "other-bundle"}
	other.TargetPath = hookDir + "/other-bundle"
	writeWorkspaceFile(t, f.workspace, other.TargetPath, "")
	seedHookEntries(t, f.workspace, other)

	res := f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate)}, nil)

	if _, ok := hookEntriesByPath(res.Manifest)[other.TargetPath]; !ok {
		t.Error("the bundle-level entry of a bundle this run did not write was dropped")
	}
}

func TestExecute_HookDryRun_WritesNothingAndReportsBackup(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	edited := f.flatItem(domain.ActionUpdate)
	edited.BackupBeforeWrite = true

	res := f.executeHooks(t, []domain.PlanItem{edited, f.nestedItem(domain.ActionCreate)}, func(r *deploy.ExecRequest) {
		r.DryRun = true
	})

	assertFileHolds(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	if fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(hookNestedRel))) {
		t.Error("dry run wrote a hook file")
	}
	if _, err := os.Stat(filepath.Join(f.workspace, manifest.Dir, manifest.BackupDir)); err == nil {
		t.Error("dry run created the backup directory")
	}
	if len(res.Actions) != 2 || res.Actions[0].Taken != domain.TakenBackedUp || res.Actions[0].BackupPath != "" {
		t.Errorf("Actions = %+v; want a TakenBackedUp record without a path, then a created record", res.Actions)
	}
	if len(hookEntriesByPath(res.Manifest)) != 2 {
		t.Errorf("dry-run manifest hook entries = %v; want per-file entries for the planned writes", hookEntriesByPath(res.Manifest))
	}
}

func TestExecute_HookSourceUnreadable_FileSkippedWithGap(t *testing.T) {
	f := newHookBundleFixture(t)
	item := f.flatItem(domain.ActionCreate)
	item.SourcePath = filepath.Join(t.TempDir(), "missing.json")
	spy := newSpyCollector()
	req := deploy.ExecRequest{
		Plan:       newPlan(f.workspace, item, f.nestedItem(domain.ActionCreate)),
		MosaicRoot: t.TempDir(),
		Content:    hookContent,
	}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), spy).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(hookFlatRel))) {
		t.Error("a file with an unreadable source was written")
	}
	assertFileHolds(t, f.workspace, hookNestedRel, hookNestedSource)
	entries := hookEntriesByPath(res.Manifest)
	if _, ok := entries[hookFlatRel]; ok {
		t.Error("an entry records a file that was not written")
	}
	if _, ok := entries[hookNestedRel]; !ok {
		t.Error("the readable file has no entry")
	}
	if !spy.hasGapKind(domain.GapManualStep) {
		t.Error("no gap reported for the unreadable hook source")
	}
}

func TestExecute_RemovalOfHookFile_DropsOnlyThatFilesEntry(t *testing.T) {
	f := newHookBundleFixture(t)
	f.executeHooks(t, []domain.PlanItem{f.flatItem(domain.ActionCreate), f.nestedItem(domain.ActionCreate)}, nil)
	if !fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(hookNestedRel))) {
		t.Fatal("precondition: the setup run did not write the nested hook file")
	}
	plan := newPlan(f.workspace)
	plan.Removals = []domain.Removal{{Ref: perFileHookRef, Path: hookNestedRel, DropEntry: true, Reason: "moved"}}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(),
		deploy.ExecRequest{Plan: plan, MosaicRoot: t.TempDir(), Content: hookContent})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	entries := hookEntriesByPath(res.Manifest)
	if _, ok := entries[hookNestedRel]; ok {
		t.Error("the removed file's entry survived")
	}
	if _, ok := entries[hookFlatRel]; !ok {
		t.Error("the other file's entry was dropped with it")
	}
	if fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(hookNestedRel))) {
		t.Error("the removed file is still on disk")
	}
}

// saveObserverStore runs onSave at the moment the executor attempts the manifest save, then
// fails the save like failSaveStore.
type saveObserverStore struct {
	*failSaveStore
	onSave func()
}

func (s *saveObserverStore) Save(workspaceRoot string, m domain.Manifest) error {
	s.onSave()
	return s.failSaveStore.Save(workspaceRoot, m)
}

func TestAtomicReversal_HookItemWritesAndBackupsReversedOnSaveFailure(t *testing.T) {
	f := newHookBundleFixture(t)
	writeWorkspaceFile(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	edited := f.flatItem(domain.ActionUpdate)
	edited.BackupBeforeWrite = true
	saveSeen := false
	store := &failSaveStore{inner: manifest.NewStore(), saveErr: errors.New("simulated manifest save failure")}
	observing := &saveObserverStore{failSaveStore: store, onSave: func() {
		saveSeen = true
		// The writes and the backup must have happened before the reversal.
		assertFileHolds(t, f.workspace, hookFlatRel, hookFlatSource)
		assertFileHolds(t, f.workspace, hookNestedRel, hookNestedSource)
		if backups := backupFilesFor(t, f.workspace, hookFlatRel); len(backups) != 1 {
			t.Errorf("backups at save time = %v; want exactly one for the edited file", backups)
		}
	}}
	req := deploy.ExecRequest{
		Plan:       newPlan(f.workspace, edited, f.nestedItem(domain.ActionCreate)),
		MosaicRoot: t.TempDir(),
		Content:    hookContent,
		Atomic:     true,
	}

	res, err := deploy.NewExecutor(observing, newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !saveSeen {
		t.Fatal("the manifest save was never attempted")
	}

	if !res.Reverted {
		t.Fatal("Reverted = false after a manifest save failure")
	}
	assertFileHolds(t, f.workspace, hookFlatRel, hookOnDiskEdit)
	if fileExistsAt(t, filepath.Join(f.workspace, filepath.FromSlash(hookNestedRel))) {
		t.Error("a hook file created by the failed run survived the reversal")
	}
	if backups := backupFilesFor(t, f.workspace, hookFlatRel); len(backups) != 0 {
		t.Errorf("backups = %v; the backup made for the reverted write must be removed", backups)
	}
}
