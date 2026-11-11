package deploy

// backup_before_update.go backs a file up before an update overwrites it, for updates caused
// by a local edit of Deploy-owned content (PlanItem.BackupBeforeWrite).

import (
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// needsBackupBeforeWrite reports whether the item is an update whose existing file must be
// backed up before it is overwritten.
func needsBackupBeforeWrite(item domain.PlanItem) bool {
	return item.Action == domain.ActionUpdate && item.BackupBeforeWrite
}

// backupBeforeProbe backs up the file of the item used as the workspace probe before the probe
// write replaces it, and remembers the backup for the item's own execution. It returns false
// when the backup failed and the probe must not be written. Items that need no backup, and
// files that do not exist, pass.
func backupBeforeProbe(req ExecRequest, item domain.PlanItem) bool {
	if !needsBackupBeforeWrite(item) {
		return true
	}
	srcPath := filepath.Join(req.Plan.WorkspacePath, item.TargetPath)
	if _, err := os.Stat(srcPath); err != nil {
		return true
	}
	backupPath, err := createBackup(req.Plan.WorkspacePath, item.TargetPath, srcPath)
	if err != nil {
		return false
	}
	req.probeBackups[item.TargetPath] = backupPath
	return true
}

// backupBeforeUpdate backs up the existing file of an update item that is marked
// BackupBeforeWrite, unless the probe already did. It records the backup path in ar and
// returns backedUp=true when a backup exists. It returns failed=true, with ar set to
// TakenFailed, when the backup could not be written; the file is then left untouched. Items
// that need no backup, and files that do not exist, are left alone.
func backupBeforeUpdate(item domain.PlanItem, ar *domain.ActionRecord, req ExecRequest, deployRoot string) (backedUp, failed bool) {
	if !needsBackupBeforeWrite(item) {
		return false, false
	}
	if done, ok := req.probeBackups[item.TargetPath]; ok {
		ar.BackupPath = done
		return true, false
	}
	srcPath := filepath.Join(deployRoot, item.TargetPath)
	if _, err := os.Stat(srcPath); err != nil {
		return false, false
	}
	backupPath, err := createBackup(req.Plan.WorkspacePath, item.TargetPath, srcPath)
	if err != nil {
		ar.Taken = domain.TakenFailed
		ar.Err = fmt.Sprintf("backup failed: %s", err)
		return false, true
	}
	ar.BackupPath = backupPath
	return true, false
}

// ownedContentOfWritten returns the Deploy-owned content record of the bytes written for an
// agent, skill or hook file, so the next run can compare against what Deploy wrote. It is nil
// for other kinds, for empty content and for an agent whose body cannot be parsed.
func ownedContentOfWritten(kind domain.ArtifactKind, content []byte) *domain.OwnedContent {
	if len(content) == 0 || (kind != domain.ArtifactAgent && kind != domain.ArtifactSkill && kind != domain.ArtifactHook) {
		return nil
	}
	owned, err := transform.OwnedContentOf(kind, content)
	if err != nil {
		return nil
	}
	return &owned
}
