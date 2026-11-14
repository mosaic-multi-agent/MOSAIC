package tui

// resume_frontmatter_damage_test.go covers a resume of a run whose frontmatter
// is damaged. With a usable last-good copy the run is offered for resume, marked
// damaged, and the wizard is steered by what the copy records; the session
// started for it repairs the artifact. Without a usable copy the run is shown as
// unresumable and damaged and cannot be chosen.

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/runscan"
	"mosaic-run/internal/runselect"
	"mosaic-run/internal/tui/screens/setup"
)

// damagedFrontmatterWithCopy is a run whose frontmatter is damaged and whose
// last-good copy is the artifact as it was written.
func damagedFrontmatterWithCopy(a flowArtifact) flowArtifact {
	a.damagedFrontmatter = true
	a.lastGoodCopy = true
	return a
}

func TestResumeFrontmatterDamage_WithLastGoodCopy_RunSelectionDoesNotRefuseTheRun(t *testing.T) {
	f := newResumeFlow(t, damagedFrontmatterWithCopy(fullyRecorded()), flowOrchestrator{})

	f.selectRun()

	if f.m.screen != screenSetupHarness {
		t.Fatalf("screen = %v after choosing the run, want screenSetupHarness (%v); view:\n%s",
			f.m.screen, screenSetupHarness, f.m.View())
	}
}

func TestResumeFrontmatterDamage_WithLastGoodCopy_FullyRecordedRunAsksOnlyTheTwoTimeouts(t *testing.T) {
	f := newResumeFlow(t, damagedFrontmatterWithCopy(fullyRecorded()), flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "timeout", "hitl-timeout")
}

func TestResumeFrontmatterDamage_WithLastGoodCopy_PendingCommitSetupAsksTheCommitBranch(t *testing.T) {
	a := damagedFrontmatterWithCopy(fullyRecorded())
	a.branch = ""
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	found := false
	for _, name := range asked {
		found = found || name == "commit-branch"
	}
	if !found {
		t.Errorf("a run whose copy records a pending commit setup was not asked the commit branch; asked = %v", asked)
	}
}

func TestResumeFrontmatterDamage_WithLastGoodCopy_DriftedVersionAsksVersionDriftFromTheCopysVersion(t *testing.T) {
	a := damagedFrontmatterWithCopy(fullyRecorded())
	a.version = "0.9"
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.toWizard()

	asked := f.answerWizard(false)

	assertAsked(t, asked, "timeout", "hitl-timeout", "drift")
}

func TestResumeFrontmatterDamage_WithLastGoodCopy_AnnouncementIsMarkedDamagedAndKeepsTheCopysPosition(t *testing.T) {
	f := newResumeFlow(t, damagedFrontmatterWithCopy(fullyRecorded()), flowOrchestrator{})
	f.selectRun()

	id := f.m.announceIdentity()

	if !id.Damaged {
		t.Error("Identity.Damaged = false, want true for a run whose frontmatter is damaged")
	}
	if id.Position == nil {
		t.Error("Identity.Position = nil, want the position the last-good copy records")
	}
}

func TestResumeFrontmatterDamage_WithLastGoodCopy_StartsAResumeSessionForTheRun(t *testing.T) {
	f := newResumeFlow(t, damagedFrontmatterWithCopy(fullyRecorded()), flowOrchestrator{})
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
}

func TestResumeFrontmatterDamage_WithoutUsableCopy_AnUnusableCopyIsRefusedNamingTheDamage(t *testing.T) {
	a := fullyRecorded()
	a.damagedFrontmatter = true
	f := newResumeFlow(t, a, flowOrchestrator{})
	// A copy that is not a readable artifact must not stand in for the run's values.
	copyOfOther := []byte("not an artifact\n")
	writeFlowLastGoodCopy(t, f.folder+"/Orchestration.md", copyOfOther)

	f.selectRun()

	if f.m.screen != screenDone || !f.showing("damaged") {
		t.Errorf("screen = %v, want the refusal naming the damage; view:\n%s", f.m.screen, f.m.View())
	}
	if n := len(f.launched()); n != 0 {
		t.Errorf("%d session(s) launched for a refused run", n)
	}
}

// ===== The resume list =====

const frontmatterDamagedID = "20260701T120000Z-a3f9"

// frontmatterDamagedDetail is the damage text the scan carries; it avoids the
// word "damaged" so the view shows that word only through the reason.
const frontmatterDamagedDetail = "frontmatter problem at line 4"

func scanWithUnresumableDamagedFrontmatter() *runscan.ScanResult {
	return &runscan.ScanResult{
		Candidates: []runscan.RunCandidate{
			newTestCandidate("20260801T090000Z-bbbb", "/ws/Orchestration-20260801T090000Z-bbbb"),
		},
		Unresumable: []runscan.UnresumableRun{{
			RunInfo: runscan.RunInfo{
				RunID:      frontmatterDamagedID,
				FolderPath: "/ws/Orchestration-" + frontmatterDamagedID,
				Damaged:    true,
			},
			Reason: runscan.ReasonDamagedFrontmatter,
			Detail: frontmatterDamagedDetail,
		}},
	}
}

func TestResumeFrontmatterDamage_ResumeList_ShowsARunWithoutAUsableCopyAsUnresumableAndDamaged(t *testing.T) {
	sess := &stubNavSession{outcome: domain.RunOutcome{Status: domain.RunCompleted, Message: "ok"}}
	m := newRootModel(context.Background(), sess, Options{
		Theme:      tuicommon.DefaultTheme(),
		ScanResult: scanWithUnresumableDamagedFrontmatter(),
	})

	view := m.View()

	if m.screen != screenRunSelect {
		t.Fatalf("precondition: screen = %v, want screenRunSelect", m.screen)
	}
	if !containsStr(view, frontmatterDamagedID) {
		t.Errorf("the run is not listed.\nview:\n%s", view)
	}
	if !containsStr(view, "damaged") {
		t.Errorf("the run is not shown as damaged.\nview:\n%s", view)
	}
	if !containsStr(view, frontmatterDamagedDetail) {
		t.Errorf("the view does not show what is wrong with the run (%q).\nview:\n%s", frontmatterDamagedDetail, view)
	}
}

func TestResumeFrontmatterDamage_RunSelectScreen_UnresumableDamagedChoiceIsShownAndCannotBeChosen(t *testing.T) {
	unresumable := newTestUnresumableChoice(frontmatterDamagedID, runscan.ReasonDamagedFrontmatter)
	unresumable.Run.Damaged = true
	unresumable.Detail = frontmatterDamagedDetail
	q := runselect.Question{Choices: []runselect.Choice{
		newRunChoiceFixture(),
		unresumable,
		newTestResumeChoice("20260801T090000Z-bbbb"),
	}}
	s := setup.NewRunSelectScreen(q, 100, 24, stylesFromTheme(tuicommon.DefaultTheme()))

	view := s.View()
	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !containsStr(view, frontmatterDamagedID) || !containsStr(view, "damaged") {
		t.Errorf("the run must be shown as unresumable and damaged.\nview:\n%s", view)
	}
	if !s.Done() || s.SelectedChoiceID() != "20260801T090000Z-bbbb" {
		t.Errorf("SelectedChoiceID() = %q (done %v), want the cursor to skip the unresumable run", s.SelectedChoiceID(), s.Done())
	}
}
