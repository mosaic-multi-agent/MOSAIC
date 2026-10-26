package tui

// resume_refusal_tier_test.go covers when a resume is refused. Refusals that
// need only the run artifact (the artifact cannot be read, or records no
// workflow) appear right after the run is chosen, before the user answers any
// question, the harness question included. Refusals that need the deployed
// files (a recorded setting the deployment can no longer satisfy, a workflow the
// orchestrator file no longer declares) appear after the harness question and
// before any configuration question or the settings summary. A workflow version
// mismatch is not a refusal: the version-drift question handles it.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// ===== Tier 1: the artifact alone =====

func TestResumeRefusal_MissingArtifact_AppearsRightAfterRunSelection(t *testing.T) {
	f := newResumeFlow(t, flowArtifact{missing: true}, flowOrchestrator{})

	f.selectRun()

	if f.m.screen != screenDone {
		t.Fatalf("screen = %v after choosing a run whose artifact is missing, want screenDone (%v); "+
			"the refusal must come before the harness question", f.m.screen, screenDone)
	}
	if !f.showing("records no workflow") {
		t.Errorf("the refusal does not say the run records no workflow; view:\n%s", f.m.View())
	}
}

func TestResumeRefusal_UnreadableArtifact_AppearsRightAfterRunSelection(t *testing.T) {
	f := newResumeFlow(t, flowArtifact{garbage: true}, flowOrchestrator{})

	f.selectRun()

	if f.m.screen != screenDone {
		t.Errorf("screen = %v after choosing a run whose artifact cannot be read, want screenDone (%v)",
			f.m.screen, screenDone)
	}
}

func TestResumeRefusal_ArtifactOnlyRefusal_StartsNoSession(t *testing.T) {
	f := newResumeFlow(t, flowArtifact{missing: true}, flowOrchestrator{})

	f.selectRun()
	f.key(tea.KeyEnter)
	f.answerWizard(false)
	f.key(tea.KeyEnter)

	if n := len(f.launched()); n != 0 {
		t.Errorf("%d session(s) launched for a refused run", n)
	}
}

// ===== Tier 2: the deployed files =====

func TestResumeRefusal_DeploymentCannotSatisfyARecordedSetting_AppearsAfterTheHarnessQuestion(t *testing.T) {
	// The run records checkpoints and commits but no agent selection; the
	// orchestrator file declares no infrastructure agent.
	a := fullyRecorded()
	a.settings.InfraClassSelections = nil
	f := newResumeFlow(t, a, flowOrchestrator{noInfra: true})

	f.selectRun()
	if f.m.screen != screenSetupHarness {
		t.Fatalf("screen = %v after choosing the run, want screenSetupHarness (%v): a refusal that needs "+
			"the deployed files cannot be shown before the harness is known", f.m.screen, screenSetupHarness)
	}
	f.answerHarness()

	if f.m.screen != screenDone {
		t.Fatalf("screen = %v after the harness question, want screenDone (%v); the refusal must come "+
			"before any configuration question", f.m.screen, screenDone)
	}
	if !f.showing("no checkpoint provider") && !f.showing("no commit-class infrastructure agent") &&
		!f.showing("no infrastructure agent") {
		t.Errorf("the refusal does not say which recorded setting the deployment cannot satisfy; view:\n%s",
			f.m.View())
	}
}

func TestResumeRefusal_WorkflowNoLongerDeclared_AppearsAfterTheHarnessQuestion(t *testing.T) {
	a := fullyRecorded()
	a.workflow = domain.WorkflowID("retired-flow")
	f := newResumeFlow(t, a, flowOrchestrator{})

	f.selectRun()
	if f.m.screen != screenSetupHarness {
		t.Fatalf("screen = %v after choosing the run, want screenSetupHarness (%v)", f.m.screen, screenSetupHarness)
	}
	f.answerHarness()

	if f.m.screen != screenDone {
		t.Fatalf("screen = %v after the harness question, want screenDone (%v)", f.m.screen, screenDone)
	}
	if !f.showing("retired-flow") || !f.showing("not declared") {
		t.Errorf("the refusal does not name the workflow the deployment no longer declares; view:\n%s", f.m.View())
	}
}

func TestResumeRefusal_DeploymentRefusal_AsksNoConfigurationQuestionAndShowsNoSummary(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{noInfra: true})

	f.selectRun()
	f.answerHarness()

	if f.showing(summaryTitle) {
		t.Errorf("the settings summary was shown for a refused run; view:\n%s", f.m.View())
	}
	if f.m.screen == screenSetupConfig {
		t.Errorf("a configuration question was shown for a refused run: %q", f.prompt())
	}
	if n := len(f.launched()); n != 0 {
		t.Errorf("%d session(s) launched for a refused run", n)
	}
}

// ===== A workflow version mismatch is not a refusal =====

func TestResumeRefusal_VersionMismatch_IsNotRefusedAndLeadsToTheVersionDriftQuestion(t *testing.T) {
	a := fullyRecorded()
	a.version = "0.9"
	f := newResumeFlow(t, a, flowOrchestrator{})

	f.selectRun()
	if f.m.screen == screenDone {
		t.Fatalf("a workflow version mismatch refused the run right after run selection; view:\n%s", f.m.View())
	}
	f.answerHarness()

	if f.m.screen != screenSetupConfig {
		t.Fatalf("screen = %v after the harness question, want screenSetupConfig (%v): a version mismatch "+
			"is not a refusal", f.m.screen, screenSetupConfig)
	}
	f.typeText("45m")
	f.key(tea.KeyEnter)
	if got := f.prompt(); got != "drift" {
		t.Errorf("after the timeout the wizard shows %q, want the version-drift question", got)
	}
}

// A run the deployment can fully satisfy is not refused at either tier.
func TestResumeRefusal_SatisfiableRun_ReachesTheConfigurationWizard(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})

	f.selectRun()
	f.answerHarness()

	if f.m.screen != screenSetupConfig {
		t.Errorf("screen = %v for a run the deployment satisfies, want screenSetupConfig (%v)",
			f.m.screen, screenSetupConfig)
	}
}
