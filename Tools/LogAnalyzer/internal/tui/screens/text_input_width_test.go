package screens_test

// The path inputs of the source and export screens keep a value wider than the
// screen inside the screen width with the cursor and its surrounding text
// visible, and follow the width the screen is drawn at.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-log-analyzer/internal/tui/screens"
)

// wideEntry has 62 distinct characters, so a visible slice identifies its position.
const wideEntry = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func typeEntry(s screens.Screen, text string) screens.Screen {
	for _, r := range text {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return s
}

func assertEntryWithin(t *testing.T, view string, width int) {
	t.Helper()
	for _, line := range viewLines(view) {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
}

func assertEntryShows(t *testing.T, view string, parts ...string) {
	t.Helper()
	plain := stripStyling(view)
	for _, p := range parts {
		if !strings.Contains(plain, p) {
			t.Fatalf("view does not show %q:\n%s", p, plain)
		}
	}
}

func assertEntryHides(t *testing.T, view string, parts ...string) {
	t.Helper()
	plain := stripStyling(view)
	for _, p := range parts {
		if strings.Contains(plain, p) {
			t.Fatalf("view must not show %q:\n%s", p, plain)
		}
	}
}

func TestSourceScreen_WideValueFitsNarrowedWidthWithCursorVisible(t *testing.T) {
	colourOff(t)
	var s screens.Screen = newSource(100, 24)
	s = typeEntry(s, wideEntry)

	view := s.View(30, 24)

	assertEntryWithin(t, view, 30)
	assertEntryShows(t, view, wideEntry[len(wideEntry)-5:])
	assertEntryHides(t, view, wideEntry[:5])
}

func TestSourceScreen_WidthGrowingShowsMoreOfTheValue(t *testing.T) {
	colourOff(t)
	var s screens.Screen = newSource(30, 24)
	s = typeEntry(s, wideEntry)
	assertEntryHides(t, s.View(30, 24), wideEntry[:5])

	view := s.View(120, 24)

	assertEntryWithin(t, view, 120)
	assertEntryShows(t, view, wideEntry)
}

func TestExportScreen_WideValueFitsNarrowedWidthWithCursorVisible(t *testing.T) {
	colourOff(t)
	var s screens.Screen = newExport("", 100, 24)
	s = typeEntry(s, wideEntry)

	view := s.View(30, 24)

	assertEntryWithin(t, view, 30)
	assertEntryShows(t, view, wideEntry[len(wideEntry)-5:])
}

func TestExportScreen_WidthGrowingShowsMoreOfTheValue(t *testing.T) {
	colourOff(t)
	var s screens.Screen = newExport("", 30, 24)
	s = typeEntry(s, wideEntry)

	view := s.View(140, 24)

	assertEntryWithin(t, view, 140)
	assertEntryShows(t, view, wideEntry)
}
