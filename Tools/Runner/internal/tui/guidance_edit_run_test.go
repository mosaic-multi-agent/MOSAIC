package tui

// The guidance key follows the session: it is offered once the session holds
// guidance, withdrawn when the session ends it, and the run keeps delivering
// its messages while the editor is open.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

// activate makes the fixture's session hold g, then delivers a run message as
// the live run does, so the screen can notice the change.
func (fx *guidanceFixture) activate(g domain.Guidance) {
	session.ActivateStandingGuidance(fx.holder, g, fx.store, "/run/folder", fx.log)
	fx.m.Update(stepRecordedMsg{entry: entry("tick#1", "PLANNING", "", domain.StatusSUCCESS)})
}

func TestGuidanceKey_OfferedWhenTheSessionActivatesGuidanceAfterTheScreenExists(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{}, false)
	if view := fx.m.View(); strings.Contains(view, "e guidance") {
		t.Fatalf("precondition: the key is offered before guidance exists:\n%s", view)
	}

	fx.activate(domain.Guidance{TaskDescription: "late guidance"})

	if view := fx.m.View(); !strings.Contains(view, guidanceHelpBar) {
		t.Errorf("help bar lacks the guidance key after the session activated guidance:\n%s", view)
	}
	pressRune(fx.m, "e")
	if view := fx.m.View(); !strings.Contains(view, guidanceTaskLabel) || !strings.Contains(view, "late guidance") {
		t.Errorf("the e key did not open the late guidance:\n%s", view)
	}
}

func TestGuidanceKey_WithdrawnWhenTheSessionEndsItsGuidance(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	if view := fx.m.View(); !strings.Contains(view, guidanceHelpBar) {
		t.Fatalf("precondition: the key is not offered with guidance in effect:\n%s", view)
	}

	session.DeactivateStandingGuidance(fx.holder)
	fx.m.Update(stepRecordedMsg{entry: entry("tick#1", "PLANNING", "", domain.StatusSUCCESS)})
	pressRune(fx.m, "e")

	view := fx.m.View()
	if !strings.Contains(view, guidanceHelpBarWithout) || strings.Contains(view, "e guidance") {
		t.Errorf("help bar still offers the guidance key after the session ended it:\n%s", view)
	}
	if fx.m.screen != screenProgress || strings.Contains(view, guidanceTaskLabel) {
		t.Errorf("the e key opened the editor after the session ended its guidance:\n%s", view)
	}
}

func TestGuidanceKey_UppercaseEOpensTheEditor(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)

	pressRune(fx.m, "E")

	if view := fx.m.View(); !strings.Contains(view, guidanceTaskLabel) {
		t.Errorf("E did not open the guidance editor:\n%s", view)
	}
}

func TestGuidanceKey_ShiftTabMovesFocusBackToTheFirstField(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{}, true)
	fx.openEditor(t)

	sendKey(fx.m, tea.KeyTab)
	typeLines(fx.m, "second field")
	sendKey(fx.m, tea.KeyShiftTab)
	typeLines(fx.m, "first field")
	sendKey(fx.m, tea.KeyCtrlS)

	want := domain.Guidance{TaskDescription: "first field", Constraints: "second field"}
	if saves := fx.store.saves(); len(saves) != 1 || saves[0] != want {
		t.Errorf("saved = %+v, want exactly one save of %+v", saves, want)
	}
}

func TestGuidanceKey_ReopeningAfterEscShowsTheGuidanceInEffectNotTheDiscardedText(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "base"}, true)
	fx.openEditor(t)
	typeLines(fx.m, "", "discarded")
	sendKey(fx.m, tea.KeyEsc)

	pressRune(fx.m, "e")

	view := fx.m.View()
	if !strings.Contains(view, guidanceTaskLabel) || !strings.Contains(view, "base") || strings.Contains(view, "discarded") {
		t.Errorf("reopened editor does not show the unchanged guidance:\n%s", view)
	}
}

func TestGuidanceKey_ReopeningAfterSaveShowsTheSavedGuidance(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "base"}, true)
	fx.openEditor(t)
	typeLines(fx.m, "", "added")
	sendKey(fx.m, tea.KeyCtrlS)

	pressRune(fx.m, "e")

	view := fx.m.View()
	if !strings.Contains(view, "base") || !strings.Contains(view, "added") {
		t.Errorf("reopened editor does not show the saved guidance:\n%s", view)
	}
}

func TestGuidanceKey_SavingWithoutChangesKeepsGuidanceAndReturnsToProgress(t *testing.T) {
	initial := domain.Guidance{TaskDescription: "base", Constraints: "c1"}
	fx := newGuidanceFixture(t, initial, true)
	fx.openEditor(t)

	sendKey(fx.m, tea.KeyCtrlS)

	if got, _ := fx.holder.Current(); got != initial {
		t.Errorf("guidance in effect = %+v, want unchanged %+v", got, initial)
	}
	if fx.m.screen != screenProgress {
		t.Errorf("screen = %v, want the progress screen after saving", fx.m.screen)
	}
}

func TestGuidanceKey_RowsRecordedWhileEditingAreOnTheProgressScreenAfterEsc(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	fx.openEditor(t)
	typeLines(fx.m, "", "typed before the row")

	fx.m.Update(stepRecordedMsg{entry: entry("builder#2", "BUILDING", "", domain.StatusSUCCESS)})
	if view := fx.m.View(); !strings.Contains(view, guidanceTaskLabel) || !strings.Contains(view, "typed before the row") {
		t.Errorf("a recorded step disturbed the open editor:\n%s", view)
	}
	sendKey(fx.m, tea.KeyEsc)

	view := fx.m.View()
	for _, want := range []string{"planner#1", "builder#2"} {
		if !strings.Contains(view, want) {
			t.Errorf("progress view lacks row %q after editing:\n%s", want, view)
		}
	}
}

func TestGuidanceKey_RowsRecordedWhileEditingAreOnTheProgressScreenAfterSave(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	fx.openEditor(t)

	fx.m.Update(stepRecordedMsg{entry: entry("builder#2", "BUILDING", "", domain.StatusSUCCESS)})
	sendKey(fx.m, tea.KeyCtrlS)

	if view := fx.m.View(); !strings.Contains(view, "planner#1") || !strings.Contains(view, "builder#2") {
		t.Errorf("progress view lacks a row after saving:\n%s", view)
	}
}

func TestGuidanceKey_SessionQuestionArrivingWhileEditingIsNotLostAndGuidanceIsKept(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	fx.openEditor(t)
	typeLines(fx.m, "", "unsaved")
	reply := make(chan answerMsg, 1)

	fx.m.Update(questionMsg{kind: questionSelectOne, choiceQ: guidanceReviewQuestion("d", false), reply: reply})
	sendKey(fx.m, tea.KeyEsc)

	if fx.m.screen != screenQuestion {
		t.Errorf("screen = %v after leaving the editor, want the pending session question in front", fx.m.screen)
	}
	select {
	case ans := <-reply:
		t.Errorf("the pending question was answered by the editor's esc: %+v", ans)
	default:
	}
	if got, _ := fx.holder.Current(); got != (domain.Guidance{TaskDescription: "x"}) {
		t.Errorf("guidance in effect = %+v, want unchanged", got)
	}
}

func TestGuidanceKey_RunEndingWhileEditingShowsTheEndOfRunAfterEsc(t *testing.T) {
	fx := newGuidanceFixture(t, domain.Guidance{TaskDescription: "x"}, true)
	fx.openEditor(t)

	fx.m.Update(runDoneMsg{outcome: domain.RunOutcome{Status: domain.RunCompleted}})
	sendKey(fx.m, tea.KeyEsc)

	if fx.m.screen != screenDone {
		t.Errorf("screen = %v after leaving the editor of a finished run, want the done screen", fx.m.screen)
	}
	if view := fx.m.View(); strings.Contains(view, guidanceTaskLabel) {
		t.Errorf("the guidance editor is still shown after the run ended:\n%s", view)
	}
}
