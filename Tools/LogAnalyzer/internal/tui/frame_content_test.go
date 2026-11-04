package tui

// Content checks for framed per-run views that ignore styling and spacing.

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/tui/screens"
)

var escapeRE = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// frameLines splits a rendered view into styling-free lines.
func frameLines(view string) []string {
	if view == "" {
		return nil
	}
	return strings.Split(escapeRE.ReplaceAllString(view, ""), "\n")
}

// helpOverlayText presses the help key on the screen and returns the lines
// between the header/tab bar and the help line of the rendered overlay.
func helpOverlayText(s screens.Screen, width, height int) string {
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(screens.KeyHelp)})
	lines := frameLines(s.View(width, height))
	if len(lines) < 3 {
		return ""
	}
	return strings.Join(lines[2:len(lines)-1], "\n")
}

// assertListsKey fails when no entry of the overlay text starts with the key.
func assertListsKey(t *testing.T, overlay, key string) {
	t.Helper()
	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(key) + `\s+\S`).MatchString(overlay) {
		t.Errorf("help overlay must list key %q\n%s", key, overlay)
	}
}
