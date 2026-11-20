package screens_test

// final_screens_keys_test.go verifies the final-screen contract: on every final screen a key
// dismisses the screen (the root model then returns to the first screen), except the scroll
// keys of the scrolling run summary; and each help line says what the keys do.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/tui/screens"
)

// finalScreen is the part of a final screen the contract covers.
type finalScreen interface {
	Update(msg tea.Msg) bool
	Done() bool
	View() string
}

func newFinalScreens() map[string]func() finalScreen {
	return map[string]func() finalScreen{
		"run summary": func() finalScreen {
			return screens.NewSummaryScreen(cleanSummary(), 120, 40, plainStyles())
		},
		"promote summary": func() finalScreen {
			return screens.NewPromoteSummaryScreen(basicPromoteResult(), 120, 40, plainStyles())
		},
		"transform summary": func() finalScreen {
			return screens.NewTransformSummaryScreen(tsTransformedResult(), 120, 40, plainStyles())
		},
	}
}

func dismissKeys() map[string]tea.KeyMsg {
	return map[string]tea.KeyMsg{
		"q":     {Type: tea.KeyRunes, Runes: []rune{'q'}},
		"x":     {Type: tea.KeyRunes, Runes: []rune{'x'}},
		"enter": {Type: tea.KeyEnter},
		"esc":   {Type: tea.KeyEsc},
		"space": {Type: tea.KeySpace, Runes: []rune{' '}},
		"tab":   {Type: tea.KeyTab},
		"left":  {Type: tea.KeyLeft},
	}
}

func TestFinalScreens_AnyKeyDismisses(t *testing.T) {
	for name, build := range newFinalScreens() {
		for keyName, key := range dismissKeys() {
			s := build()
			if !s.Update(key) || !s.Done() {
				t.Errorf("%s: key %q did not dismiss the screen", name, keyName)
			}
		}
	}
}

func TestFinalScreens_NonKeyMessageDoesNotDismiss(t *testing.T) {
	for name, build := range newFinalScreens() {
		s := build()
		if s.Update(tea.WindowSizeMsg{Width: 100, Height: 30}) || s.Done() {
			t.Errorf("%s: a non-key message dismissed the screen", name)
		}
	}
}

func TestSummaryScreen_ScrollKeysDoNotDismiss(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"up":   {Type: tea.KeyUp},
		"down": {Type: tea.KeyDown},
		"k":    {Type: tea.KeyRunes, Runes: []rune{'k'}},
		"j":    {Type: tea.KeyRunes, Runes: []rune{'j'}},
	}
	for name, key := range keys {
		s := screens.NewSummaryScreen(cleanSummary(), 120, 40, plainStyles())
		if s.Update(key) || s.Done() {
			t.Errorf("scroll key %q dismissed the run summary; scroll keys must only scroll", name)
		}
	}
}

func TestSummaryScreen_JKeyStillScrollsContent(t *testing.T) {
	s := screens.NewSummaryScreen(cleanSummary(), 80, 10, plainStyles())
	before := s.View()

	for i := 0; i < 5; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	if s.View() == before {
		t.Error("view unchanged after 'j' presses on a summary taller than the window; 'j' must scroll")
	}
}

func lastNonEmptyLine(view string) string {
	lines := strings.Split(strings.TrimRight(view, "\n \t"), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func TestSummaryScreen_HelpLine_NamesScrollKeysAndBackToStart(t *testing.T) {
	s := screens.NewSummaryScreen(cleanSummary(), 120, 40, plainStyles())

	help := collapseWhitespace(lastNonEmptyLine(s.View()))

	for _, want := range []string{"k up", "j down", "other keys: back to start", "ctrl+c: quit"} {
		if !strings.Contains(help, want) {
			t.Errorf("run summary help line %q does not contain %q", help, want)
		}
	}
	if strings.Contains(help, "q/enter exit") {
		t.Errorf("run summary help line %q still advertises q/enter exit", help)
	}
}

func TestNonScrollingFinalScreens_HelpLine_SaysAnyKeyGoesBackToStart(t *testing.T) {
	builds := newFinalScreens()
	for _, name := range []string{"promote summary", "transform summary"} {
		view := collapseWhitespace(builds[name]().View())

		if !strings.Contains(view, "any key: back to start") || !strings.Contains(view, "ctrl+c: quit") {
			t.Errorf("%s: help line lacks 'any key: back to start  ctrl+c: quit':\n%s", name, view)
		}
		if strings.Contains(view, "press q") || strings.Contains(view, "q/enter") {
			t.Errorf("%s: view still advertises exiting with q/enter:\n%s", name, view)
		}
	}
}
