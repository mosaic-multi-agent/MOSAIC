package tui

// resume_summary_test.go covers the settings summary a resume shows before the
// first dispatch: one read-only screen listing every recorded setting (the same
// text the CLI prints), which the user confirms to start the run or leaves to go
// back to the previous question.

import (
	"context"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/resumecheck"
)

// toSummary drives a resume through the wizard and returns with the model on
// the settings summary; the test stops there if it is not shown.
func (f *resumeFlow) toSummary() {
	f.t.Helper()
	f.toWizard()
	f.answerWizard(false)
	if !f.showing(summaryTitle) {
		f.t.Fatalf("the settings summary is not shown after the wizard; screen = %v, view:\n%s", f.m.screen, f.m.View())
	}
}

// sharedSummaryLines returns the lines the CLI prints for the same run: the
// shared summary built from the run's artifact and the deployed infrastructure.
func (f *resumeFlow) sharedSummaryLines() []string {
	f.t.Helper()
	state, err := artifact.NewFileStore(filepath.Join(f.folder, "Orchestration.md")).Read(context.Background())
	if err != nil {
		f.t.Fatalf("read the run artifact: %v", err)
	}
	agents, err := orchfile.EnumerateInfrastructureAgents(f.orchPath)
	if err != nil {
		f.t.Fatalf("enumerate infrastructure agents: %v", err)
	}
	return resumecheck.NewSummary(state, agents).Lines()
}

func TestResumeSummary_ShownAfterTheWizardAndBeforeTheFirstDispatch(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})

	f.toSummary()

	if !f.showing(summaryTitle) {
		t.Fatalf("the settings summary is not shown after the wizard; screen = %v, view:\n%s", f.m.screen, f.m.View())
	}
	if n := len(f.launched()); n != 0 {
		t.Errorf("%d session(s) launched before the summary was confirmed", n)
	}
	if f.m.screen == screenProgress {
		t.Error("the run started before the summary was confirmed")
	}
}

func TestResumeSummary_MatchesTheSharedSummaryTheCLIPrints(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()

	for _, line := range f.sharedSummaryLines() {
		if !f.showing(line) {
			t.Errorf("the summary screen lacks the shared summary line %q; view:\n%s", line, f.m.View())
		}
	}
}

func TestResumeSummary_ListsEveryRecordedSetting(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()

	for _, want := range []string{
		"Recorded settings for run " + flowRunID,
		"Workflow: " + recordedWorkflowID + " (version " + deployedVersion + ")",
		"Mode: auto",
		"Pre-consultation: on",
		"Manual resolution: on",
		"Checkpoints: on (cp-b)",
		"Commits: on (branch mosaic/run/" + flowRunID,
		"Review loop limit: 5",
		"checkpoint=cp-b",
		"commit=git-commit",
		"Overrides:",
	} {
		if !f.showing(want) {
			t.Errorf("the summary does not show %q; view:\n%s", want, f.m.View())
		}
	}
}

func TestResumeSummary_Confirm_StartsTheRun(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()

	f.key(tea.KeyEnter)

	if f.m.screen != screenProgress {
		t.Errorf("screen = %v after confirming the summary, want screenProgress (%v)", f.m.screen, screenProgress)
	}
	launched := f.launched()
	if len(launched) != 1 {
		t.Fatalf("sessions launched = %d after confirming, want 1", len(launched))
	}
	if launched[0].isNewRun {
		t.Error("the launched session is a new run, want a resume")
	}
	if launched[0].cfg.Timeout.String() != "45m0s" {
		t.Errorf("launched with timeout %v, want the answered 45m", launched[0].cfg.Timeout)
	}
}

// The saved-guidance review is asked by the session, through the question
// overlay, once the run has started: confirming the summary must leave it
// reachable.
func TestResumeSummary_Confirm_LeavesTheGuidanceReviewReachable(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()
	f.key(tea.KeyEnter)
	if f.m.screen != screenProgress {
		t.Fatalf("precondition: screen = %v, want screenProgress", f.m.screen)
	}

	reply := make(chan answerMsg, 1)
	f.m.Update(questionMsg{kind: questionSelectOne, choiceQ: guidanceReviewQuestion(guidanceDetail("keep it small", ""), true), reply: reply})

	if f.m.screen != screenQuestion {
		t.Errorf("screen = %v after the session asked the guidance review, want screenQuestion (%v)",
			f.m.screen, screenQuestion)
	}
}

func TestResumeSummary_Back_ReturnsToThePreviousQuestionWithoutStarting(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()

	f.key(tea.KeyEsc)

	if f.m.screen != screenSetupConfig {
		t.Fatalf("screen = %v after going back from the summary, want screenSetupConfig (%v)",
			f.m.screen, screenSetupConfig)
	}
	if got := f.prompt(); got != "timeout" {
		t.Errorf("going back showed %q, want the last question asked (the timeout)", got)
	}
	if n := len(f.launched()); n != 0 {
		t.Errorf("%d session(s) launched by going back", n)
	}
}

func TestResumeSummary_Back_ThenForward_KeepsTheEarlierAnswer(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.toSummary()
	f.key(tea.KeyEsc)

	f.key(tea.KeyEnter) // accept the timeout as it was answered

	if !f.showing(summaryTitle) {
		t.Fatalf("the summary is not shown again after answering the same question; screen = %v, view:\n%s",
			f.m.screen, f.m.View())
	}
	f.key(tea.KeyEnter)
	launched := f.launched()
	if len(launched) != 1 {
		t.Fatalf("sessions launched = %d, want 1", len(launched))
	}
	if launched[0].cfg.Timeout.String() != "45m0s" {
		t.Errorf("launched with timeout %v, want the earlier answer 45m", launched[0].cfg.Timeout)
	}
}

func TestResumeSummary_Back_FromADriftedRun_ReturnsToTheVersionDriftQuestion(t *testing.T) {
	a := fullyRecorded()
	a.version = "0.9"
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toSummary()
	if !f.showing(summaryTitle) {
		t.Fatalf("precondition: the summary is not shown; screen = %v", f.m.screen)
	}

	f.key(tea.KeyEsc)

	if f.m.screen != screenSetupConfig || f.prompt() != "drift" {
		t.Errorf("going back landed on screen %v prompt %q, want the version-drift question",
			f.m.screen, f.prompt())
	}
}

func TestResumeSummary_RunThatRecordsNoMode_ShowsTheAdoptedSettings(t *testing.T) {
	a := fullyRecorded()
	a.settings = domain.RunSettings{InfraClassSelections: recordedSettings().InfraClassSelections}
	a.branch = ""
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toSummary() // answers: auto, pre-consultation on (default), manual resolution off

	if !f.showing(summaryTitle) {
		t.Fatalf("the summary is not shown; screen = %v, view:\n%s", f.m.screen, f.m.View())
	}
	if !f.showing("Mode: auto") {
		t.Errorf("the summary does not show the mode being adopted; view:\n%s", f.m.View())
	}
}

// The summary belongs to a resume: a new run is not shown one.
func TestResumeSummary_NewRun_StartsWithoutASummary(t *testing.T) {
	m := newTestModelNewRun()
	wf := domain.WorkflowRegion{Info: domain.WorkflowInfo{ID: "test-workflow", Version: "1.0"}}
	driveModelToConfigScreen(t, m, wf)
	f := &resumeFlow{t: t, m: m}

	f.answerWizard(false)

	if f.m.screen != screenProgress {
		t.Errorf("screen = %v after a new run's wizard, want screenProgress (%v)", f.m.screen, screenProgress)
	}
	if f.showing(summaryTitle) {
		t.Error("a new run was shown the resume summary")
	}
}

// The summary is the last screen before the session starts on every harness,
// including the one that asks for a permission mode after configuration.
func TestResumeSummary_GHCPHarness_SummaryFollowsThePermissionModeScreen(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.selectRun()
	idx := -1
	for i, sel := range harness.CLISelections() {
		if sel.ID == "ghcp-cli" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the ghcp-cli harness is not offered")
	}
	for i := 0; i < idx; i++ {
		f.key(tea.KeyDown)
	}
	f.answerHarness()
	f.answerWizard(false)
	if f.m.screen != screenSetupGHCPMode {
		t.Fatalf("screen = %v after the wizard on ghcp-cli, want screenSetupGHCPMode (%v)", f.m.screen, screenSetupGHCPMode)
	}

	f.key(tea.KeyEnter) // accept the permission mode

	if !f.showing(summaryTitle) {
		t.Fatalf("the summary is not shown after the permission mode; screen = %v, view:\n%s", f.m.screen, f.m.View())
	}
	if n := len(f.launched()); n != 0 {
		t.Errorf("%d session(s) launched before the summary was confirmed", n)
	}
	f.key(tea.KeyEnter)
	if f.m.screen != screenProgress {
		t.Errorf("screen = %v after confirming, want screenProgress (%v)", f.m.screen, screenProgress)
	}
}
