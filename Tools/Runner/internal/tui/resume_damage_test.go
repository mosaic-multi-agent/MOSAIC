package tui

// resume_damage_test.go covers a resume of a run whose Orchestration.md is
// damaged. The run is neither hidden nor refused before it starts: it is listed
// marked damaged, its recorded facts (version, runner settings, commit setup)
// still steer the wizard, and the session started for it repairs the artifact.
// A damaged frontmatter records nothing to resume from, so that run is refused
// naming the damage.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/runscan"
	"mosaic-run/internal/runselect"
	"mosaic-run/internal/tui/screens/setup"
)

func damagedArtifact(a flowArtifact) flowArtifact {
	a.damaged = true
	return a
}

// ===== Resume facts: the frontmatter still answers =====

func TestResumeDamage_RunSelection_DoesNotRefuseTheRun(t *testing.T) {
	f := newResumeFlow(t, damagedArtifact(fullyRecorded()), flowOrchestrator{})

	f.selectRun()

	if f.m.screen != screenSetupHarness {
		t.Fatalf("screen = %v after choosing a damaged run, want screenSetupHarness (%v); "+
			"view:\n%s", f.m.screen, screenSetupHarness, f.m.View())
	}
}

func TestResumeDamage_FullyRecordedRun_AsksOnlyTheTwoTimeouts(t *testing.T) {
	f := newResumeFlow(t, damagedArtifact(fullyRecorded()), flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "timeout", "hitl-timeout")
}

func TestResumeDamage_DriftedVersion_AsksVersionDriftFromTheRecordedVersion(t *testing.T) {
	a := damagedArtifact(fullyRecorded())
	a.version = "0.9"
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "timeout", "hitl-timeout", "drift")
}

func TestResumeDamage_PendingCommitSetup_AsksTheCommitBranch(t *testing.T) {
	a := damagedArtifact(fullyRecorded())
	a.branch = ""
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	found := false
	for _, name := range asked {
		found = found || name == "commit-branch"
	}
	if !found {
		t.Errorf("a damaged run with a pending commit setup was not asked the commit branch; asked = %v", asked)
	}
}

func TestResumeDamage_RunThatRecordsNoMode_AsksModePreConsultationAndManualResolution(t *testing.T) {
	a := damagedArtifact(fullyRecorded())
	a.settings = domain.RunSettings{InfraClassSelections: recordedSettings().InfraClassSelections}
	a.branch = ""
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "mode", "timeout", "hitl-timeout", "pre-consultation", "manual-resolution")
}

func TestResumeDamage_AnnouncementIdentity_IsMarkedDamagedAndKeepsTheRecordedPosition(t *testing.T) {
	f := newResumeFlow(t, damagedArtifact(fullyRecorded()), flowOrchestrator{})
	f.selectRun()

	id := f.m.announceIdentity()

	if !id.Damaged {
		t.Error("Identity.Damaged = false, want true for a run whose artifact is damaged")
	}
	if id.Position == nil {
		t.Error("Identity.Position = nil, want the position the intact frontmatter records")
	}
}

func TestResumeDamage_AnnouncementIdentity_OfAnUndamagedRun_IsNotMarkedDamaged(t *testing.T) {
	f := newResumeFlow(t, fullyRecorded(), flowOrchestrator{})
	f.selectRun()

	if f.m.announceIdentity().Damaged {
		t.Error("Identity.Damaged = true for a run that is not damaged")
	}
}

func TestResumeDamage_StartsAResumeSessionForTheRun(t *testing.T) {
	f := newResumeFlow(t, damagedArtifact(fullyRecorded()), flowOrchestrator{})
	f.toWizard()
	f.answerWizard(false)
	f.key(tea.KeyEnter) // confirm the settings summary

	launched := f.launched()

	if len(launched) != 1 {
		t.Fatalf("sessions launched = %d, want 1 (screen = %v)", len(launched), f.m.screen)
	}
	if launched[0].isNewRun {
		t.Error("the session was built for a new run, want a resume that reaches the session's repair")
	}
	if launched[0].runFolder != f.folder {
		t.Errorf("session run folder = %q, want %q", launched[0].runFolder, f.folder)
	}
}

// ===== A damaged frontmatter records nothing to resume from =====

func TestResumeDamage_DamagedFrontmatterWithoutLastGoodCopy_IsRefusedNamingTheDamage(t *testing.T) {
	a := fullyRecorded()
	a.damagedFrontmatter = true
	f := newResumeFlow(t, a, flowOrchestrator{})

	f.selectRun()

	if f.m.screen != screenDone {
		t.Fatalf("screen = %v, want screenDone (%v)", f.m.screen, screenDone)
	}
	if !f.showing("damaged") {
		t.Errorf("the refusal does not name the damage; view:\n%s", f.m.View())
	}
	if n := len(f.launched()); n != 0 {
		t.Errorf("%d session(s) launched for a refused run", n)
	}
}

// ===== The resume list =====

func TestResumeDamage_ResumeList_ShowsTheDamagedRunMarkedDamaged(t *testing.T) {
	const damagedID = "20260701T120000Z-a3f9"
	damaged := newTestCandidate(damagedID, "/ws/Orchestration-"+damagedID)
	damaged.Damaged = true
	m := newModelWithScan([]runscan.RunCandidate{damaged})

	view := m.View()

	if m.screen != screenRunSelect {
		t.Fatalf("precondition: screen = %v, want screenRunSelect", m.screen)
	}
	if !containsStr(view, damagedID) {
		t.Errorf("the damaged run is not listed.\nview:\n%s", view)
	}
	if !containsStr(view, "damaged") {
		t.Errorf("the damaged run is not marked damaged.\nview:\n%s", view)
	}
}

func TestResumeDamage_ResumeList_DoesNotMarkAnUndamagedRun(t *testing.T) {
	const healthyID = "20260701T120000Z-a3f9"
	m := newModelWithScan([]runscan.RunCandidate{newTestCandidate(healthyID, "/ws/Orchestration-"+healthyID)})

	view := m.View()

	if containsStr(view, "damaged") {
		t.Errorf("an undamaged run is marked damaged.\nview:\n%s", view)
	}
}

func TestResumeDamage_RunSelectScreen_ShowsADamagedChoiceMarkedDamagedAndSelectable(t *testing.T) {
	const damagedID = "20260701T120000Z-a3f9"
	choice := newTestResumeChoice(damagedID)
	choice.Run.Damaged = true
	q := runselect.Question{Choices: []runselect.Choice{newRunChoiceFixture(), choice}}
	s := setup.NewRunSelectScreen(q, 100, 24, stylesFromTheme(tuicommon.DefaultTheme()))

	view := s.View()

	if !containsStr(view, damagedID) || !containsStr(view, "damaged") {
		t.Errorf("the damaged run must be shown marked damaged.\nview:\n%s", view)
	}
}

func TestResumeDamage_ResumeList_DamagedRunCanBeChosen(t *testing.T) {
	f := newResumeFlow(t, damagedArtifact(fullyRecorded()), flowOrchestrator{})

	f.key(tea.KeyDown)
	f.key(tea.KeyEnter)

	if f.m.screen != screenSetupHarness {
		t.Fatalf("screen = %v after choosing the damaged run from the list, want screenSetupHarness (%v); "+
			"view:\n%s", f.m.screen, screenSetupHarness, f.m.View())
	}
	if f.m.selections.runID != flowRunID || f.m.selections.isNewRun {
		t.Errorf("selection = run %q new=%v, want the damaged run %q as a resume",
			f.m.selections.runID, f.m.selections.isNewRun, flowRunID)
	}
}

// ===== History and restart =====
//
// The three tests below are regression guards: the history and restart readers
// already treat any read error as "carry on", so they pass before the damage
// handling exists and are not evidence for it.

func damageError() *domain.ArtifactDamageError {
	return &domain.ArtifactDamageError{
		Resource: restartTestRunFolder + "/Orchestration.md",
		Problems: []domain.ArtifactDamageProblem{{Section: "ExecutionLog", Line: 9, Problem: "row has 3 cells, the header has 8"}},
	}
}

func TestLoadHistory_DamagedArtifact_YieldsNoRowsAndNeverFails(t *testing.T) {
	fx := newRestartFixture(t, &readOnlyStore{err: damageError()})
	fx.m.selections.isNewRun = false

	rows := fx.m.loadHistory()

	if len(rows) != 0 {
		t.Errorf("loadHistory returned %d rows for a damaged artifact, want none", len(rows))
	}
}

func TestRestartResume_DamagedArtifact_StartsSessionAsResume(t *testing.T) {
	fx := newRestartFixture(t, &readOnlyStore{err: damageError()})

	fx.m.restartAsResume(true)

	calls := *fx.factoryCalls
	if len(calls) == 0 {
		t.Fatal("no session was built for the restart")
	}
	if calls[len(calls)-1].isNewRun {
		t.Error("the restart built a session for a new run over a damaged artifact, want a resume")
	}
}

func TestRunArtifactExists_DamagedArtifact_ReportsItExists(t *testing.T) {
	fx := newRestartFixture(t, &readOnlyStore{err: damageError()})

	if !fx.m.runArtifactExists() {
		t.Error("runArtifactExists = false for a damaged artifact, want true: it is there and is repaired when the run starts")
	}
}
