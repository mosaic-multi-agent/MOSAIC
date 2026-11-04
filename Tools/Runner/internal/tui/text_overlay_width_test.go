package tui

// The text question overlay keeps a value wider than the terminal on one line
// within the overlay width, with the cursor and the text next to it visible,
// and follows terminal resize. The manual dispatch task message is such a
// text question.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens/inputtest"
)

// openTextOverlayAt shows q as a text question over the progress screen on a
// terminal of the given width.
func openTextOverlayAt(t *testing.T, width int, q interaction.TextQuestion) *rootModel {
	t.Helper()
	m := newTestModel()
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	resizeTerminal(m, width, 30)
	m.Update(questionMsg{kind: questionAskText, textQ: q, reply: make(chan answerMsg, 1)})
	if m.screen != screenQuestion || m.textOverlay == nil {
		t.Fatalf("precondition: text overlay not open, screen = %v", m.screen)
	}
	return m
}

// manualTaskQuestion is the manual dispatch task message question, prefilled
// with value.
func manualTaskQuestion(value string) interaction.TextQuestion {
	return interaction.TextQuestion{
		Question: interaction.Question{
			ID:     domain.QuestionManualTask,
			Prompt: "Enter the task description for implementer (row 3):",
		},
		Default: value,
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("the task description must not be empty")
			}
			return nil
		},
	}
}

func TestTextOverlay_WideDefaultStaysWithinWidthWithTailVisible(t *testing.T) {
	q := interaction.TextQuestion{Question: interaction.Question{Title: "Enter value:"}, Default: inputtest.WideValue()}
	m := openTextOverlayAt(t, 80, q)

	view := m.textOverlay.view()

	inputtest.AssertLinesWithin(t, view, 80)
	inputtest.AssertTailLineWithin(t, view, 80)
}

func TestTextOverlay_WideTypedValueStaysWithinWidthWithTailVisible(t *testing.T) {
	m := openTextOverlayAt(t, 80, interaction.TextQuestion{Question: interaction.Question{Title: "Enter value:"}})

	inputtest.Type(func(msg tea.Msg) tea.Cmd { m.Update(msg); return nil }, inputtest.WideValue())

	view := m.textOverlay.view()
	inputtest.AssertLinesWithin(t, view, 80)
	inputtest.AssertTailLineWithin(t, view, 80)
}

func TestTextOverlay_ManualDispatchTaskMessageShowsLongValueWithCursorVisible(t *testing.T) {
	m := openTextOverlayAt(t, 80, manualTaskQuestion(inputtest.WideValue()))

	view := m.textOverlay.view()

	inputtest.AssertLinesWithin(t, view, 80)
	inputtest.AssertTailLineWithin(t, view, 80)
}

func TestTextOverlay_CursorMovedToStartShowsStartOfValue(t *testing.T) {
	m := openTextOverlayAt(t, 80, manualTaskQuestion(inputtest.WideValue()))

	sendKey(m, tea.KeyHome)

	view := m.textOverlay.view()
	inputtest.AssertLinesWithin(t, view, 80)
	if !strings.Contains(view, "abcdefghij") {
		t.Errorf("start of the value is not visible with the cursor at the start:\n%s", view)
	}
	if strings.Contains(view, inputtest.Tail) {
		t.Errorf("end of the value is still shown although the value is wider than the overlay:\n%s", view)
	}
}

func TestTextOverlay_NarrowerTerminalKeepsValueWithinNewWidthWithTailVisible(t *testing.T) {
	m := openTextOverlayAt(t, 100, manualTaskQuestion(inputtest.WideValue()))

	resizeTerminal(m, 50, 30)

	view := m.textOverlay.view()
	inputtest.AssertLinesWithin(t, view, 50)
	inputtest.AssertTailLineWithin(t, view, 50)
}

func TestTextOverlay_WiderTerminalShowsMoreOfTheValue(t *testing.T) {
	m := openTextOverlayAt(t, 50, manualTaskQuestion(inputtest.WideValue()))

	narrow := m.textOverlay.view()
	inputtest.AssertLinesWithin(t, narrow, 50)
	inputtest.AssertTailLineWithin(t, narrow, 50)
	if strings.Contains(narrow, inputtest.WideValue()) {
		t.Fatalf("the whole value is shown at width 50 although it is wider:\n%s", narrow)
	}

	resizeTerminal(m, 200, 30)

	view := m.textOverlay.view()
	inputtest.AssertLinesWithin(t, view, 200)
	if !strings.Contains(view, inputtest.WideValue()) {
		t.Errorf("the whole value fits at width 200 but is not shown:\n%s", view)
	}
}
