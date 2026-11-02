package tui

// The question overlays (select one, text) and the error screen stay inside the
// terminal however many options or how long a message there is, keep every
// existing text, and draw their messages in signal roles: validation and error
// text in the bad role.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/interaction"
	tuicommon "mosaic-common/tui"
	"mosaic-log-analyzer/internal/app"
)

// modelAt returns a model whose terminal has the given size.
func modelAt(width, height int) Model {
	m := newNavModel()
	m.width, m.height = width, height
	return m
}

// selectOverlayModel opens a select-one overlay over n options named opt-01 ...
// and returns the model with the channel the answer is sent on.
func selectOverlayModel(t *testing.T, width, height, n int, label func(i int) string, allowSkip bool) (Model, chan answerMsg) {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("opt-%02d", i+1)
	}
	reply := make(chan answerMsg, 1)
	p := makePendingSelect(app.QuestionPricingAction, ids, reply)
	p.choiceQ.AllowSkip = allowSkip
	if label != nil {
		for i := range p.choiceQ.Options {
			p.choiceQ.Options[i].Label = label(i)
		}
	}
	updated, _ := modelAt(width, height).Update(p)
	m := asModel(updated)
	m.interact.Close()
	if m.screen != screenQuestion || m.selectOverlay == nil {
		t.Fatalf("setup: screen = %q, want the select overlay", m.screen)
	}
	return m, reply
}

// textOverlayModel opens a text overlay for a question with the given texts.
func textOverlayModel(t *testing.T, width, height int, q interaction.TextQuestion) Model {
	t.Helper()
	p := makePendingText(app.QuestionPricingRate, make(chan answerMsg, 1))
	q.Question.ID = app.QuestionPricingRate
	p.textQ = q
	updated, _ := modelAt(width, height).Update(p)
	m := asModel(updated)
	m.interact.Close()
	if m.screen != screenQuestion || m.textOverlay == nil {
		t.Fatalf("setup: screen = %q, want the text overlay", m.screen)
	}
	return m
}

func TestSelectOverlay_ManyOptionsStayWithinTheTerminal(t *testing.T) {
	colourOff(t)
	m, _ := selectOverlayModel(t, 40, 10, 30, nil, true)

	assertViewWithin(t, m.View(), 40, 10)
}

func TestSelectOverlay_CursorOptionStaysVisibleAndMarkedWhileScrolling(t *testing.T) {
	colourOff(t)
	m, _ := selectOverlayModel(t, 40, 10, 30, nil, true)

	for i := 0; i < 25; i++ {
		m = press(m, "j")
		assertViewWithin(t, m.View(), 40, 10)
	}

	line := rawLineWith(t, m.View(), "opt-26")
	if !strings.HasPrefix(escapeRE.ReplaceAllString(line, ""), ">") {
		t.Errorf("the option under the cursor must carry the cursor marker: %q", line)
	}
}

func TestSelectOverlay_SkipEntryIsReachableAtTheEndOfALongListAndAnswersSkipped(t *testing.T) {
	colourOff(t)
	m, reply := selectOverlayModel(t, 40, 8, 12, nil, true)

	for i := 0; i < 12; i++ {
		m = press(m, "j")
	}
	line := rawLineWith(t, m.View(), "Skip")
	if !strings.HasPrefix(escapeRE.ReplaceAllString(line, ""), ">") {
		t.Errorf("the skip entry must carry the cursor marker: %q", line)
	}
	assertViewWithin(t, m.View(), 40, 8)
	pressType(m, tea.KeyEnter)

	select {
	case ans := <-reply:
		if ans.choiceAns.Status != interaction.SkippedOne {
			t.Errorf("status = %q, want skipped", ans.choiceAns.Status)
		}
	default:
		t.Error("enter on the skip entry must answer the question")
	}
}

func TestSelectOverlay_LongLabelsNeverReachPastTheWidthOrHeight(t *testing.T) {
	colourOff(t)
	long := func(i int) string {
		return fmt.Sprintf("option %d %s", i, strings.Repeat("with a very long label ", 6))
	}

	for _, size := range [][2]int{{80, 24}, {40, 12}, {30, 8}, {20, 6}, {12, 4}} {
		m, _ := selectOverlayModel(t, size[0], size[1], 5, long, true)

		assertViewWithin(t, m.View(), size[0], size[1])
	}
}

func TestSelectOverlay_KeepsTitleLabelsSkipAndKeyHelpAsText(t *testing.T) {
	colourOff(t)
	m, _ := selectOverlayModel(t, 80, 24, 3, nil, true)

	view := escapeRE.ReplaceAllString(m.View(), "")

	assertContains(t, view, string(app.QuestionPricingAction), "opt-01", "opt-02", "opt-03", "Skip",
		"enter select", "esc cancel", "ctrl+c quit")
}

func TestTextOverlay_ValidationMessageIsDrawnInTheBadRole(t *testing.T) {
	colourOn(t)
	q := interaction.TextQuestion{Validate: func(string) error { return errors.New("not a number") }}
	m := textOverlayModel(t, 80, 24, q)

	m = pressType(press(m, "x"), tea.KeyEnter)

	assertDrawnInRole(t, rawLineWith(t, m.View(), "not a number"), "not a number", tuicommon.RoleSignalBad)
}

func TestTextOverlay_ValidationMessageReadsAsTextWithColourOff(t *testing.T) {
	colourOff(t)
	q := interaction.TextQuestion{Validate: func(string) error { return errors.New("not a number") }}
	m := textOverlayModel(t, 80, 24, q)

	m = pressType(press(m, "x"), tea.KeyEnter)

	assertContains(t, m.View(), "not a number", "enter confirm", "esc cancel")
	assertNotContains(t, m.View(), "\x1b[")
}

func TestTextOverlay_LongPromptDetailAndMessageStayWithinTheTerminal(t *testing.T) {
	colourOff(t)
	words := strings.Repeat("a very long sentence ", 12)
	q := interaction.TextQuestion{
		Question: interaction.Question{Title: "Input rate", Prompt: words, Detail: words},
		Validate: func(string) error { return errors.New(words) },
	}

	for _, size := range [][2]int{{80, 24}, {40, 12}, {30, 8}, {20, 6}, {12, 4}} {
		m := textOverlayModel(t, size[0], size[1], q)
		m = pressType(press(m, "x"), tea.KeyEnter)

		assertViewWithin(t, m.View(), size[0], size[1])
	}
}

func TestTextOverlay_QuestionMarkIsTypedAsText(t *testing.T) {
	m := textOverlayModel(t, 80, 24, interaction.TextQuestion{})

	m = press(press(press(m, "a"), "?"), "b")

	if got := m.textOverlay.answer().Text; got != "a?b" {
		t.Errorf("overlay text = %q, want a?b", got)
	}
}

func errorModel(width, height int, msg string) Model {
	m := modelAt(width, height)
	m.screen, m.errMsg = screenError, msg
	return m
}

func TestErrorScreen_MessageIsDrawnInTheBadRoleAndKeepsItsText(t *testing.T) {
	colourOn(t)

	view := errorModel(80, 24, "could not read the logs: access denied").View()

	assertDrawnInRole(t, rawLineWith(t, view, "could not read the logs"), "could not read the logs: access denied", tuicommon.RoleSignalBad)
	assertContains(t, escapeRE.ReplaceAllString(view, ""), "Log Analyzer — Error", "Press any key to quit.")
}

func TestErrorScreen_ReadsAsPlainTextWithColourOff(t *testing.T) {
	colourOff(t)

	view := errorModel(80, 24, "could not read the logs").View()

	assertContains(t, view, "Log Analyzer — Error", "could not read the logs", "Press any key to quit.")
	assertNotContains(t, view, "\x1b[")
}

func TestErrorScreen_LongMessageNeverReachesPastTheTerminal(t *testing.T) {
	colourOff(t)
	msg := "The specified path is not a usable logs tree: " + strings.Repeat("deep/", 60)

	for _, size := range [][2]int{{80, 24}, {40, 6}, {20, 4}, {12, 3}} {
		assertViewWithin(t, errorModel(size[0], size[1], msg).View(), size[0], size[1])
	}
}

func TestErrorScreen_LongMessageStillShowsTheTitleAndTheKeyHint(t *testing.T) {
	colourOff(t)
	msg := "The specified path is not a usable logs tree: " + strings.Repeat("deep/", 60)

	view := errorModel(40, 6, msg).View()

	assertContains(t, view, "Log Analyzer — Error", "Press any key to quit.")
}
