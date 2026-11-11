package deploy_test

// remove_policy_test.go covers the rules around removals and replacements: paths the run
// writes are never deleted, failures in non-atomic mode, the fallback tier, manifest entry
// matching, unrecorded files and a dry run that combines both lists.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
)

func hookRemoval(path string) domain.Removal {
	return domain.Removal{Ref: hookRef(), Path: path, Reason: "moved"}
}

func TestRemoval_HookFileWrittenByTheRun_IsKeptNotDeleted(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "NEW HOOK")
	plan := removalPlan(ws, []domain.Removal{hookRemoval(".github/hooks/h.sh")})

	res, _ := execReplacing(t, plan, []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, ".github/hooks/h.sh", "NEW HOOK")
	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalKept {
		t.Fatalf("Removals = %+v; want one record Taken=%q", res.Removals, domain.RemovalKept)
	}
	if res.Removals[0].BackupPath != "" {
		t.Errorf("BackupPath = %q for a kept path; want empty", res.Removals[0].BackupPath)
	}
}

func TestRemoval_RegistrationTargetWrittenByTheRun_IsKeptNotDeleted(t *testing.T) {
	ws := t.TempDir()
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "hook", registrationStep(regTarget, `{"fragment":1}`))
	plan := removalPlan(ws, []domain.Removal{hookRemoval(regTarget)})

	res, _ := execReplacing(t, plan, []domain.HookPlan{hp}, false, false)

	assertFileHolds(t, ws, regTarget, `{"fragment":1}`)
	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalKept {
		t.Fatalf("Removals = %+v; want one record Taken=%q", res.Removals, domain.RemovalKept)
	}
}

func TestRemoval_NonAtomicFailure_IsRecordedAndOtherRemovalsContinue(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/dir": "WAS A FILE", "agents/old.md": "OLD"})
	// Turn the recorded file into a non-empty directory: it cannot be backed up as a file.
	if err := os.Remove(filepath.Join(ws, "agents", "dir")); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, ws, "agents/dir/inner.md", "INNER")
	rms := []domain.Removal{agentRemoval("agents/dir"), agentRemoval("agents/old.md")}

	res := execWithPlan(t, removalPlan(ws, rms), false, false)

	if res.Partial == nil {
		t.Error("Partial is nil; a failed removal must set it")
	}
	if len(res.Removals) != 2 || res.Removals[0].Taken != domain.RemovalFailed || res.Removals[0].Err == "" {
		t.Fatalf("Removals = %+v; want the first record failed with Err set", res.Removals)
	}
	if res.Removals[1].Taken != domain.RemovalRemoved {
		t.Errorf("second Taken = %q; want %q (the run continues)", res.Removals[1].Taken, domain.RemovalRemoved)
	}
	assertFileHolds(t, ws, "agents/dir/inner.md", "INNER")
	paths := manifestTargetPaths(t, ws)
	// agents/dir is no longer a regular file, so reconciliation dropped its entry before the run.
	if hasPath(paths, "agents/old.md") {
		t.Errorf("manifest paths = %v; want the removed entry dropped", paths)
	}
}

func TestReplacement_NonAtomicBackupFailure_RecordsErrAndDoesNotOverwrite(t *testing.T) {
	ws := t.TempDir()
	// A non-empty directory at the hook destination cannot be backed up as a file.
	writeWorkspaceFile(t, ws, ".github/hooks/h.sh/inner.txt", "INNER")
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "NEW HOOK")

	res, _ := execReplacing(t, planReplacing(ws, replacementOf(".github/hooks/h.sh")), []domain.HookPlan{hp}, false, false)

	if res.Partial == nil {
		t.Error("Partial is nil; a failed replacement backup must set it")
	}
	if len(res.Replacements) != 1 || res.Replacements[0].Err == "" || res.Replacements[0].BackupPath != "" {
		t.Fatalf("Replacements = %+v; want one record with Err set and no BackupPath", res.Replacements)
	}
	assertFileHolds(t, ws, ".github/hooks/h.sh/inner.txt", "INNER")
}

func TestRemoval_FallbackTier_SkipsRemovalAndTouchesNoWorkspaceFile(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, "docs/old.md", "OLD")
	blockPath(t, filepath.Join(ws, "agents"))
	item := newAgentItem("x", "agents/x.md", domain.ActionCreate)

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{{Ref: agentRefFor("docs/old.md"), Path: "docs/old.md"}}, item), false, false)

	if res.Fallback == domain.FallbackNone {
		t.Fatal("Fallback = none; the blocked workspace should have forced a fallback tier")
	}
	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalSkipped {
		t.Fatalf("Removals = %+v; want one record Taken=%q", res.Removals, domain.RemovalSkipped)
	}
	assertFileHolds(t, ws, "docs/old.md", "OLD")
	if len(backupFilesFor(t, ws, "docs/old.md")) != 0 {
		t.Error("backup created although the removal was skipped")
	}
}

func TestRemoval_DropEntryMatchesRefAndPath_SameRefOnlyDropsWhenBothMatch(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/a.md": "A"})
	rm := domain.Removal{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "other"}, Path: "agents/a.md", DropEntry: true}

	execWithPlan(t, removalPlan(ws, []domain.Removal{rm}), false, false)

	if fileExistsAt(t, filepath.Join(ws, "agents", "a.md")) {
		t.Error("file not removed; the removal must delete the file whatever the manifest entry's Ref")
	}
	if !hasPath(manifestTargetPaths(t, ws), "agents/a.md") {
		t.Error("entry dropped although the removal's Ref differs from the recorded Ref")
	}
}

func TestRemoval_BackslashPath_MatchesEntryAfterNormalisation(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/a.md": "A"})
	rm := agentRemoval("agents/a.md")
	rm.Path = `agents\a.md`

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{rm}), false, false)

	if res.Partial != nil {
		t.Fatalf("Partial = %v; want nil", res.Partial)
	}
	if hasPath(manifestTargetPaths(t, ws), "agents/a.md") {
		t.Error("entry not dropped for a backslash Path")
	}
	if fileExistsAt(t, filepath.Join(ws, "agents", "a.md")) {
		t.Error("file not removed for a backslash Path")
	}
}

func TestRemoval_FileNotInManifestWithDropEntry_IsRemovedWithoutError(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, "notes/free.md", "FREE")
	rm := domain.Removal{Ref: agentRefFor("notes/free.md"), Path: "notes/free.md", DropEntry: true}

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{rm}), false, false)

	if res.Partial != nil {
		t.Fatalf("Partial = %v; an unrecorded file must not fail the run", res.Partial)
	}
	if fileExistsAt(t, filepath.Join(ws, "notes", "free.md")) {
		t.Error("unrecorded file was not removed")
	}
	assertBackupHolds(t, ws, "notes/free.md", "FREE")
}

func TestDryRun_RemovalAndHookReplacementCombined_ReportsBothTouchesNothing(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "OLD"})
	writeWorkspaceFile(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	hp := hookPlanFor(t, ".github/hooks", "h.sh", "NEW HOOK")
	plan := removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")})
	plan.Replacements = []domain.Replacement{replacementOf(".github/hooks/h.sh")}

	res, _ := execReplacing(t, plan, []domain.HookPlan{hp}, false, true)

	assertFileHolds(t, ws, "agents/old.md", "OLD")
	assertFileHolds(t, ws, ".github/hooks/h.sh", "OLD HOOK")
	if _, err := os.Stat(filepath.Join(ws, ".mosaic", "backups")); err == nil {
		t.Error("dry run created the backups directory")
	}
	if len(res.Removals) != 1 || res.Removals[0].Taken != domain.RemovalRemoved {
		t.Errorf("Removals = %+v; want one record Taken=%q", res.Removals, domain.RemovalRemoved)
	}
	if len(res.Replacements) != 1 || res.Replacements[0].BackupPath != "" {
		t.Errorf("Replacements = %+v; want one record with empty BackupPath", res.Replacements)
	}
}
