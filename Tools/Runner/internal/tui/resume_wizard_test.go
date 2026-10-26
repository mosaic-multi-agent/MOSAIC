package tui

// resume_wizard_test.go drives a resume through the model, from run selection
// to the end of the configuration wizard, and checks which questions the user
// is asked. A resumed run records its checkpoints, commits, commit branch, mode,
// pre-consultation, manual resolution, review loop limit and infrastructure
// selections, so none of those is asked again.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// recordedSettingQuestions are the questions a resume must never ask when the
// artifact records the setting.
var recordedSettingQuestions = []string{
	"checkpoints", "commits", "commit-branch", "mode", "pre-consultation",
	"manual-resolution", "review-limit", "infra-class",
}

func assertNeverAsked(t *testing.T, asked []string, banned []string) {
	t.Helper()
	for _, b := range banned {
		for _, a := range asked {
			if a == b {
				t.Errorf("the resume asked %q, which the run records; questions asked = %v", b, asked)
			}
		}
	}
}

func assertAsked(t *testing.T, asked []string, want ...string) {
	t.Helper()
	if strings.Join(asked, ",") != strings.Join(want, ",") {
		t.Errorf("questions asked = %v, want %v", asked, want)
	}
}

func TestResumeWizard_FullyRecordedRun_AsksOnlyTheTimeout(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "timeout")
}

func TestResumeWizard_FullyRecordedRun_NeverAsksARecordedSetting(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(true)

	assertNeverAsked(t, asked, recordedSettingQuestions)
}

func TestResumeWizard_DriftedVersion_AsksTheTimeoutThenVersionDrift(t *testing.T) {
	a := fullyRecorded()
	a.version = "0.9"
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "timeout", "drift")
}

func TestResumeWizard_PendingCommitSetup_AsksTheCommitBranch(t *testing.T) {
	a := fullyRecorded()
	a.branch = "" // commits are on, but commit setup never recorded a branch
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	found := false
	for _, name := range asked {
		found = found || name == "commit-branch"
	}
	if !found {
		t.Errorf("a resume that retries commit setup did not ask the commit branch; asked = %v", asked)
	}
	assertNeverAsked(t, asked, []string{
		"checkpoints", "commits", "mode", "pre-consultation", "manual-resolution", "review-limit", "infra-class",
	})
}

func TestResumeWizard_RunThatRecordsNoMode_AsksModePreConsultationAndManualResolution(t *testing.T) {
	a := fullyRecorded()
	// a native-created artifact records no Runner setting, only the agent selections
	a.settings = domain.RunSettings{InfraClassSelections: recordedSettings().InfraClassSelections}
	a.branch = ""
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "mode", "timeout", "pre-consultation", "manual-resolution")
}

// The branch question was the one whose answer produced a refusal after every
// question had been answered: the artifact records the mosaic-owned branch and
// the user's answer supplied the other variant. The user answering the
// remaining questions in the least default way supplies no variant at all now.
func TestResumeWizard_RecordedBranch_UserAnswersNeverSupplyABranchVariant(t *testing.T) {
	for _, branch := range []string{"mosaic/run/" + flowRunID, "main"} {
		t.Run(branch, func(t *testing.T) {
			a := fullyRecorded()
			a.branch = branch
			f := newResumeFlow(t, a, flowOrchestrator{})
			f.toWizard()
			f.answerWizard(true)
			f.key(tea.KeyEnter) // confirm the settings summary

			launched := f.launched()
			if len(launched) != 1 {
				t.Fatalf("sessions launched = %d, want 1 (screen = %v)", len(launched), f.m.screen)
			}
			cfg := launched[0].cfg
			if cfg.Supplied.CommitBranchVariant {
				t.Errorf("a commit-branch variant was supplied (%q) although the run records its branch %q; "+
					"the session refuses a variant that differs from the record", cfg.Settings.CommitBranchVariant, branch)
			}
			if cfg.Supplied != (domain.SuppliedSettings{}) {
				t.Errorf("Supplied = %+v, want nothing supplied: the user was asked about no recorded setting", cfg.Supplied)
			}
		})
	}
}
