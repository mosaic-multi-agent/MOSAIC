package deploy_test

// atomic_remove_test.go covers atomic mode for removals and replacements: a failed run
// restores every removed and replaced file with its original bytes, leaves the manifest and
// the TODO file as they were, and keeps the backups.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// blockOnSaveStore deletes nothing itself: on Save it turns blockPath into a directory
// (so restoring a file there must fail) and then returns an error.
type blockOnSaveStore struct {
	inner     manifest.Store
	blockPath string
}

func (s *blockOnSaveStore) Load(ws string) (manifest.Snapshot, error) { return s.inner.Load(ws) }

func (s *blockOnSaveStore) Save(string, domain.Manifest) error {
	_ = os.MkdirAll(s.blockPath, 0o755)
	return errors.New("simulated manifest save failure")
}

func failingSaveStore() manifest.Store {
	return &failSaveStore{inner: manifest.NewStore(), saveErr: errors.New("simulated manifest save failure")}
}

func execAtomicWith(t *testing.T, store manifest.Store, plan domain.Plan, hooks []domain.HookPlan) deploy.ExecResult {
	t.Helper()
	req := deploy.ExecRequest{
		Plan:       plan,
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte("# new")),
		Hooks:      hooks,
		Atomic:     true,
	}
	res, err := deploy.NewExecutor(store, newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return res
}

func assertNoTodoFile(t *testing.T, ws string) {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(ws, "MOSAIC-DEPLOYMENT-TODO*"))
	if len(matches) != 0 {
		t.Errorf("TODO file written by a reverted run: %v", matches)
	}
}

func TestAtomicRemoval_SaveFailure_RestoresRemovedFileAndManifest(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	writeWorkspaceFile(t, ws, "agents/old.md", "USER EDITED")
	manifestPath := filepath.Join(ws, ".mosaic", "manifest.yaml")
	before := readFile(t, manifestPath)

	res := execAtomicWith(t, failingSaveStore(), removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), nil)

	if !res.Reverted || res.Partial == nil {
		t.Fatalf("Reverted=%v Partial=%v; want a reverted run carrying the error", res.Reverted, res.Partial)
	}
	assertFileHolds(t, ws, "agents/old.md", "USER EDITED")
	if got := readFile(t, manifestPath); string(got) != string(before) {
		t.Error("manifest file changed by a reverted run")
	}
	assertBackupHolds(t, ws, "agents/old.md", "USER EDITED")
	assertNoTodoFile(t, ws)
}

func TestAtomicRemoval_LaterWriteFailure_RestoresRemovedFile(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	blockPath(t, filepath.Join(ws, "skills"))
	// A successful write comes first so the failing write is a true mid-plan failure (atomic
	// reversal) rather than a total failure that takes the mosaic-root fallback tier.
	first := newAgentItem("fresh", "agents/fresh.md", domain.ActionCreate)
	item := newAgentItem("new", "skills/new.md", domain.ActionCreate)

	res := execAtomicWith(t, manifest.NewStore(), removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}, first, item), nil)

	if !res.Reverted {
		t.Fatal("Reverted is false; want true after a failed write")
	}
	assertFileHolds(t, ws, "agents/old.md", "OLD")
	if !hasPath(manifestTargetPaths(t, ws), "agents/old.md") {
		t.Error("manifest lost the removed entry although the run was reverted")
	}
	assertBackupHolds(t, ws, "agents/old.md", "OLD")
	assertNoTodoFile(t, ws)
}

func TestAtomicReplacement_SaveFailure_RestoresRegistrationFile(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	res := execAtomicWith(t, failingSaveStore(), planReplacing(ws, replacementOf(regTarget)), []domain.HookPlan{hp})

	if !res.Reverted {
		t.Fatal("Reverted is false; want true")
	}
	assertFileHolds(t, ws, regTarget, `{"user":"edited"}`)
	if fileExistsAt(t, filepath.Join(ws, ".github", "hooks", "h.sh")) {
		t.Error("hook file created by the reverted run still exists")
	}
	assertBackupHolds(t, ws, regTarget, `{"user":"edited"}`)
}

func TestAtomicReplacement_SaveFailure_RestoresHookFileAndPlanItem(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/x.md": "OLD AGENT"})
	writeWorkspaceFile(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "NEW HOOK")
	item := newAgentItem("x", "agents/x.md", domain.ActionUpdate)
	plan := planWithItems(ws, item, replacementOf(".github/hooks/h.sh"), domain.Replacement{Ref: item.Ref, Path: "agents/x.md"})

	res := execAtomicWith(t, failingSaveStore(), plan, []domain.HookPlan{hp})

	if !res.Reverted {
		t.Fatal("Reverted is false; want true")
	}
	assertFileHolds(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	assertFileHolds(t, ws, "agents/x.md", "OLD AGENT")
	assertBackupHolds(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	assertBackupHolds(t, ws, "agents/x.md", "OLD AGENT")
}

func TestAtomicRemoval_RestoreFailure_ReportedInRevertFailures(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	removed := filepath.Join(ws, "agents", "old.md")
	store := &blockOnSaveStore{inner: manifest.NewStore(), blockPath: removed}

	res := execAtomicWith(t, store, removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), nil)

	if !res.Reverted {
		t.Fatal("Reverted is false; want true")
	}
	found := false
	for _, f := range res.RevertFailures {
		if f.Path == removed && f.Restore {
			found = true
		}
	}
	if !found {
		t.Errorf("RevertFailures = %+v; want an entry for %q with Restore=true", res.RevertFailures, removed)
	}
	assertBackupHolds(t, ws, "agents/old.md", "OLD")
}

func TestAtomicRemoval_BackupFailure_ReversesEarlierRemovals(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	// A non-empty directory cannot be backed up as a file, so its removal fails after the
	// earlier removal was already applied.
	writeWorkspaceFile(t, ws, "agents/dir/inner.md", "INNER")
	rms := []domain.Removal{agentRemoval("agents/old.md"), {Ref: agentRefFor("agents/dir"), Path: "agents/dir"}}

	res := execAtomicWith(t, manifest.NewStore(), removalPlan(ws, rms), nil)

	if !res.Reverted || res.Partial == nil {
		t.Fatalf("Reverted=%v Partial=%v; want a reverted run carrying the error", res.Reverted, res.Partial)
	}
	assertFileHolds(t, ws, "agents/old.md", "OLD")
	assertFileHolds(t, ws, "agents/dir/inner.md", "INNER")
	if !hasPath(manifestTargetPaths(t, ws), "agents/old.md") {
		t.Error("manifest lost the entry although the run was reverted")
	}
}

func TestAtomicMovingRequest_UnreadableHookSource_RestoresRemovedFile(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	hp := domain.HookPlan{
		Supported: true,
		TargetDir: ".github/hooks",
		Files:     []domain.HookFile{{SourcePath: filepath.Join(t.TempDir(), "missing.sh"), TargetName: "h.sh"}},
	}

	res := execAtomicWith(t, manifest.NewStore(), removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), []domain.HookPlan{hp})

	if !res.Reverted {
		t.Fatal("Reverted is false; an unreadable hook source in a moving request must reverse the run")
	}
	assertFileHolds(t, ws, "agents/old.md", "OLD")
}

func TestAtomicReplacement_Success_ReplacesFileKeepsBackupAndNotReverted(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))

	res := execAtomicWith(t, manifest.NewStore(), planReplacing(ws, replacementOf(regTarget)), []domain.HookPlan{hp})

	if res.Reverted || res.Partial != nil {
		t.Fatalf("Reverted=%v Partial=%v; want a clean run", res.Reverted, res.Partial)
	}
	assertFileHolds(t, ws, regTarget, `{"fragment":1}`)
	assertBackupHolds(t, ws, regTarget, `{"user":"edited"}`)
}

func TestAtomicRemoval_Success_RemovesFileKeepsBackupAndNotReverted(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})

	res := execAtomicWith(t, manifest.NewStore(), removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), nil)

	if res.Reverted || res.Partial != nil {
		t.Fatalf("Reverted=%v Partial=%v; want a clean run", res.Reverted, res.Partial)
	}
	if fileExistsAt(t, filepath.Join(ws, "agents", "old.md")) {
		t.Error("file still exists after a successful atomic removal")
	}
	assertBackupHolds(t, ws, "agents/old.md", "OLD")
	if hasPath(manifestTargetPaths(t, ws), "agents/old.md") {
		t.Error("entry still in the manifest after a successful atomic removal")
	}
}
