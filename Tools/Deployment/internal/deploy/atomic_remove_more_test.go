package deploy_test

// atomic_remove_more_test.go extends the atomic removal and replacement coverage: the TODO
// file stays as it was on a reversed run, a reversed replacement leaves the manifest alone,
// locally modified files are restored byte for byte, files created by the run are deleted
// again, and a restored file keeps its mode.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

func todoFiles(ws string) []string {
	matches, _ := filepath.Glob(filepath.Join(ws, "MOSAIC-DEPLOYMENT-TODO*"))
	return matches
}

// gapEmittingHook returns a hook plan whose registration file exists unrequested in ws, so a
// completed run would emit a registration gap and write a TODO file.
func gapEmittingHook(t *testing.T, ws string) domain.HookPlan {
	t.Helper()
	writeWorkspaceFile(t, ws, regTarget, `{"user":"edited"}`)
	return hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))
}

func TestAtomicRemoval_SaveFailure_WritesNoTodoFileDespiteGaps(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	hp := gapEmittingHook(t, ws)

	res := execAtomicWith(t, failingSaveStore(), removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), []domain.HookPlan{hp})

	if !res.Reverted {
		t.Fatal("Reverted is false; want true")
	}
	if got := todoFiles(ws); len(got) != 0 {
		t.Errorf("TODO file written by a reverted run: %v", got)
	}
	assertFileHolds(t, ws, "agents/old.md", "OLD")
	// The backup proves the removal really ran before the reversal.
	assertBackupHolds(t, ws, "agents/old.md", "OLD")
}

func TestAtomicRemoval_SaveFailure_LeavesPreExistingTodoFileUntouched(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	hp := gapEmittingHook(t, ws)
	writeWorkspaceFile(t, ws, "MOSAIC-DEPLOYMENT-TODO.md", "PREVIOUS TODO")

	res := execAtomicWith(t, failingSaveStore(), removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), []domain.HookPlan{hp})

	if !res.Reverted {
		t.Fatal("Reverted is false; want true")
	}
	got := todoFiles(ws)
	if len(got) != 1 || filepath.Base(got[0]) != "MOSAIC-DEPLOYMENT-TODO.md" {
		t.Fatalf("TODO files = %v; want only the pre-existing one", got)
	}
	assertFileHolds(t, ws, "MOSAIC-DEPLOYMENT-TODO.md", "PREVIOUS TODO")
	assertBackupHolds(t, ws, "agents/old.md", "OLD")
}

func TestAtomicReplacement_LaterWriteFailure_RestoresModifiedFileAndManifest(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/x.md": "OLD AGENT"})
	writeWorkspaceFile(t, ws, "agents/x.md", "USER EDITED")
	manifestPath := filepath.Join(ws, ".mosaic", "manifest.yaml")
	before := readFile(t, manifestPath)
	blockPath(t, filepath.Join(ws, "skills"))
	x := newAgentItem("x", "agents/x.md", domain.ActionUpdate)
	failing := newAgentItem("new", "skills/new.md", domain.ActionCreate)
	plan := newPlan(ws, x, failing)
	plan.Replacements = []domain.Replacement{{Ref: x.Ref, Path: "agents/x.md"}}

	res := execAtomicWith(t, manifest.NewStore(), plan, nil)

	if !res.Reverted {
		t.Fatal("Reverted is false; want true after a failed write")
	}
	assertFileHolds(t, ws, "agents/x.md", "USER EDITED")
	assertBackupHolds(t, ws, "agents/x.md", "USER EDITED")
	if got := readFile(t, manifestPath); string(got) != string(before) {
		t.Error("manifest file changed by a reverted replacement")
	}
}

func TestAtomicMovingRequest_FileCreatedByTheRunIsDeletedOnReversal(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	blockPath(t, filepath.Join(ws, "skills"))
	created := newAgentItem("fresh", "agents/fresh.md", domain.ActionCreate)
	failing := newAgentItem("new", "skills/new.md", domain.ActionCreate)
	plan := removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}, created, failing)

	res := execAtomicWith(t, manifest.NewStore(), plan, nil)

	if !res.Reverted {
		t.Fatal("Reverted is false; want true")
	}
	if fileExistsAt(t, filepath.Join(ws, "agents", "fresh.md")) {
		t.Error("file created by the reverted run still exists")
	}
	assertFileHolds(t, ws, "agents/old.md", "OLD")
	assertBackupHolds(t, ws, "agents/old.md", "OLD")
}

func TestAtomicRemoval_Reversal_RestoresFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on Windows")
	}
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	abs := filepath.Join(ws, "agents", "old.md")
	if err := os.Chmod(abs, 0o600); err != nil {
		t.Fatal(err)
	}

	res := execAtomicWith(t, failingSaveStore(), removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), nil)

	if !res.Reverted {
		t.Fatal("Reverted is false; want true")
	}
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatalf("restored file missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("restored mode = %v; want 0600", info.Mode().Perm())
	}
}
