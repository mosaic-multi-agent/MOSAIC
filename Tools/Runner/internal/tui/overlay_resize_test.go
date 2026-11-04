package tui

// Every kind of question overlay follows a terminal resize: its content is laid
// out for the new terminal size, as if the question had been asked at that size.
// For the multi-select question this includes the open add-entry field.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/interaction"
	"mosaic-run/internal/tui/screens/inputtest"
)

// longTitle is wider than 40 cells and narrower than 120, so it fits on one line
// in a wide terminal and must wrap in a narrow one.
var longTitle = strings.TrimSpace(strings.Repeat("alpha bravo charlie ", 5))

// openOverlayAt asks q over the progress screen on a terminal of the given width.
func openOverlayAt(t *testing.T, width int, qMsg questionMsg) *rootModel {
	t.Helper()
	m := newTestModel()
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	resizeTerminal(m, width, 30)
	qMsg.reply = make(chan answerMsg, 1)
	m.Update(qMsg)
	if m.screen != screenQuestion {
		t.Fatalf("precondition: question overlay not open, screen = %v", m.screen)
	}
	return m
}

func TestSelectOverlay_FollowsTerminalResize(t *testing.T) {
	q := newTestChoiceQuestion(longTitle, []string{"one", "two"})
	q.Prompt = longTitle
	q.AllowSkip = true
	m := openOverlayAt(t, 120, questionMsg{kind: questionSelectOne, choiceQ: q})

	resizeTerminal(m, 40, 30)

	if m.selectOverlay == nil {
		t.Fatal("select overlay closed by the resize")
	}
	inputtest.AssertLinesWithin(t, m.selectOverlay.view(), 40)
}

func TestConfirmOverlay_FollowsTerminalResize(t *testing.T) {
	q := interaction.Question{Title: longTitle, Prompt: longTitle}
	m := openOverlayAt(t, 120, questionMsg{kind: questionConfirm, confirmQ: q})

	resizeTerminal(m, 40, 30)

	if m.confirmOverlay == nil {
		t.Fatal("confirm overlay closed by the resize")
	}
	inputtest.AssertLinesWithin(t, m.confirmOverlay.view(), 40)
}

func TestMultiSelectOverlay_FollowsTerminalResize(t *testing.T) {
	q := newTestChoiceQuestion(longTitle, []string{"one", "two"})
	q.Prompt = longTitle
	m := openOverlayAt(t, 120, questionMsg{kind: questionSelectMany, choiceQ: q})

	resizeTerminal(m, 40, 30)

	if m.multiOverlay == nil {
		t.Fatal("multi-select overlay closed by the resize")
	}
	inputtest.AssertLinesWithin(t, m.multiOverlay.view(), 40)
}

func TestTextOverlay_LongTitleAndValueFollowTerminalResize(t *testing.T) {
	q := interaction.TextQuestion{
		Question: interaction.Question{Title: longTitle, Detail: longTitle},
		Default:  inputtest.WideValue(),
	}
	m := openOverlayAt(t, 120, questionMsg{kind: questionAskText, textQ: q})

	resizeTerminal(m, 40, 30)

	view := m.textOverlay.view()
	inputtest.AssertLinesWithin(t, view, 40)
	inputtest.AssertTailLineWithin(t, view, 40)
}

// openAddEntryField opens the multi-select add-entry row and types the wide
// value into it.
func openAddEntryField(t *testing.T, width int) *rootModel {
	t.Helper()
	q := newTestChoiceQuestion("Pick items", []string{"one"})
	q.AllowCustom = true
	q.CustomPrompt = "Add item"
	m := openOverlayAt(t, width, questionMsg{kind: questionSelectMany, choiceQ: q})

	sendKey(m, tea.KeyDown) // from the only option onto the add-entry row
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.multiOverlay == nil || !m.multiOverlay.list.Editing() {
		t.Fatal("precondition: the add-entry field is not open")
	}
	inputtest.Type(func(msg tea.Msg) tea.Cmd { m.Update(msg); return nil }, inputtest.WideValue())
	return m
}

func TestMultiSelectOverlay_WideCustomEntryStaysWithinWidthWithCursorVisible(t *testing.T) {
	m := openAddEntryField(t, 80)

	view := m.multiOverlay.view()

	inputtest.AssertLinesWithin(t, view, 80)
	inputtest.AssertTailLineWithin(t, view, 80)
}

func TestMultiSelectOverlay_OpenCustomEntryFollowsNarrowerTerminal(t *testing.T) {
	m := openAddEntryField(t, 100)

	resizeTerminal(m, 50, 30)

	view := m.multiOverlay.view()
	inputtest.AssertLinesWithin(t, view, 50)
	inputtest.AssertTailLineWithin(t, view, 50)
	if !m.multiOverlay.list.Editing() {
		t.Error("the resize closed the add-entry field")
	}
}

func TestMultiSelectOverlay_ResizeKeepsTypedCustomEntry(t *testing.T) {
	m := openAddEntryField(t, 100)

	resizeTerminal(m, 50, 30)
	sendKey(m, tea.KeyEnter)

	got := m.multiOverlay.answer().Custom
	if len(got) != 1 || got[0] != inputtest.WideValue() {
		t.Errorf("custom entries after resize and Enter = %q, want the typed value %q", got, inputtest.WideValue())
	}
}

func TestTextOverlay_ResizeKeepsTypedValue(t *testing.T) {
	m := openOverlayAt(t, 100, questionMsg{kind: questionAskText, textQ: interaction.TextQuestion{Question: interaction.Question{Title: "Enter value:"}}})
	inputtest.Type(func(msg tea.Msg) tea.Cmd { m.Update(msg); return nil }, "hello")

	resizeTerminal(m, 50, 30)

	if m.textOverlay == nil {
		t.Fatal("text overlay closed by the resize")
	}
	if got := m.textOverlay.answer().Text; got != "hello" {
		t.Errorf("text after resize = %q, want %q", got, "hello")
	}
}

func TestSelectOverlay_ResizeKeepsCursorRow(t *testing.T) {
	q := newTestChoiceQuestion("Pick one", []string{"one", "two"})
	m := openOverlayAt(t, 100, questionMsg{kind: questionSelectOne, choiceQ: q})
	sendKey(m, tea.KeyDown)

	resizeTerminal(m, 50, 30)

	if m.selectOverlay == nil {
		t.Fatal("select overlay closed by the resize")
	}
	if got := m.selectOverlay.answer().OptionID; got != "two" {
		t.Errorf("selected option after resize = %q, want %q", got, "two")
	}
}

func TestMultiSelectOverlay_ResizeKeepsToggledOptions(t *testing.T) {
	q := newTestChoiceQuestion("Pick items", []string{"one", "two"})
	m := openOverlayAt(t, 100, questionMsg{kind: questionSelectMany, choiceQ: q})
	sendKey(m, tea.KeyDown)
	m.Update(tea.KeyMsg{Type: tea.KeySpace})

	resizeTerminal(m, 50, 30)

	if m.multiOverlay == nil {
		t.Fatal("multi-select overlay closed by the resize")
	}
	got := m.multiOverlay.answer().OptionIDs
	if len(got) != 1 || got[0] != "two" {
		t.Errorf("checked options after resize = %q, want [two]", got)
	}
}

func TestMultilineTextOverlay_FollowsTerminalResize(t *testing.T) {
	q := interaction.TextQuestion{
		Question:  interaction.Question{Title: longTitle, Detail: longTitle},
		Default:   "first line",
		Multiline: true,
	}
	m := openOverlayAt(t, 120, questionMsg{kind: questionAskText, textQ: q})

	resizeTerminal(m, 40, 30)

	if m.textOverlay == nil {
		t.Fatal("multiline text overlay closed by the resize")
	}
	inputtest.AssertLinesWithin(t, m.textOverlay.view(), 40)
}
