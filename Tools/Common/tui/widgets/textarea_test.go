package widgets_test

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/tui/widgets"
)

func newTestTextArea() *widgets.TextArea {
	ta := widgets.NewTextArea("Guidance", 40, 6, widgets.DefaultTextAreaStyles())
	ta.Focus()
	return ta
}

func typeRunes(ta *widgets.TextArea, s string) {
	for _, r := range s {
		ta.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func pressEnter(ta *widgets.TextArea) { ta.Update(tea.KeyMsg{Type: tea.KeyEnter}) }

func pressCtrlS(ta *widgets.TextArea) { ta.Update(tea.KeyMsg{Type: tea.KeyCtrlS}) }

func paste(ta *widgets.TextArea, s string) {
	ta.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
}

func TestTextAreaTypedLinesAndParagraphsReturnedIntact(t *testing.T) {
	ta := newTestTextArea()

	typeRunes(ta, "first")
	pressEnter(ta)
	typeRunes(ta, "second")
	pressEnter(ta)
	pressEnter(ta)
	typeRunes(ta, "new paragraph")

	if got, want := ta.Value(), "first\nsecond\n\nnew paragraph"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
	if ta.Done() {
		t.Fatal("Enter must never submit")
	}
}

func TestTextAreaEnterAtEndKeepsTrailingLineBreak(t *testing.T) {
	ta := newTestTextArea()

	typeRunes(ta, "a")
	pressEnter(ta)

	if got, want := ta.Value(), "a\n"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaPasteFlaggedMultiLineKeptAndDoesNotSubmit(t *testing.T) {
	ta := newTestTextArea()

	paste(ta, "one\ntwo\n\nthree")

	if got, want := ta.Value(), "one\ntwo\n\nthree"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
	if ta.Done() {
		t.Fatal("a paste must not submit")
	}
}

func TestTextAreaPasteDeliveredAsKeyEventsNeedsExplicitSubmit(t *testing.T) {
	ta := newTestTextArea()
	const text = "alpha\nbeta\n\ngamma"

	// Windows delivers a paste as one key event per character, line breaks as Enter.
	for _, r := range text {
		if r == '\n' {
			pressEnter(ta)
		} else {
			ta.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		if ta.Done() {
			t.Fatal("pasted line breaks must not submit")
		}
	}
	if got := ta.Value(); got != text {
		t.Fatalf("value = %q, want %q", got, text)
	}

	pressCtrlS(ta)

	if !ta.Done() {
		t.Fatal("submit still needs the explicit key")
	}
	if got := ta.Value(); got != text {
		t.Fatalf("submitted value = %q, want %q", got, text)
	}
}

func TestTextAreaPasteThenCtrlSReturnsPastedTextIntact(t *testing.T) {
	ta := newTestTextArea()
	// Leading/trailing blank lines, multi-byte runes and a tab.
	text := "\n\nlead\néè 日本\tTabbed\ntail\n\n"

	paste(ta, text)
	pressCtrlS(ta)

	if !ta.Done() {
		t.Fatal("ctrl+s must submit after a paste")
	}
	if got := ta.Value(); got != text {
		t.Fatalf("value = %q, want %q", got, text)
	}
}

func TestTextAreaPasteWithCRLFAndLoneCRNormalisedToLF(t *testing.T) {
	ta := newTestTextArea()

	paste(ta, "a\r\nb\rc")

	if got, want := ta.Value(), "a\nb\nc"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaLongPasteOverHundredLinesNotTruncated(t *testing.T) {
	ta := newTestTextArea()
	lines := make([]string, 250)
	for i := range lines {
		lines[i] = strings.Repeat("x", 120)
	}
	text := strings.Join(lines, "\n")

	paste(ta, text)

	if got := ta.Value(); got != text {
		t.Fatalf("long paste altered: got %d bytes, want %d bytes", len(got), len(text))
	}
}

func TestTextAreaInitialValueReturnedAndNormalised(t *testing.T) {
	ta := widgets.NewTextArea("L", 40, 6, widgets.DefaultTextAreaStyles())

	ta.SetValue("keep\r\nthis\n\npara")

	if got, want := ta.Value(), "keep\nthis\n\npara"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaInitialValueShownAndEditable(t *testing.T) {
	ta := widgets.NewTextArea("L", 40, 6, widgets.DefaultTextAreaStyles())
	ta.SetValue("seed text")
	ta.Focus()

	if !strings.Contains(ta.View(), "seed text") {
		t.Fatalf("view must show the initial value, got:\n%s", ta.View())
	}

	typeRunes(ta, "!")
	pressEnter(ta)
	typeRunes(ta, "more")

	// SetValue leaves the cursor at the end of the text.
	if got, want := ta.Value(), "seed text!\nmore"; got != want {
		t.Fatalf("typing must edit the initial value: got %q, want %q", got, want)
	}
}

func TestTextAreaCtrlSSubmits(t *testing.T) {
	ta := newTestTextArea()
	typeRunes(ta, "hello")
	pressEnter(ta)
	typeRunes(ta, "world")

	pressCtrlS(ta)

	if !ta.Done() {
		t.Fatal("ctrl+s must submit")
	}
	if ta.Back() {
		t.Fatal("ctrl+s must not report Back")
	}
	if got, want := ta.Value(), "hello\nworld"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaCtrlSWithoutValidatorSubmitsEmptyText(t *testing.T) {
	ta := newTestTextArea()

	pressCtrlS(ta)

	if !ta.Done() {
		t.Fatal("ctrl+s without a validator must submit")
	}
	if got := ta.Value(); got != "" {
		t.Fatalf("value = %q, want empty", got)
	}
}

func TestTextAreaEscCancels(t *testing.T) {
	ta := newTestTextArea()
	typeRunes(ta, "draft")

	ta.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !ta.Back() {
		t.Fatal("esc must set Back")
	}
	if ta.Done() {
		t.Fatal("esc must not submit")
	}
}

// Guard: keys that could submit or cancel by accident must not. The text
// assertion after Enter is the part that requires the widget behaviour.
func TestTextAreaStrayKeysDoNotSubmitOrCancel(t *testing.T) {
	first := newTestTextArea()
	typeRunes(first, "text")
	pressEnter(first)
	if got, want := first.Value(), "text\n"; got != want {
		t.Fatalf("Enter must insert a line break: got %q, want %q", got, want)
	}

	keys := map[string]tea.KeyMsg{
		"enter":          {Type: tea.KeyEnter},
		"alt+enter":      {Type: tea.KeyEnter, Alt: true},
		"ctrl+j":         {Type: tea.KeyCtrlJ},
		"ctrl+m":         {Type: tea.KeyCtrlM},
		"ctrl+d":         {Type: tea.KeyCtrlD},
		"ctrl+c":         {Type: tea.KeyCtrlC},
		"tab":            {Type: tea.KeyTab},
		"backspace":      {Type: tea.KeyBackspace},
		"left":           {Type: tea.KeyLeft},
		"runes":          {Type: tea.KeyRunes, Runes: []rune("x")},
		"pasted ctrl+s":  {Type: tea.KeyCtrlS, Paste: true},
		"pasted enter":   {Type: tea.KeyEnter, Paste: true},
		"pasted newline": {Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true},
	}
	for name, k := range keys {
		ta := newTestTextArea()
		typeRunes(ta, "text")

		ta.Update(k)

		if ta.Done() || ta.Back() {
			t.Errorf("%s changed state (done=%v back=%v)", name, ta.Done(), ta.Back())
		}
	}
}

func TestTextAreaValidatorBlocksInvalidSubmitAndShowsError(t *testing.T) {
	ta := newTestTextArea()
	ta.SetValidate(func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("required")
		}
		return nil
	})

	pressCtrlS(ta)
	if ta.Done() || ta.ErrMsg() == "" {
		t.Fatalf("empty text must set ErrMsg and not Done (done=%v err=%q)", ta.Done(), ta.ErrMsg())
	}
	if !strings.Contains(ta.View(), "required") {
		t.Fatal("view must show the validation error")
	}

	typeRunes(ta, "x")
	pressCtrlS(ta)
	if !ta.Done() || ta.ErrMsg() != "" {
		t.Fatalf("valid text must set Done and clear ErrMsg (done=%v err=%q)", ta.Done(), ta.ErrMsg())
	}
}

func TestTextAreaEscAfterFailedValidationCancels(t *testing.T) {
	ta := newTestTextArea()
	ta.SetValidate(func(string) error { return errors.New("always invalid") })
	pressCtrlS(ta)

	ta.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !ta.Back() || ta.Done() {
		t.Fatalf("esc must cancel regardless of validation (back=%v done=%v)", ta.Back(), ta.Done())
	}
}

func TestTextAreaBlurredIgnoresKeysUntilFocused(t *testing.T) {
	ta := newTestTextArea()
	typeRunes(ta, "a")

	ta.Blur()
	typeRunes(ta, "b")
	ta.Focus()
	typeRunes(ta, "c")

	if got, want := ta.Value(), "ac"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaPasteInsertedAtCursorMidText(t *testing.T) {
	ta := newTestTextArea()
	typeRunes(ta, "xy")
	ta.Update(tea.KeyMsg{Type: tea.KeyLeft})

	paste(ta, "A\nB")

	if got, want := ta.Value(), "xA\nBy"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaUpMovesToPreviousLineForEditing(t *testing.T) {
	ta := newTestTextArea()
	paste(ta, "ab\ncd")

	ta.Update(tea.KeyMsg{Type: tea.KeyUp})
	typeRunes(ta, "!")

	if got, want := ta.Value(), "ab!\ncd"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaBackspaceAtLineStartJoinsLines(t *testing.T) {
	ta := newTestTextArea()
	paste(ta, "ab\ncd")
	ta.Update(tea.KeyMsg{Type: tea.KeyLeft})
	ta.Update(tea.KeyMsg{Type: tea.KeyLeft})

	ta.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	if got, want := ta.Value(), "abcd"; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func TestTextAreaResetClearsFlagsKeepsText(t *testing.T) {
	ta := newTestTextArea()
	typeRunes(ta, "keep")
	pressCtrlS(ta)
	if !ta.Done() {
		t.Fatal("precondition: ctrl+s must submit")
	}

	ta.Reset()

	if ta.Done() || ta.Back() || ta.ErrMsg() != "" {
		t.Fatal("Reset must clear Done, Back and ErrMsg")
	}
	if got := ta.Value(); got != "keep" {
		t.Fatalf("value = %q, want %q", got, "keep")
	}
}

func TestTextAreaResizeDoesNotLoseText(t *testing.T) {
	ta := newTestTextArea()
	text := "line one\nline two is a bit longer than the narrow width\n\nthird"
	paste(ta, text)

	ta.Resize(10, 2)
	if got := ta.Value(); got != text {
		t.Fatalf("after shrinking value = %q, want %q", got, text)
	}
	if ta.View() == "" {
		t.Fatal("view must still render after shrinking")
	}

	ta.Resize(80, 20)
	if got := ta.Value(); got != text {
		t.Fatalf("after growing value = %q, want %q", got, text)
	}

	typeRunes(ta, "!")
	if got, want := ta.Value(), text+"!"; got != want {
		t.Fatalf("typing after resize: value = %q, want %q", got, want)
	}
}

func TestTextAreaViewShowsLabelAndKeyHints(t *testing.T) {
	ta := newTestTextArea()

	v := ta.View()

	if !strings.Contains(v, "Guidance") {
		t.Fatalf("view must contain the label, got:\n%s", v)
	}
	if !strings.Contains(v, "ctrl+s") || !strings.Contains(v, "esc") {
		t.Fatalf("view must contain the key hints, got:\n%s", v)
	}
}
