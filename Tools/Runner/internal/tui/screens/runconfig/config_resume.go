package runconfig

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// stepAfterTimeout returns the step that follows the harness timeout: the
// version-drift question when it applies, then whatever follows it.
func (s *ConfigScreen) stepAfterTimeout() configStep {
	if s.showsVersionDrift() {
		return configStepVersionDrift
	}
	return s.stepAfterVersionDrift()
}

// stepAfterVersionDrift returns the step that follows the version-drift
// question. A new run goes on to its checkpoints question. A resume never asks
// a recorded setting: it asks only the commit branch of a pending commit-setup
// retry and, for a run that records no mode, the runner settings.
func (s *ConfigScreen) stepAfterVersionDrift() configStep {
	if s.isNewRun {
		return configStepCheckpoints
	}
	if s.pendingCommitRetry() {
		return configStepCommitBranch
	}
	return s.nextAfterCommitSection()
}

// stepBeforeResumeSettings returns the step a resume shows before its
// commit-branch and runner-settings questions: the version-drift question when
// it applies, else the harness timeout.
func (s *ConfigScreen) stepBeforeResumeSettings() configStep {
	if s.showsVersionDrift() {
		return configStepVersionDrift
	}
	return configStepHarnessTimeout
}

// entryCursor returns the cursor a step starts with when the wizard moves
// forward into it: the explicit-choice commit-branch retry has no option
// preselected, and pre-consultation is suggested as enabled.
func (s *ConfigScreen) entryCursor(step configStep) int {
	switch step {
	case configStepCommitBranch:
		if s.pendingCommitRetry() {
			return -1
		}
	case configStepPreConsult:
		return 1
	}
	return 0
}

// Reopen puts a finished resume wizard back on the last question it asked, with
// every answer kept, so the user can change it after seeing the settings
// summary. Back-navigation from there follows the usual step order.
func (s *ConfigScreen) Reopen() tea.Cmd {
	s.back = false
	s.step = s.lastResumeStep()
	s.cursor = s.reopenCursor()
	if s.step == configStepHarnessTimeout {
		return s.timeoutInput.Init()
	}
	return nil
}

// lastResumeStep returns the last question a resume asks.
func (s *ConfigScreen) lastResumeStep() configStep {
	switch {
	case s.needsAdoption:
		return configStepManualResolution
	case s.pendingCommitRetry():
		return configStepCommitBranch
	}
	return s.stepBeforeResumeSettings()
}

// reopenCursor returns the cursor that shows the answer given to the step the
// wizard was reopened on.
func (s *ConfigScreen) reopenCursor() int {
	switch s.step {
	case configStepManualResolution:
		if s.sel.Settings.ManualResolution {
			return 1
		}
	case configStepCommitBranch:
		for i, v := range domain.CommitBranchVariants() {
			if v == s.sel.Settings.CommitBranchVariant {
				return i
			}
		}
		return -1
	case configStepVersionDrift:
		if !s.sel.AllowVersionDrift {
			return 1
		}
	}
	return 0
}
