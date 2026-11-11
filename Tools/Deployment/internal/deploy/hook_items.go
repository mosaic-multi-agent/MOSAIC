package deploy

// hook_items.go holds the executor behaviour specific to hook plan items. A hook bundle is
// planned as one item per deployed file; the file's bytes come from the item's own source file
// and the file is recorded in the manifest like any other written file.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/logging"
)

// itemContent returns the bytes written for a plan item: the bytes of the item's own source
// file for a hook file, the content callback's result for every other kind.
func itemContent(item domain.PlanItem, req ExecRequest) ([]byte, error) {
	if item.Ref.Kind == domain.ArtifactHook {
		content, err := os.ReadFile(item.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("hook source file %q could not be read: %w", item.SourcePath, err)
		}
		return content, nil
	}
	return req.Content(item)
}

// writesFile reports whether the item, with the user's decisions, writes its target file.
func writesFile(item domain.PlanItem, conflicts map[string]domain.ConflictDecision) bool {
	switch item.Action {
	case domain.ActionCreate, domain.ActionUpdate:
		return true
	case domain.ActionConflict:
		d := conflicts[item.TargetPath]
		return d == domain.DecisionOverwrite || d == domain.DecisionBackupThenOverwrite
	}
	return false
}

// skipUnreadableHookSource handles a hook item whose source file cannot be read when the
// request has no removals or replacements: the file is not written, the problem is logged, and a
// manual-step gap tells the user. It reports ok=false when the item is no such case (not a hook
// item, no write, readable source, or a strict request, where the write itself fails the run).
func (e *executor) skipUnreadableHookSource(item domain.PlanItem, req ExecRequest, deployRoot string, strict bool) (ar domain.ActionRecord, gap domain.Gap, ok bool) {
	if strict || item.Ref.Kind != domain.ArtifactHook || !writesFile(item, req.Conflicts) {
		return ar, gap, false
	}
	_, err := os.ReadFile(item.SourcePath)
	if err == nil {
		return ar, gap, false
	}
	e.log.Event(logging.Event{
		Time:    time.Now(),
		Level:   logging.LevelError,
		Kind:    "hook",
		Subject: item.SourcePath,
		Message: fmt.Sprintf("hook source file unreadable, skipping: %s", err),
	})
	ar = domain.ActionRecord{
		Ref:        item.Ref,
		TargetPath: filepath.Join(deployRoot, item.TargetPath),
		Stale:      item.Stale,
		Taken:      domain.TakenFailed,
		Err:        fmt.Sprintf("hook source file %q could not be read and was not deployed: %s", item.SourcePath, err),
	}
	gap = domain.Gap{
		Kind:    domain.GapManualStep,
		Subject: path.Base(filepath.ToSlash(item.TargetPath)),
		Detail:  ar.Err,
	}
	return ar, gap, true
}

// journalHookBackup records the backup made before a hook file was overwritten, so an atomic
// reversal removes it together with the write it belonged to.
func journalHookBackup(item domain.PlanItem, ar domain.ActionRecord, journal *writeJournal) {
	if journal != nil && item.Ref.Kind == domain.ArtifactHook && ar.BackupPath != "" {
		journal.recordNew(ar.BackupPath)
	}
}

// backupConflictFile backs up the file of a conflict item the user chose to back up before
// overwriting. A hook file that is already gone has nothing to back up and yields no path.
func backupConflictFile(item domain.PlanItem, workspace, srcPath string) (string, error) {
	if item.Ref.Kind == domain.ArtifactHook {
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			return "", nil
		}
	}
	return createBackup(workspace, item.TargetPath, srcPath)
}

// dropBundleLevelEntries removes the bundle-level records of the hook ref once the run records
// per-file entries for it.
func dropBundleLevelEntries(entryMap map[entryKey]domain.ManifestEntry, harness string, ref domain.ArtifactRef) {
	for k, e := range entryMap {
		if k.Harness == harness && k.Ref == ref && e.IsBundleLevelHook() {
			delete(entryMap, k)
		}
	}
}

// writtenHookFiles returns the normalised target paths of the hook files the run wrote.
func writtenHookFiles(actions []domain.ActionRecord, deployRoot string) map[string]bool {
	written := map[string]bool{}
	for _, a := range actions {
		if a.Ref.Kind != domain.ArtifactHook {
			continue
		}
		switch a.Taken {
		case domain.TakenCreated, domain.TakenUpdated, domain.TakenBackedUp:
			if rel, err := filepath.Rel(deployRoot, a.TargetPath); err == nil {
				written[normRel(rel)] = true
			}
		}
	}
	return written
}

// bundleWritten reports whether the run wrote at least one file of the hook plan.
func bundleWritten(hp domain.HookPlan, written map[string]bool) bool {
	dir := strings.ReplaceAll(hp.TargetDir, "\\", "/")
	for _, hf := range hp.Files {
		if written[normRel(path.Join(dir, hf.TargetName))] {
			return true
		}
	}
	return false
}
