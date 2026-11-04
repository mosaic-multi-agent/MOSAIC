package runconfig

// resume_wizard_test.go verifies which questions the configuration wizard asks
// on a resume. A resumed run already records its checkpoints, commits, commit
// branch, mode, pre-consultation, manual resolution, review loop limit and
// infrastructure selections, so the wizard asks only what is genuinely
// per-session: the harness timeout, the version-drift question when the
// versions differ, the commit branch for the pending commit-setup retry, and
// (only for a run that records no mode) mode, pre-consultation and manual
// resolution.
//
// The tests drive the wizard by what it shows, so they do not depend on the
// position of a step relative to another beyond what is stated.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// Prompt names used by the helpers below.
const (
	promptTimeout      = "timeout"
	promptDrift        = "drift"
	promptMode         = "mode"
	promptCommitBranch = "commit-branch"
	promptCheckpoints  = "checkpoints"
	promptCommits      = "commits"
	promptPreConsult   = "pre-consultation"
	promptManual       = "manual-resolution"
	promptReviewLimit  = "review-limit"
	promptInfraClass   = "infra-class"
)

// promptName names the prompt the screen is showing.
func promptName(s *ConfigScreen) string {
	view := screenText(s)
	for _, p := range []struct{ marker, name string }{
		{"invocation timeout", promptTimeout},
		{"allow workflow version drift", promptDrift},
		{"execution mode", promptMode},
		{"commit branch variant", promptCommitBranch},
		{"checkpoints:", promptCheckpoints},
		{"commits:", promptCommits},
		{"pre-consultation", promptPreConsult},
		{"manual resolution", promptManual},
		{"review loop limit", promptReviewLimit},
		{"agent for this run", promptInfraClass},
	} {
		if strings.Contains(view, p.marker) {
			return p.name
		}
	}
	return "unknown"
}

// advanceResume answers the current prompt with its default. The mode prompt
// has no default, so auto is chosen; the commit-branch retry prompt has none
// either, so the first variant is chosen.
func advanceResume(s *ConfigScreen) {
	if promptName(s) == promptCommitBranch {
		pressKey(s, tea.KeyDown)
		pressKey(s, tea.KeyEnter)
		return
	}
	advanceOnce(s)
}

// resumePrompts answers every prompt and returns the names of the prompts shown,
// in order.
func resumePrompts(t *testing.T, s *ConfigScreen) []string {
	t.Helper()
	var names []string
	for i := 0; i < 30; i++ {
		if s.Done() {
			return names
		}
		names = append(names, promptName(s))
		advanceResume(s)
	}
	t.Fatalf("the wizard did not finish within 30 prompts; saw %v", names)
	return nil
}

// recordedRunAgents declares every kind of infrastructure the wizard could ask
// about: two checkpoint agents (a selection prompt on a new run) and two commit
// agents.
func recordedRunAgents() []domain.DeclaredInfraAgent {
	return []domain.DeclaredInfraAgent{
		{Name: "checkpoint-manager-git", Class: "checkpoint"},
		{Name: "checkpoint-manager-alt", Class: "checkpoint"},
		{Name: "commit-manager-git", Class: "commit"},
		{Name: "commit-manager-alt", Class: "commit"},
	}
}

// newRecordedResume is a resume of a run that records every setting.
func newRecordedResume(recordedVersion, currentVersion string) *ConfigScreen {
	s := newDrivenScreen(false, recordedRunAgents())
	s.SetNeedsRunnerAdoption(false)
	s.SetVersionDriftInfo(recordedVersion, currentVersion)
	return s
}

func assertPrompts(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("prompts shown = %v, want %v", got, want)
	}
}

// ===== A resume of a fully recorded run =====

func TestConfigScreen_Resume_RecordedRun_AsksOnlyTheTimeout(t *testing.T) {
	s := newRecordedResume("1.0", "1.0")

	got := resumePrompts(t, s)

	assertPrompts(t, got, promptTimeout)
}

func TestConfigScreen_Resume_RecordedRun_NeverAsksARecordedSetting(t *testing.T) {
	s := newRecordedResume("1.0", "1.0")

	got := resumePrompts(t, s)

	for _, banned := range []string{
		promptCheckpoints, promptCommits, promptCommitBranch, promptMode, promptPreConsult,
		promptManual, promptReviewLimit, promptInfraClass,
	} {
		for _, name := range got {
			if name == banned {
				t.Errorf("a resume of a run that records %q asked it again; prompts shown = %v", banned, got)
			}
		}
	}
}

// Nothing was answered about a recorded setting, so nothing is supplied: the
// session compares only supplied values with the artifact, and a supplied
// commit-branch variant that differs from the record is a refusal.
func TestConfigScreen_Resume_RecordedRun_SuppliesNoRecordedSetting(t *testing.T) {
	s := newRecordedResume("1.0", "1.0")
	resumePrompts(t, s)

	sel := s.Selection()

	if sel.Supplied != (domain.SuppliedSettings{}) {
		t.Errorf("Supplied = %+v, want nothing supplied when no recorded setting was asked", sel.Supplied)
	}
	if sel.Settings.CommitBranchVariant != "" {
		t.Errorf("CommitBranchVariant = %q, want empty", sel.Settings.CommitBranchVariant)
	}
	if sel.Timeout == 0 {
		t.Error("Timeout = 0, want the answered timeout")
	}
}

func TestConfigScreen_Resume_DriftedVersions_AsksTheTimeoutThenVersionDrift(t *testing.T) {
	s := newRecordedResume("0.9", "1.0")

	got := resumePrompts(t, s)

	assertPrompts(t, got, promptTimeout, promptDrift)
}

// ===== The pending commit-setup retry =====

func newPendingCommitRetry() *ConfigScreen {
	s := newRecordedResume("1.0", "1.0")
	s.SetCommitSetupPending(true)
	return s
}

func TestConfigScreen_Resume_PendingCommitSetup_AsksTheCommitBranchAndTheTimeoutOnly(t *testing.T) {
	s := newPendingCommitRetry()

	got := resumePrompts(t, s)

	if len(got) != 2 {
		t.Fatalf("prompts shown = %v, want exactly the timeout and the commit branch", got)
	}
	seen := map[string]bool{got[0]: true, got[1]: true}
	if !seen[promptTimeout] || !seen[promptCommitBranch] {
		t.Errorf("prompts shown = %v, want the timeout and the commit branch", got)
	}
}

func TestConfigScreen_Resume_PendingCommitSetup_RecordsTheChosenBranchVariant(t *testing.T) {
	s := newPendingCommitRetry()
	resumePrompts(t, s)

	sel := s.Selection()

	if !sel.Supplied.CommitBranchVariant {
		t.Error("Supplied.CommitBranchVariant = false, want true: the user chose it")
	}
	if want := domain.CommitBranchVariants()[0]; sel.Settings.CommitBranchVariant != want {
		t.Errorf("CommitBranchVariant = %q, want %q", sel.Settings.CommitBranchVariant, want)
	}
}

// ===== A run that records no mode (adoption) =====

func newAdoptionResume(recordedVersion, currentVersion string) *ConfigScreen {
	s := newRecordedResume(recordedVersion, currentVersion)
	s.SetNeedsRunnerAdoption(true)
	return s
}

func TestConfigScreen_Resume_Adoption_AsksModePreConsultationAndManualResolutionOnly(t *testing.T) {
	s := newAdoptionResume("1.0", "1.0")

	got := resumePrompts(t, s) // advanceOnce picks auto

	assertPrompts(t, got, promptMode, promptTimeout, promptPreConsult, promptManual)
}

func TestConfigScreen_Resume_Adoption_OrchestratedMode_SkipsPreConsultation(t *testing.T) {
	s := newAdoptionResume("1.0", "1.0")
	if !driveUntilShowing(s, modePromptMarker) {
		t.Fatal("precondition: the mode prompt is not shown")
	}
	pressKey(s, tea.KeyDown) // orchestrated
	pressKey(s, tea.KeyEnter)

	got := resumePrompts(t, s)

	assertPrompts(t, got, promptTimeout, promptManual)
}

func TestConfigScreen_Resume_Adoption_NeverAsksCheckpointsCommitsOrInfrastructure(t *testing.T) {
	s := newAdoptionResume("0.9", "1.0")

	got := resumePrompts(t, s)

	for _, banned := range []string{promptCheckpoints, promptCommits, promptCommitBranch, promptReviewLimit, promptInfraClass} {
		for _, name := range got {
			if name == banned {
				t.Errorf("an adoption resume asked %q; prompts shown = %v", banned, got)
			}
		}
	}
}

// ===== Back-navigation never lands on a skipped step =====

// assertBackReturnsToThePreviousQuestion checks, for every prompt the forward
// pass shows, that esc returns to the prompt shown before it (or leaves the
// wizard from the first one) and so never lands on a step the forward pass
// skipped.
func assertBackReturnsToThePreviousQuestion(t *testing.T, build func() *ConfigScreen) {
	t.Helper()
	forward := resumePrompts(t, build())
	for k := range forward {
		s := build()
		for i := 0; i < k; i++ {
			advanceResume(s)
		}
		if got := promptName(s); got != forward[k] {
			t.Fatalf("forward pass drifted: prompt %d = %q, want %q", k, got, forward[k])
		}

		pressKey(s, tea.KeyEsc)

		if k == 0 {
			if !s.Back() {
				t.Errorf("esc on the first prompt %q did not leave the wizard (showing %q)", forward[0], promptName(s))
			}
			continue
		}
		if got := promptName(s); got != forward[k-1] {
			t.Errorf("esc from %q landed on %q, want %q (forward order %v)", forward[k], got, forward[k-1], forward)
		}
	}
}

func TestConfigScreen_Resume_Back_RecordedRun(t *testing.T) {
	assertBackReturnsToThePreviousQuestion(t, func() *ConfigScreen { return newRecordedResume("1.0", "1.0") })
}

func TestConfigScreen_Resume_Back_DriftedVersions(t *testing.T) {
	assertBackReturnsToThePreviousQuestion(t, func() *ConfigScreen { return newRecordedResume("0.9", "1.0") })
}

func TestConfigScreen_Resume_Back_PendingCommitSetup(t *testing.T) {
	assertBackReturnsToThePreviousQuestion(t, newPendingCommitRetry)
}

func TestConfigScreen_Resume_Back_Adoption(t *testing.T) {
	assertBackReturnsToThePreviousQuestion(t, func() *ConfigScreen { return newAdoptionResume("0.9", "1.0") })
}

// ===== A new run is unchanged =====

func TestConfigScreen_NewRun_AsksEveryQuestionAsBefore(t *testing.T) {
	s := newDrivenScreen(true, recordedRunAgents())
	s.SetVersionDriftInfo("", "")

	got := resumePrompts(t, s)

	assertPrompts(t, got,
		promptMode, promptTimeout, promptCheckpoints, promptCommits,
		promptPreConsult, promptManual, promptInfraClass, promptInfraClass, promptReviewLimit)
}
