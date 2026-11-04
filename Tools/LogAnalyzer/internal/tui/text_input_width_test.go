package tui

// text_input_width_test.go verifies that the single-line inputs of the
// LogAnalyzer TUI keep a value wider than the terminal inside the terminal
// width with the cursor and the text around it visible, and that they follow a
// terminal resize in both directions.

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// wideInput has 62 distinct characters, so a visible slice identifies its position.
const wideInput = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var inputAnsiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func resizeModel(m Model, width, height int) Model {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return asModel(updated)
}

// typeSlowly types text one rune at a time, each after a typing gap, so no key is read as a paste.
func typeSlowly(clk *steppedClock, m Model, text string) Model {
	for _, r := range text {
		m = pressAfterTypingGap(clk, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func assertInputWithin(t *testing.T, view string, width int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
}

func assertInputShows(t *testing.T, view string, parts ...string) {
	t.Helper()
	plain := inputAnsiRE.ReplaceAllString(view, "")
	for _, p := range parts {
		if !strings.Contains(plain, p) {
			t.Fatalf("view does not show %q:\n%s", p, plain)
		}
	}
}

func assertInputHides(t *testing.T, view string, parts ...string) {
	t.Helper()
	plain := inputAnsiRE.ReplaceAllString(view, "")
	for _, p := range parts {
		if strings.Contains(plain, p) {
			t.Fatalf("view must not show %q:\n%s", p, plain)
		}
	}
}

func TestTextOverlay_WideValueStaysWithinWidthAndShowsTail(t *testing.T) {
	clk := installSteppedClock(t)
	m, _ := openTextOverlay(t, nil)
	m = resizeModel(m, 30, 24)

	m = typeSlowly(clk, m, wideInput)

	view := m.View()
	assertInputWithin(t, view, 30)
	assertInputShows(t, view, wideInput[len(wideInput)-5:])
	assertInputHides(t, view, wideInput[:5])
}

func TestTextOverlay_CursorMovedToStartShowsHeadNotTail(t *testing.T) {
	clk := installSteppedClock(t)
	m, _ := openTextOverlay(t, nil)
	m = resizeModel(m, 30, 24)
	m = typeSlowly(clk, m, wideInput)

	m = pressAfterTypingGap(clk, m, tea.KeyMsg{Type: tea.KeyHome})

	view := m.View()
	assertInputWithin(t, view, 30)
	assertInputShows(t, view, wideInput[:5])
	assertInputHides(t, view, wideInput[len(wideInput)-5:])
}

func TestTextOverlay_ShrinkingTerminalKeepsCursorTextVisible(t *testing.T) {
	clk := installSteppedClock(t)
	m, _ := openTextOverlay(t, nil)
	m = typeSlowly(clk, m, wideInput[:40])

	m = resizeModel(m, 30, 24)

	view := m.View()
	assertInputWithin(t, view, 30)
	assertInputShows(t, view, wideInput[35:40])
}

func TestTextOverlay_GrowingTerminalShowsMoreOfTheValue(t *testing.T) {
	clk := installSteppedClock(t)
	m, _ := openTextOverlay(t, nil)
	m = resizeModel(m, 30, 24)
	m = typeSlowly(clk, m, wideInput)
	assertInputHides(t, m.View(), wideInput[:5])

	m = resizeModel(m, 100, 24)

	view := m.View()
	assertInputWithin(t, view, 100)
	assertInputShows(t, view, wideInput)
}

func TestTextOverlay_ResizeKeepsTheTypedAnswer(t *testing.T) {
	clk := installSteppedClock(t)
	m, _ := openTextOverlay(t, nil)
	m = typeSlowly(clk, m, wideInput)

	m = resizeModel(m, 25, 24)
	m = resizeModel(m, 90, 24)

	if got := m.textOverlay.answer().Text; got != wideInput {
		t.Errorf("answer text = %q, want %q", got, wideInput)
	}
}

func TestSourceScreen_WideValueFollowsTerminalResize(t *testing.T) {
	clk := installSteppedClock(t)
	m, _ := openSourceScreen(t)
	m = typeSlowly(clk, m, wideInput)

	m = resizeModel(m, 30, 24)

	view := m.View()
	assertInputWithin(t, view, 30)
	assertInputShows(t, view, wideInput[len(wideInput)-5:])
	assertInputHides(t, view, wideInput[:5])
}
