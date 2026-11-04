package deploy

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

// deployHooks writes hook bundle files and applies registration steps.
// Hook files are copied to <deployRoot>/<HookPlan.TargetDir>/<HookFile.TargetName>.
// Registration steps follow the write-if-absent / TODO-if-present policy (AC17.5):
//   - !Performable: always emit GapManualStep; never write anything.
//   - Performable, target absent: write the fragment; emit no gap.
//   - Performable, target exists: never modify; emit GapHookRegistration.
//
// Registration targets are relative to the workspace, not the deployment root,
// because they are user-owned config files (e.g. .claude/settings.json).
//
// journal is non-nil in atomic mode. When non-nil, each hook file write is recorded in the
// journal immediately before the write so the executor can reverse hook files if the run
// later fails (e.g. a manifest save failure). If the journal record itself fails (e.g. an
// existing hook file at the destination is unreadable), the write is skipped and a gap is
// emitted — hook problems are gaps, not reversal triggers.
//
// opts carries the opt-in behaviour of a request with removals or replacements: strict makes
// an unreadable source, a failed journal record or a failed write an error instead of a gap;
// replace lists registration targets (workspace-relative) that are overwritten even when they
// exist; blocked lists paths whose backup failed and which must not be written. With the zero
// opts the behaviour is the policy described above.
func (e *executor) deployHooks(deployRoot, workspace string, hooks []domain.HookPlan, dryRun bool, journal *writeJournal, opts hookOpts) ([]domain.Gap, error) {
	var gaps []domain.Gap

	for _, hp := range hooks {
		if !hp.Supported {
			continue
		}

		if !dryRun {
			// Copy hook files to the target directory under the deployment root.
			targetDir := filepath.Join(deployRoot, hp.TargetDir)
			for _, hf := range hp.Files {
				if opts.blocked[normRel(path.Join(strings.ReplaceAll(hp.TargetDir, "\\", "/"), hf.TargetName))] {
					continue
				}
				content, err := os.ReadFile(hf.SourcePath)
				if err != nil {
					if opts.strict {
						return gaps, fmt.Errorf("hook source file %q could not be read: %w", hf.SourcePath, err)
					}
					// Source file unreadable: this indicates a catalog integrity problem.
					// Log the error and emit a manual-step gap so the user is informed and
					// the hook file does not silently disappear from the deployment (AC17.6).
					e.log.Event(logging.Event{
						Time:    time.Now(),
						Level:   logging.LevelError,
						Kind:    "hook",
						Subject: hf.SourcePath,
						Message: fmt.Sprintf("hook source file unreadable, skipping: %s", err),
					})
					gaps = append(gaps, domain.Gap{
						Kind:    domain.GapManualStep,
						Subject: hf.TargetName,
						Detail:  fmt.Sprintf("hook source file %q could not be read and was not deployed: %s", hf.SourcePath, err),
					})
					continue
				}
				dest := filepath.Join(targetDir, hf.TargetName)
				// Journal the destination before writing so an atomic reversal can undo this write.
				// If the journal record fails (e.g. an existing hook file at dest is unreadable),
				// skip the write and emit a gap rather than proceeding with an un-journaled write.
				if journal != nil {
					if recErr := journal.record(dest); recErr != nil {
						if opts.strict {
							return gaps, fmt.Errorf("hook file %q could not be journaled for atomic reversal: %w", dest, recErr)
						}
						e.log.Event(logging.Event{
							Time:    time.Now(),
							Level:   logging.LevelError,
							Kind:    "hook",
							Subject: dest,
							Message: fmt.Sprintf("atomic journal record failed for hook file, skipping write: %s", recErr),
						})
						gaps = append(gaps, domain.Gap{
							Kind:    domain.GapManualStep,
							Subject: hf.TargetName,
							Detail:  fmt.Sprintf("hook file %q could not be journaled for atomic reversal and was not deployed: %s", dest, recErr),
						})
						continue
					}
				}
				if werr := mkdirAndWrite(dest, content); werr != nil && opts.strict {
					return gaps, fmt.Errorf("hook file %q could not be written: %w", dest, werr)
				}
			}

			// Apply registration steps.
			for _, step := range hp.Registration {
				g, err := applyRegistrationStep(step, workspace, journal, opts)
				if err != nil {
					return gaps, err
				}
				if g != nil {
					gaps = append(gaps, *g)
				}
			}
		}
	}

	return gaps, nil
}

// hookOpts are the opt-in options of deployHooks; the zero value keeps the default policy.
type hookOpts struct {
	strict  bool
	replace map[string]bool
	blocked map[string]bool
}

// applyRegistrationStep enforces the hook registration policy for one step.
// It returns a Gap when the step produces a TODO item, or nil when the fragment
// was written successfully (no user action required).
//
// journal is non-nil in atomic mode. When non-nil and the target is absent (so a write will
// occur), the target path is recorded in the journal before the write. Recording a
// non-existent path always succeeds (it captures existed=false), so this is a no-error path.
//
// When opts.replace lists the step's target, an existing file is overwritten by the fragment
// (the caller has already backed it up) and no gap is emitted; a journal or write failure is
// then returned as an error. A target in opts.blocked is left alone.
func applyRegistrationStep(step domain.RegistrationStep, workspace string, journal *writeJournal, opts hookOpts) (*domain.Gap, error) {
	if !step.Performable {
		// Tool cannot perform this step at all (e.g. a user-level editor setting).
		// Always emit a manual-step gap.
		return &domain.Gap{
			Kind:     domain.GapManualStep,
			Subject:  step.ID,
			Detail:   step.Instruction,
			Fragment: step.Fragment,
		}, nil
	}

	// Performable: check whether the target file already exists.
	targetPath := filepath.Join(workspace, step.TargetPath)
	rel := normRel(step.TargetPath)
	if opts.blocked[rel] {
		return nil, nil
	}
	replace := opts.replace[rel]
	if _, err := os.Stat(targetPath); err == nil && !replace {
		// Target exists: never modify; emit a hook-registration gap so the user
		// can paste the fragment manually (AC17.5).
		return &domain.Gap{
			Kind:     domain.GapHookRegistration,
			Subject:  step.ID,
			Detail:   step.Instruction,
			Fragment: step.Fragment,
		}, nil
	}

	// Target absent (or replacement requested): journal then write the fragment (AC17.5).
	// Recording a non-existent path always succeeds (it captures existed=false so reversal
	// will delete the file); recording an existing file can fail if it is unreadable.
	if journal != nil {
		if err := journal.record(targetPath); err != nil && replace {
			return nil, fmt.Errorf("registration file %q could not be journaled for atomic reversal: %w", targetPath, err)
		}
	}
	if err := mkdirAndWrite(targetPath, []byte(step.Fragment)); err != nil && replace {
		return nil, fmt.Errorf("registration file %q could not be written: %w", targetPath, err)
	}
	return nil, nil
}
