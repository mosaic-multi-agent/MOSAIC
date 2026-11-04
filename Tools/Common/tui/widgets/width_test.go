package widgets_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui/widgets"
)

// wideText has 62 distinct characters, so any visible slice identifies its position.
const wideText = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func assertLinesWithin(t *testing.T, view string, width int) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", i, got, width, line)
		}
	}
}

func assertViewShows(t *testing.T, view string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(view, p) {
			t.Fatalf("view %q does not show %q", view, p)
		}
	}
}

// inputLine returns the field line of a TextInput view (the line after the label).
func inputLine(t *testing.T, in *widgets.TextInput) string {
	t.Helper()
	lines := strings.Split(in.View(), "\n")
	if len(lines) < 2 {
		t.Fatalf("view has %d lines, want label and input line", len(lines))
	}
	return lines[1]
}

func TestTextInputFieldLineStaysWithinConstructionWidth(t *testing.T) {
	in := widgets.NewTextInput("Name", "", 20, widgets.DefaultTextInputStyles())
	in.SetValue(wideText)

	line := inputLine(t, in)

	assertLinesWithin(t, line, 20)
	assertViewShows(t, line, wideText[len(wideText)-5:])
}

func TestTextInputFieldLineStaysWithinConstructionWidthWithoutLabel(t *testing.T) {
	in := widgets.NewTextInput("", "", 20, widgets.DefaultTextInputStyles())
	in.SetValue(wideText)

	view := in.View()

	assertLinesWithin(t, view, 20)
	assertViewShows(t, view, wideText[len(wideText)-5:])
}

func TestTextInputResizeNarrowerBoundsFieldLine(t *testing.T) {
	in := widgets.NewTextInput("Name", "", 40, widgets.DefaultTextInputStyles())
	in.SetValue(wideText)

	in.Resize(14)

	line := inputLine(t, in)
	assertLinesWithin(t, line, 14)
	assertViewShows(t, line, wideText[len(wideText)-5:])
}

func TestTextInputResizeWiderShowsMoreOfValue(t *testing.T) {
	in := widgets.NewTextInput("Name", "", 14, widgets.DefaultTextInputStyles())
	in.SetValue(wideText)

	in.Resize(100)

	line := inputLine(t, in)
	assertLinesWithin(t, line, 100)
	assertViewShows(t, line, wideText)
}

func TestTextInputLongPlaceholderStaysWithinWidth(t *testing.T) {
	in := widgets.NewTextInput("Name", "a very long placeholder that cannot fit", 16, widgets.DefaultTextInputStyles())

	assertLinesWithin(t, inputLine(t, in), 16)
}

func TestTextInputResizeKeepsValue(t *testing.T) {
	in := widgets.NewTextInput("Name", "", 40, widgets.DefaultTextInputStyles())
	in.SetValue(wideText)

	in.Resize(10)

	if got := in.Value(); got != wideText {
		t.Fatalf("value changed by resize: %q", got)
	}
}

// openWideAddEntry opens the add-entry field and types wideText into it.
func openWideAddEntry(t *testing.T) *widgets.MultiSelect {
	t.Helper()
	m := newCustomSelect(t)
	for i := 0; i < 6; i++ {
		press(m, tea.KeyDown)
	}
	press(m, tea.KeySpace)
	if !m.Editing() {
		t.Fatal("add-entry field did not open")
	}
	typeText(m, wideText)
	return m
}

const addEntryRows = 3 // items a and b plus the add-entry row

func assertRowsWithin(t *testing.T, m *widgets.MultiSelect, width int) string {
	t.Helper()
	view := m.View()
	if got := len(strings.Split(view, "\n")); got != addEntryRows {
		t.Fatalf("view has %d lines, want one per row (%d): %q", got, addEntryRows, view)
	}
	assertLinesWithin(t, view, width)
	return view
}

func TestMultiSelectAddEntryRowStaysWithinWidthWithCursorVisible(t *testing.T) {
	m := openWideAddEntry(t)

	view := assertRowsWithin(t, m, 40)

	assertViewShows(t, view, wideText[len(wideText)-5:])
}

func TestMultiSelectResizeNarrowerWhileAddEntryOpenKeepsRowWithinNewWidth(t *testing.T) {
	m := openWideAddEntry(t)

	m.Resize(10, 26)

	if !m.Editing() {
		t.Fatal("resize closed the add-entry field")
	}
	view := assertRowsWithin(t, m, 26)
	assertViewShows(t, view, wideText[len(wideText)-5:])
}

func TestMultiSelectResizeWiderWhileAddEntryOpenShowsMoreOfValue(t *testing.T) {
	m := openWideAddEntry(t)
	m.Resize(10, 26)

	m.Resize(10, 120)

	view := assertRowsWithin(t, m, 120)
	assertViewShows(t, view, wideText)
}

func TestMultiSelectResizeKeepsTypedAddEntryValue(t *testing.T) {
	m := openWideAddEntry(t)

	m.Resize(10, 26)
	press(m, tea.KeyEnter)

	got := m.CustomEntries()
	if len(got) != 1 || got[0] != wideText {
		t.Fatalf("custom entries = %v, want [%q]", got, wideText)
	}
}
