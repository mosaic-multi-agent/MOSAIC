package screens_test

// draft_width_test.go verifies that the screens built on the draft entry keep a
// value wider than the screen inside the screen width with the cursor and the
// text around it visible, and that they follow the screen's Resize.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-agent-test/internal/tui/screens"
)

// draftScreen is the surface the draft-entry screens share.
type draftScreen interface {
	Update(msg tea.Msg) tea.Cmd
	View() string
	Resize(width int)
}

// entryCase builds a screen at a width and supplies a distinct-character value
// that the screen's entry accepts.
type entryCase struct {
	name  string
	build func(width int) draftScreen
	value string
}

// digitValue is 45 digits in runs of five, so a visible slice identifies its position.
const digitValue = "111112222233333444445555566666777778888899999"

const textValue = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func entryCases() []entryCase {
	st := plainStyles()
	return []entryCase{
		{"SummaryInput", func(w int) draftScreen { return screens.NewSummaryInputScreen("", w, st) }, textValue},
		{"StoreInput", func(w int) draftScreen { return screens.NewStoreInputScreen("", w, st) }, textValue},
		{"ReportPath", func(w int) draftScreen { return screens.NewReportPathScreen("", w, st) }, textValue},
		{"CatalogFolder", func(w int) draftScreen { return screens.NewCatalogFolderScreen("", w, st) }, textValue},
		{"Repetitions", func(w int) draftScreen { return screens.NewRepetitionsScreen(1, w, st) }, digitValue},
		{"MaxConcurrentRuns", func(w int) draftScreen { return screens.NewMaxConcurrentRunsScreen(1, w, st) }, digitValue},
	}
}

func typeInto(s draftScreen, text string) {
	for _, r := range text {
		s.Update(runeKey(r))
	}
}

func assertLinesWithin(t *testing.T, view string, width int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
}

func assertScreenShows(t *testing.T, view string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(view, p) {
			t.Fatalf("view does not show %q:\n%s", p, view)
		}
	}
}

func assertScreenHides(t *testing.T, view string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if strings.Contains(view, p) {
			t.Fatalf("view must not show %q:\n%s", p, view)
		}
	}
}

func TestDraftScreens_WideValueAtConstructionStaysWithinWidthAndShowsTail(t *testing.T) {
	for _, c := range entryCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.build(30)
			typeInto(s, c.value)

			view := s.View()

			assertLinesWithin(t, view, 30)
			assertScreenShows(t, view, c.value[len(c.value)-5:])
			assertScreenHides(t, view, c.value[:5])
		})
	}
}

func TestDraftScreens_CursorMovedToStartShowsHeadNotTail(t *testing.T) {
	for _, c := range entryCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.build(30)
			typeInto(s, c.value)

			s.Update(tea.KeyMsg{Type: tea.KeyHome})

			view := s.View()
			assertLinesWithin(t, view, 30)
			assertScreenShows(t, view, c.value[:5])
			assertScreenHides(t, view, c.value[len(c.value)-5:])
		})
	}
}

func TestDraftScreens_CursorInMiddleShowsTextOnBothSides(t *testing.T) {
	for _, c := range entryCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.build(30)
			typeInto(s, c.value)
			s.Update(tea.KeyMsg{Type: tea.KeyHome})
			for i := 0; i < 22; i++ {
				s.Update(tea.KeyMsg{Type: tea.KeyRight})
			}

			view := s.View()

			assertLinesWithin(t, view, 30)
			assertScreenShows(t, view, c.value[18:26])
		})
	}
}

func TestDraftScreens_ResizeNarrowerKeepsViewWithinNewWidthAndCursorVisible(t *testing.T) {
	for _, c := range entryCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.build(100)
			typeInto(s, c.value)
			assertScreenShows(t, s.View(), c.value)

			s.Resize(30)

			view := s.View()
			assertLinesWithin(t, view, 30)
			assertScreenShows(t, view, c.value[len(c.value)-5:])
			assertScreenHides(t, view, c.value[:5])
		})
	}
}

func TestDraftScreens_ResizeWiderShowsMoreOfTheValue(t *testing.T) {
	for _, c := range entryCases() {
		t.Run(c.name, func(t *testing.T) {
			s := c.build(30)
			typeInto(s, c.value)
			assertScreenHides(t, s.View(), c.value[:5])

			s.Resize(100)

			view := s.View()
			assertLinesWithin(t, view, 100)
			assertScreenShows(t, view, c.value)
		})
	}
}
