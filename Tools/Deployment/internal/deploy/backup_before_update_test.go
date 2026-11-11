package deploy_test

// backup_before_update_test.go specifies the executor's handling of updates caused by a local
// edit of Deploy-owned content (PlanItem.BackupBeforeWrite): the current file is backed up
// under .mosaic/backups before it is overwritten, and the action record names the backup.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	editedAgentPath = "agents/runner.md"
	editedOnDisk    = "# runner as edited by hand\n"
	freshContent    = "# runner as Deploy writes it\n"
)

// editUpdateItem is an agent update caused by a local edit of Deploy-owned content.
func editUpdateItem() domain.PlanItem {
	item := newAgentItem("runner", editedAgentPath, domain.ActionUpdate)
	item.BackupBeforeWrite = true
	item.AddReason("Deploy-owned content was edited locally")
	return item
}

// executeEditUpdate runs one request over a workspace holding the edited file.
func executeEditUpdate(t *testing.T, item domain.PlanItem, dry bool) (string, deploy.ExecResult, error) {
	t.Helper()
	workspace := t.TempDir()
	writeExisting(t, workspace, item.TargetPath, []byte(editedOnDisk))
	req := deploy.ExecRequest{
		Plan:       newPlan(workspace, item),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte(freshContent)),
		DryRun:     dry,
	}
	ex := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector())
	res, err := ex.Execute(context.Background(), req)
	return workspace, res, err
}

func TestExecute_EditCausedUpdate_BacksUpEditedFileBeforeOverwrite(t *testing.T) {
	workspace, res, err := executeEditUpdate(t, editUpdateItem(), false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, filepath.Join(workspace, editedAgentPath)); !bytes.Equal(got, []byte(freshContent)) {
		t.Errorf("target = %q; want the fresh content", got)
	}
	backups := backupFilesUnder(t, workspace)
	if len(backups) != 1 {
		t.Fatalf("backup files = %v; want exactly one under .mosaic/backups", backups)
	}
	if got := readFile(t, backups[0]); !bytes.Equal(got, []byte(editedOnDisk)) {
		t.Errorf("backup content = %q; want the edited file as it was", got)
	}
	if len(res.Actions) != 1 || res.Actions[0].Taken != domain.TakenBackedUp {
		t.Fatalf("Actions = %+v; want one record with TakenBackedUp", res.Actions)
	}
	if res.Actions[0].BackupPath != backups[0] {
		t.Errorf("BackupPath = %q; want %q", res.Actions[0].BackupPath, backups[0])
	}
}

func TestExecute_UpdateWithoutBackupMarker_WritesWithoutBackup(t *testing.T) {
	item := editUpdateItem()
	item.BackupBeforeWrite = false

	workspace, res, err := executeEditUpdate(t, item, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if backups := backupFilesUnder(t, workspace); len(backups) != 0 {
		t.Errorf("backups = %v; an ordinary update makes none", backups)
	}
	if res.Actions[0].Taken != domain.TakenUpdated {
		t.Errorf("Taken = %q; want %q", res.Actions[0].Taken, domain.TakenUpdated)
	}
}

func TestExecute_EditCausedUpdate_BackupFailureLeavesFileUntouched(t *testing.T) {
	workspace := t.TempDir()
	item := editUpdateItem()
	writeExisting(t, workspace, item.TargetPath, []byte(editedOnDisk))
	// A regular file where the backup directory must be makes every backup fail.
	blockPath(t, filepath.Join(workspace, manifest.Dir, manifest.BackupDir))
	req := deploy.ExecRequest{
		Plan:       newPlan(workspace, item),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte(freshContent)),
	}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, filepath.Join(workspace, item.TargetPath)); !bytes.Equal(got, []byte(editedOnDisk)) {
		t.Errorf("target = %q; the file must stay as it was when the backup fails", got)
	}
	rec := res.Actions[0]
	if rec.Taken != domain.TakenFailed || !strings.HasPrefix(rec.Err, "backup failed: ") {
		t.Errorf("record Taken = %q, Err = %q; want TakenFailed with \"backup failed: ...\"", rec.Taken, rec.Err)
	}
}

func TestExecute_EditCausedUpdate_DryRunReportsBackupWithoutWriting(t *testing.T) {
	workspace, res, err := executeEditUpdate(t, editUpdateItem(), true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, filepath.Join(workspace, editedAgentPath)); !bytes.Equal(got, []byte(editedOnDisk)) {
		t.Errorf("dry run changed the target: %q", got)
	}
	if _, statErr := os.Stat(filepath.Join(workspace, manifest.Dir, manifest.BackupDir)); statErr == nil {
		t.Error("dry run created the backup directory")
	}
	if rec := res.Actions[0]; rec.Taken != domain.TakenBackedUp || rec.BackupPath != "" {
		t.Errorf("record Taken = %q, BackupPath = %q; want TakenBackedUp with empty BackupPath", rec.Taken, rec.BackupPath)
	}
}

func TestExecute_EditCausedSkillUpdate_BacksUpEditedFileBeforeOverwrite(t *testing.T) {
	item := newSkillItem("lean-tdd", "skills/lean-tdd.md", domain.ActionUpdate)
	item.BackupBeforeWrite = true

	workspace, res, err := executeEditUpdate(t, item, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	backups := backupFilesUnder(t, workspace)
	if len(backups) != 1 || !bytes.Equal(readFile(t, backups[0]), []byte(editedOnDisk)) {
		t.Fatalf("backups = %v; want exactly one holding the edited skill file as it was", backups)
	}
	if got := readFile(t, filepath.Join(workspace, item.TargetPath)); !bytes.Equal(got, []byte(freshContent)) {
		t.Errorf("target = %q; want the fresh content", got)
	}
	if res.Actions[0].Taken != domain.TakenBackedUp {
		t.Errorf("Taken = %q; want %q", res.Actions[0].Taken, domain.TakenBackedUp)
	}
}

func TestExecute_EditCausedUpdate_AtomicRunBacksUpAndWrites(t *testing.T) {
	workspace := t.TempDir()
	item := editUpdateItem()
	writeExisting(t, workspace, item.TargetPath, []byte(editedOnDisk))
	req := deploy.ExecRequest{
		Plan:       newPlan(workspace, item),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte(freshContent)),
		Atomic:     true,
	}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	backups := backupFilesUnder(t, workspace)
	if len(backups) != 1 || !bytes.Equal(readFile(t, backups[0]), []byte(editedOnDisk)) {
		t.Fatalf("backups = %v; want exactly one holding the edited file as it was", backups)
	}
	if got := readFile(t, filepath.Join(workspace, item.TargetPath)); !bytes.Equal(got, []byte(freshContent)) {
		t.Errorf("target = %q; want the fresh content", got)
	}
	if res.Reverted || res.Actions[0].Taken != domain.TakenBackedUp {
		t.Errorf("Reverted = %v, Taken = %q; want a kept run with TakenBackedUp", res.Reverted, res.Actions[0].Taken)
	}
}

func TestExecute_EditCausedUpdate_RevertedAtomicRunRestoresEditedFile(t *testing.T) {
	workspace := t.TempDir()
	item := editUpdateItem()
	writeExisting(t, workspace, item.TargetPath, []byte(editedOnDisk))
	bad := newAgentItem("other", "blocked/other.md", domain.ActionCreate)
	blockPath(t, filepath.Join(workspace, "blocked"))
	req := deploy.ExecRequest{
		Plan:       newPlan(workspace, item, bad),
		MosaicRoot: t.TempDir(),
		Content:    fixedContent([]byte(freshContent)),
		Atomic:     true,
	}

	res, err := deploy.NewExecutor(manifest.NewStore(), newSpyLogger(), newSpyCollector()).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !res.Reverted {
		t.Fatal("precondition: the run was expected to revert")
	}
	if got := readFile(t, filepath.Join(workspace, item.TargetPath)); !bytes.Equal(got, []byte(editedOnDisk)) {
		t.Errorf("target = %q; a reverted run must restore the edited file", got)
	}
}

// backupFilesUnder lists the regular files below <workspace>/.mosaic/backups.
func backupFilesUnder(t *testing.T, workspace string) []string {
	t.Helper()
	var out []string
	root := filepath.Join(workspace, manifest.Dir, manifest.BackupDir)
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}
