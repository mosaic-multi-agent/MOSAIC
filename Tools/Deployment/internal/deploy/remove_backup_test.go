package deploy_test

// remove_backup_test.go covers the backup taken before each removal: the original bytes are
// kept under .mosaic/backups whether or not the user modified the file.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestRemovalBackup_UnmodifiedFile_BackedUpWithOriginalBytes(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "ORIGINAL"})

	res := execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), false, false)

	path := assertBackupHolds(t, ws, "agents/old.md", "ORIGINAL")
	if len(res.Removals) != 1 || res.Removals[0].BackupPath != path {
		t.Errorf("Removals = %+v; want BackupPath %q", res.Removals, path)
	}
}

func TestRemovalBackup_LocallyModifiedFile_BackedUpWithModifiedBytes(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/old.md": "ORIGINAL"})
	writeWorkspaceFile(t, ws, "agents/old.md", "USER EDITED")

	execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/old.md")}), false, false)

	assertBackupHolds(t, ws, "agents/old.md", "USER EDITED")
}

func TestRemovalBackup_UnrecordedFile_BackedUpBeforeDeletion(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, ".github/hooks/helper.sh", "#!/bin/sh")
	rm := domain.Removal{
		Ref:  domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "logger"},
		Path: ".github/hooks/helper.sh",
	}

	execWithPlan(t, removalPlan(ws, []domain.Removal{rm}), false, false)

	assertBackupHolds(t, ws, ".github/hooks/helper.sh", "#!/bin/sh")
	if fileExistsAt(t, ws+"/.github/hooks/helper.sh") {
		t.Error("unrecorded file was not deleted")
	}
}

func TestRemovalBackup_EveryRemovedFileGetsItsOwnBackup(t *testing.T) {
	ws := t.TempDir()
	seedAgents(t, ws, map[string]string{"agents/a.md": "AAA", "agents/b.md": "BBB"})

	execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/a.md"), agentRemoval("agents/b.md")}), false, false)

	assertBackupHolds(t, ws, "agents/a.md", "AAA")
	assertBackupHolds(t, ws, "agents/b.md", "BBB")
}

func TestRemovalBackup_AbsentFileCreatesNoBackup(t *testing.T) {
	ws := t.TempDir()

	execWithPlan(t, removalPlan(ws, []domain.Removal{agentRemoval("agents/none.md")}), false, false)

	if got := backupFilesFor(t, ws, "agents/none.md"); len(got) != 0 {
		t.Errorf("backups for an absent file: %v; want none", got)
	}
}
