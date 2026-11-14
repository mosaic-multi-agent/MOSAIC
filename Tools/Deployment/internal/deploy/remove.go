package deploy

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/domain"
)

// ErrInvalidRemoval is returned by Execute, before any write, when a Removal or Replacement
// has an empty Path, an absolute Path, or a Path that leaves the workspace ("..").
var ErrInvalidRemoval = errors.New("invalid removal or replacement path")

// normRel normalises a workspace-relative path to a cleaned, slash-separated form so paths
// from plans, hooks and manifest entries compare equal.
func normRel(p string) string {
	return path.Clean(strings.ReplaceAll(p, "\\", "/"))
}

// validRel reports whether p is a non-empty relative path that stays inside the workspace.
func validRel(p string) bool {
	if p == "" {
		return false
	}
	slashed := strings.ReplaceAll(p, "\\", "/")
	if filepath.IsAbs(p) || strings.HasPrefix(slashed, "/") || filepath.VolumeName(p) != "" {
		return false
	}
	n := normRel(p)
	return n != "." && n != ".." && !strings.HasPrefix(n, "../")
}

// validateMoves checks every Removal and Replacement path of the plan.
func validateMoves(plan domain.Plan) error {
	for _, r := range plan.Removals {
		if !validRel(r.Path) {
			return fmt.Errorf("%w: removal %q", ErrInvalidRemoval, r.Path)
		}
	}
	for _, r := range plan.Replacements {
		if !validRel(r.Path) {
			return fmt.Errorf("%w: replacement %q", ErrInvalidRemoval, r.Path)
		}
	}
	return nil
}

// moveRun carries the removal and replacement state of one Execute call.
type moveRun struct {
	plan         domain.Plan
	replace      map[string]bool // normalised paths of requested replacements
	written      map[string]bool // normalised paths this run writes
	blocked      map[string]bool // replacement paths whose backup failed; never written
	removals     []domain.RemovalRecord
	replacements []domain.ReplacementRecord
	err          error // first failure
}

// newMoveRun prepares the replacement and written-path sets for the request.
func newMoveRun(req ExecRequest) *moveRun {
	m := &moveRun{plan: req.Plan, replace: map[string]bool{}, blocked: map[string]bool{}}
	for _, r := range req.Plan.Replacements {
		m.replace[normRel(r.Path)] = true
	}
	m.written = m.writtenPaths(req)
	return m
}

// active reports whether the request asks for any removal or replacement.
func (m *moveRun) active() bool {
	return len(m.plan.Removals)+len(m.plan.Replacements) > 0
}

func (m *moveRun) fail(err error) {
	if m.err == nil {
		m.err = err
	}
}

// writtenPaths lists the workspace-relative paths the run writes: plan item files (hook files
// included) and registration targets that are absent or requested for replacement.
func (m *moveRun) writtenPaths(req ExecRequest) map[string]bool {
	set := map[string]bool{}
	for _, it := range req.Plan.Items {
		switch it.Action {
		case domain.ActionCreate, domain.ActionUpdate:
			set[normRel(it.TargetPath)] = true
		case domain.ActionConflict:
			if d := req.Conflicts[it.TargetPath]; d == domain.DecisionOverwrite || d == domain.DecisionBackupThenOverwrite {
				set[normRel(it.TargetPath)] = true
			}
		}
	}
	for _, hp := range req.Hooks {
		if !hp.Supported {
			continue
		}
		for _, step := range hp.Registration {
			if !step.Performable {
				continue
			}
			rel := normRel(step.TargetPath)
			if _, err := os.Stat(filepath.Join(req.Plan.WorkspacePath, filepath.FromSlash(rel))); err != nil || m.replace[rel] {
				set[rel] = true
			}
		}
	}
	return set
}

func (m *moveRun) absPath(rel string) string {
	return filepath.Join(m.plan.WorkspacePath, filepath.FromSlash(rel))
}

// backupReplacements backs up every existing file that a requested replacement overwrites,
// before the run writes anything. A dry run only reports. A failed backup blocks the write.
func (m *moveRun) backupReplacements(dryRun bool) {
	for _, r := range m.plan.Replacements {
		rel := normRel(r.Path)
		rec := domain.ReplacementRecord{Replacement: r}
		if !dryRun && m.written[rel] {
			if _, err := os.Lstat(m.absPath(rel)); err == nil {
				bp, berr := createBackup(m.plan.WorkspacePath, filepath.FromSlash(rel), m.absPath(rel))
				if berr != nil {
					rec.Err = fmt.Sprintf("backup of %s failed: %s", rel, berr)
					m.blocked[rel] = true
					m.fail(errors.New(rec.Err))
				} else {
					rec.BackupPath = bp
				}
			}
		}
		m.replacements = append(m.replacements, rec)
	}
}

// removeFiles deletes the requested files after backing them up and drops the manifest
// entries of those removals that ask for it. Files the run writes are kept. In fallback
// mode the workspace is untouched. journal is non-nil in atomic mode, where the first
// failure stops further removals.
func (m *moveRun) removeFiles(req ExecRequest, fallback domain.FallbackTier, journal *writeJournal, entryMap map[entryKey]domain.ManifestEntry, harness string) {
	if journal != nil && m.err != nil {
		return
	}
	for _, rm := range m.plan.Removals {
		rel := normRel(rm.Path)
		rec := domain.RemovalRecord{Removal: rm}
		switch {
		case fallback != domain.FallbackNone:
			rec.Taken = domain.RemovalSkipped
		case m.written[rel]:
			rec.Taken = domain.RemovalKept
		default:
			m.removeOne(req.DryRun, rel, journal, &rec)
		}
		if rm.DropEntry && (rec.Taken == domain.RemovalRemoved || rec.Taken == domain.RemovalAbsent) {
			delete(entryMap, keyOf(harness, rm.Ref, rel))
		}
		m.removals = append(m.removals, rec)
		if rec.Taken == domain.RemovalFailed {
			m.fail(errors.New(rec.Err))
			if journal != nil {
				return
			}
		}
	}
}

// removeOne backs up and deletes one file, filling rec with the outcome.
func (m *moveRun) removeOne(dryRun bool, rel string, journal *writeJournal, rec *domain.RemovalRecord) {
	abs := m.absPath(rel)
	if _, err := os.Lstat(abs); err != nil {
		if os.IsNotExist(err) {
			rec.Taken = domain.RemovalAbsent
			return
		}
		rec.Taken, rec.Err = domain.RemovalFailed, fmt.Sprintf("remove %s: %s", rel, err)
		return
	}
	if dryRun {
		rec.Taken = domain.RemovalRemoved
		return
	}
	if journal != nil {
		if err := journal.record(abs); err != nil {
			rec.Taken, rec.Err = domain.RemovalFailed, fmt.Sprintf("remove %s: atomic journal record failed: %s", rel, err)
			return
		}
	}
	bp, err := createBackup(m.plan.WorkspacePath, filepath.FromSlash(rel), abs)
	if err != nil {
		rec.Taken, rec.Err = domain.RemovalFailed, fmt.Sprintf("remove %s: backup failed: %s", rel, err)
		return
	}
	rec.BackupPath = bp
	if err := os.Remove(abs); err != nil {
		rec.Taken, rec.Err = domain.RemovalFailed, fmt.Sprintf("remove %s: %s", rel, err)
		return
	}
	rec.Taken = domain.RemovalRemoved
}

// unblockedItems drops plan items whose target had a failed replacement backup, adding a
// failed action record for each, so a file that could not be backed up is never overwritten.
func (m *moveRun) unblockedItems(items []domain.PlanItem, actions *[]domain.ActionRecord, deployRoot string) []domain.PlanItem {
	if len(m.blocked) == 0 {
		return items
	}
	kept := make([]domain.PlanItem, 0, len(items))
	for _, it := range items {
		if m.blocked[normRel(it.TargetPath)] {
			*actions = append(*actions, domain.ActionRecord{
				Ref:        it.Ref,
				TargetPath: filepath.Join(deployRoot, it.TargetPath),
				Taken:      domain.TakenFailed,
				Err:        "existing file could not be backed up; not overwritten",
			})
			continue
		}
		kept = append(kept, it)
	}
	return kept
}

// hookOpts derives the hook deployment options for the request.
func (m *moveRun) hookOpts() hookOpts {
	return hookOpts{strict: m.active(), replace: m.replace, blocked: m.blocked}
}

// revertedResult reverses the journal and builds the result of an atomic run that failed.
func (e *executor) revertedResult(
	journal *writeJournal, deployRoot string, fallback domain.FallbackTier,
	actions []domain.ActionRecord, prior domain.Manifest, partialErr error, m *moveRun,
) ExecResult {
	revertFailures := journal.revert()
	e.log.EndRun(computeOutcome(actions, partialErr, fallback))
	return ExecResult{
		DeploymentRoot: deployRoot,
		Fallback:       fallback,
		Actions:        actions,
		Manifest:       prior,
		Partial:        partialErr,
		Reverted:       true,
		RevertFailures: revertFailures,
		Removals:       m.removals,
		Replacements:   m.replacements,
	}
}
