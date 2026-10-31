// Package inputtest holds helpers shared by the tests of Runner's single-line
// input screens: a value wider than any test terminal, typing it into a
// screen, and the width checks on a rendered view.
package inputtest

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tail is the end of WideValue. A view that shows it shows the text next to
// the cursor when the cursor sits at the end of the value.
const Tail = "TAILMARK"

// WideValue returns a single-line value wider than every width the tests use.
func WideValue() string {
	return strings.Repeat("abcdefghij", 15) + Tail
}

// Type delivers text to update one rune at a time, as a keyboard would.
func Type(update func(tea.Msg) tea.Cmd, text string) {
	for _, r := range text {
		update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// AssertTailLineWithin fails unless view has a line showing Tail and that line
// is at most width cells wide.
func AssertTailLineWithin(t *testing.T, view string, width int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, Tail) {
			continue
		}
		if w := lipgloss.Width(line); w > width {
			t.Errorf("input line is %d cells wide, want at most %d: %q", w, width, line)
		}
		return
	}
	t.Errorf("no line shows the end of the value (cursor not visible) at width %d:\n%s", width, view)
}

// AssertLinesWithin fails when any line of view is wider than width cells.
func AssertLinesWithin(t *testing.T, view string, width int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line is %d cells wide, want at most %d: %q", w, width, line)
		}
	}
}
