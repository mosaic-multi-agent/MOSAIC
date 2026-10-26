package tui

// A multi-line text question from the session is answered through the
// multi-line widget and comes back with its line breaks intact. Enter adds a
// line break and never submits; ctrl+s submits; esc cancels.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/interaction"
	"mosaic-common/tui/pastesafe"
)

// openMultilineQuestion shows a multi-line text question over the progress screen.
func openMultilineQuestion(t *testing.T, q interaction.TextQuestion) (*rootModel, chan answerMsg) {
	t.Helper()
	m := newTestModel()
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	q.Multiline = true
	reply := make(chan answerMsg, 1)
	m.Update(questionMsg{kind: questionAskText, textQ: q, reply: reply})
	if m.screen != screenQuestion {
		t.Fatalf("precondition: question overlay not open, screen = %v", m.screen)
	}
	return m, reply
}

// typeLines types each line, pressing Enter between them.
func typeLines(m *rootModel, lines ...string) {
	for i, l := range lines {
		if i > 0 {
			sendKey(m, tea.KeyEnter)
		}
		sendRunes(m, l)
	}
}

// takeReply returns the reply sent to the session, or fails when none was sent.
func takeTextReply(t *testing.T, reply chan answerMsg) answerMsg {
	t.Helper()
	select {
	case ans := <-reply:
		return ans
	default:
		t.Fatal("no reply was sent to the waiting session")
		return answerMsg{}
	}
}

func assertNoTextReply(t *testing.T, reply chan answerMsg, why string) {
	t.Helper()
	select {
	case ans := <-reply:
		t.Fatalf("a reply was sent %s: %+v", why, ans.textAns)
	default:
	}
}

func TestMultilineQuestion_EnterAddsLineBreakAndCtrlSSubmitsTextIntact(t *testing.T) {
	m, reply := openMultilineQuestion(t, interaction.TextQuestion{Question: interaction.Question{Title: "Task guidance"}})

	typeLines(m, "first", "", "third")
	assertNoTextReply(t, reply, "while Enter was only adding line breaks")
	sendKey(m, tea.KeyCtrlS)

	ans := takeTextReply(t, reply).textAns
	if ans.Status != interaction.Answered {
		t.Fatalf("Status = %q, want %q", ans.Status, interaction.Answered)
	}
	if want := "first\n\nthird"; ans.Text != want {
		t.Errorf("Text = %q, want %q", ans.Text, want)
	}
	if m.screen != screenProgress || m.textOverlay != nil {
		t.Errorf("overlay not closed after ctrl+s: screen = %v", m.screen)
	}
}

func TestMultilineQuestion_PrefillsDefaultAndReturnsItUnchanged(t *testing.T) {
	q := interaction.TextQuestion{
		Question: interaction.Question{Title: "Task guidance"},
		Default:  "para one\nstill one\n\npara two",
	}
	m, reply := openMultilineQuestion(t, q)

	sendKey(m, tea.KeyCtrlS)

	if got := takeTextReply(t, reply).textAns.Text; got != q.Default {
		t.Errorf("Text = %q, want the default %q", got, q.Default)
	}
}

func TestMultilineQuestion_TypingAfterDefaultExtendsIt(t *testing.T) {
	q := interaction.TextQuestion{Question: interaction.Question{Title: "t"}, Default: "old"}
	m, reply := openMultilineQuestion(t, q)

	typeLines(m, "", "new")
	sendKey(m, tea.KeyCtrlS)

	if got, want := takeTextReply(t, reply).textAns.Text, "old\nnew"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}

func TestMultilineQuestion_LongPasteIsNeverTruncated(t *testing.T) {
	m, reply := openMultilineQuestion(t, interaction.TextQuestion{Question: interaction.Question{Title: "t"}})
	lines := make([]string, 150)
	for i := range lines {
		lines[i] = "line number " + strings.Repeat("x", i%7)
	}

	typeLines(m, lines...)
	sendKey(m, tea.KeyCtrlS)

	if got, want := takeTextReply(t, reply).textAns.Text, strings.Join(lines, "\n"); got != want {
		t.Errorf("a 150-line entry came back altered: got %d bytes, want %d", len(got), len(want))
	}
}

func TestMultilineQuestion_EscCancels(t *testing.T) {
	m, reply := openMultilineQuestion(t, interaction.TextQuestion{Question: interaction.Question{Title: "t"}})
	typeLines(m, "abc", "def")

	sendKey(m, tea.KeyEsc)

	if got := takeTextReply(t, reply).textAns.Status; got != interaction.Cancelled {
		t.Errorf("Status = %q, want %q", got, interaction.Cancelled)
	}
	if m.textOverlay != nil || m.screen != screenProgress {
		t.Errorf("overlay not closed after esc: screen = %v", m.screen)
	}
}

func TestMultilineQuestion_ValidateBlocksSubmitAndShowsError(t *testing.T) {
	q := interaction.TextQuestion{
		Question: interaction.Question{Title: "t"},
		Validate: func(s string) error {
			if !strings.Contains(s, "ok") {
				return errors.New("needs the magic word")
			}
			return nil
		},
	}
	m, reply := openMultilineQuestion(t, q)
	sendRunes(m, "no")

	sendKey(m, tea.KeyCtrlS)

	assertNoTextReply(t, reply, "for text the validator rejects")
	if view := m.View(); !strings.Contains(view, "needs the magic word") {
		t.Errorf("view lacks the validation error:\n%s", view)
	}
	sendRunes(m, "ok")
	sendKey(m, tea.KeyCtrlS)
	if got, want := takeTextReply(t, reply).textAns.Text, "nook"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}

func TestMultilineQuestion_ShowsTitleAndDetail(t *testing.T) {
	q := interaction.TextQuestion{Question: interaction.Question{Title: "Task guidance title", Detail: "context line"}}
	m, _ := openMultilineQuestion(t, q)

	view := m.View()

	for _, want := range []string{"Task guidance title", "context line"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

// A pasted block, with Unix or Windows line endings, arrives as one multi-line
// entry and is returned with its line breaks as single newlines.
func TestMultilineQuestion_PastedBlockKeepsItsLineBreaks(t *testing.T) {
	for name, pasted := range map[string]string{
		"unix line endings":    "alpha\nbeta\n\ngamma",
		"windows line endings": "alpha\r\nbeta\r\n\r\ngamma",
	} {
		t.Run(name, func(t *testing.T) {
			m, reply := openMultilineQuestion(t, interaction.TextQuestion{Question: interaction.Question{Title: "t"}})

			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(pasted), Paste: true})
			assertNoTextReply(t, reply, "while pasting")
			sendKey(m, tea.KeyCtrlS)

			if got, want := takeTextReply(t, reply).textAns.Text, "alpha\nbeta\n\ngamma"; got != want {
				t.Errorf("Text = %q, want %q", got, want)
			}
		})
	}
}

// A single-line question still collapses pasted line breaks to spaces.
func TestSingleLineQuestion_PastedLineBreaksStillCollapse(t *testing.T) {
	restore := pastesafe.SetClock(burstClock())
	defer restore()
	m, reply := openTextQuestion(t)

	sendRunes(m, "first")
	sendKey(m, tea.KeyEnter)
	sendRunes(m, "second")

	assertNoTextReply(t, reply, "while pasting")
	if got := m.textOverlay.answer().Text; got != "first second" {
		t.Errorf("pasted text = %q, want %q", got, "first second")
	}
}

// A single-line question keeps submitting on Enter; the multi-line widget is
// used only when the question asks for it.
func TestSingleLineQuestion_EnterStillSubmitsAndCtrlSDoesNot(t *testing.T) {
	m, reply := openTextQuestion(t)
	sendRunes(m, "abc")

	sendKey(m, tea.KeyCtrlS)
	assertNoTextReply(t, reply, "on ctrl+s in a single-line question")
	sendKey(m, tea.KeyEnter)

	ans := takeTextReply(t, reply).textAns
	if ans.Status != interaction.Answered || ans.Text != "abc" {
		t.Errorf("answer = %+v, want Answered %q", ans, "abc")
	}
}
