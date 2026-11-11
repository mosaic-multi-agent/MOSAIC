package deploy

import (
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/domain"
)

// deployHooks applies the registration steps of the hook bundles that had a file written.
// Hook files themselves are written from plan items (one per file); the hook plans supply only
// their registration steps.
// Registration steps follow the write-if-absent / TODO-if-present policy (AC17.5):
//   - !Performable: always emit GapManualStep; never write anything.
//   - Performable, target absent: write the fragment; emit no gap.
//   - Performable, target exists: never modify; emit GapHookRegistration.
//
// Registration targets are relative to the workspace, not the deployment root,
// because they are user-owned config files (e.g. .claude/settings.json).
//
// written holds the hook file paths the run wrote; a bundle none of whose files was written
// gets no registration. journal is non-nil in atomic mode and covers registration writes.
//
// opts carries the opt-in behaviour of a request with removals or replacements: strict makes a
// failed journal record or a failed write an error instead of a gap; replace lists registration
// targets (workspace-relative) that are overwritten even when they exist; blocked lists paths
// whose backup failed and which must not be written.
func (e *executor) deployHooks(workspace string, hooks []domain.HookPlan, written map[string]bool, dryRun bool, journal *writeJournal, opts hookOpts) ([]domain.Gap, error) {
	var gaps []domain.Gap
	if dryRun {
		return gaps, nil
	}
	for _, hp := range hooks {
		if !hp.Supported || !bundleWritten(hp, written) {
			continue
		}
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
