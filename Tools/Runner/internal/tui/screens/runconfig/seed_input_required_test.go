package runconfig_test

// seed_input_required_test.go verifies the seed screen's required and optional
// wording and the confirmation needed to start with a blank required seed.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens/runconfig"
)

const (
	optionalLabel    = "Seed input path (optional):"
	optionalSubtitle = "Enter an optional path to a file or directory to copy into the run folder before the first dispatch. Leave blank to start with no seeded artifacts."
	requiredLabel    = "Seed input path (required):"
)

func pressEnter(s *runconfig.SeedInputScreen) { s.Update(tea.KeyMsg{Type: tea.KeyEnter}) }

func typeText(s *runconfig.SeedInputScreen, text string) {
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
}

func requiredScreen(inputs ...string) *runconfig.SeedInputScreen {
	s := newSeedScreen()
	s.SetRequiredInputs(inputs)
	return s
}

func TestSeedInputScreen_Required_ShowsRequiredLabelAndNamesInputs(t *testing.T) {
	s := requiredScreen("Requirements.md", "Research.md")

	view := flat(s.View())

	for _, want := range []string{requiredLabel, "Requirements.md, Research.md", "first agent reads"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, optionalLabel) {
		t.Errorf("View() still shows the optional label:\n%s", view)
	}
}

func TestSeedInputScreen_Optional_ShowsTodaysWording(t *testing.T) {
	for name, s := range map[string]*runconfig.SeedInputScreen{
		"fresh": newSeedScreen(),
		"nil":   requiredScreen(),
	} {
		view := flat(s.View())
		for _, want := range []string{optionalLabel, optionalSubtitle} {
			if !strings.Contains(view, want) {
				t.Errorf("%s: View() lacks %q:\n%s", name, want, view)
			}
		}
	}
}

func TestSeedInputScreen_SetRequiredInputs_EmptyAfterRequired_RestoresOptionalWording(t *testing.T) {
	s := requiredScreen("Requirements.md")

	s.SetRequiredInputs(nil)

	view := flat(s.View())
	if !strings.Contains(view, optionalLabel) || !strings.Contains(view, optionalSubtitle) {
		t.Errorf("optional wording not restored:\n%s", view)
	}
	if strings.Contains(view, "Requirements.md") {
		t.Errorf("stale required input still shown:\n%s", view)
	}
}

func TestSeedInputScreen_SetRequiredInputs_ReplacesPreviousInputs(t *testing.T) {
	s := requiredScreen("Requirements.md")

	s.SetRequiredInputs([]string{"Plan.md"})

	view := flat(s.View())
	if !strings.Contains(view, "Plan.md") || strings.Contains(view, "Requirements.md") {
		t.Errorf("View() should name only Plan.md:\n%s", view)
	}
}

func TestSeedInputScreen_Required_NonBlankValue_EnterIsDone(t *testing.T) {
	s := requiredScreen("Requirements.md")
	typeText(s, "/seed/Requirements.md")

	pressEnter(s)

	if !s.Done() {
		t.Fatal("Done() = false after Enter with a path; want true")
	}
	if got := s.SeedInput(); got != "/seed/Requirements.md" {
		t.Errorf("SeedInput() = %q, want the entered path", got)
	}
}

func TestSeedInputScreen_Required_FirstBlankEnter_NotDoneAndNamesMissingInputs(t *testing.T) {
	s := requiredScreen("Requirements.md")

	pressEnter(s)

	if s.Done() {
		t.Fatal("Done() = true after the first Enter on a blank required seed; want false")
	}
	view := flat(s.View())
	for _, want := range []string{"No seed input given", "Requirements.md", "Press Enter again to start without it"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() lacks %q after the first blank Enter:\n%s", want, view)
		}
	}
}

func TestSeedInputScreen_Required_SecondBlankEnter_ConfirmsWithEmptySeed(t *testing.T) {
	s := requiredScreen("Requirements.md")

	pressEnter(s)
	pressEnter(s)

	if !s.Done() {
		t.Fatal("Done() = false after the second Enter on a blank required seed; want true")
	}
	if got := s.SeedInput(); got != "" {
		t.Errorf("SeedInput() = %q, want empty", got)
	}
}

func TestSeedInputScreen_Required_WhitespaceOnlyCountsAsBlank(t *testing.T) {
	s := requiredScreen("Requirements.md")
	typeText(s, "   ")

	pressEnter(s)

	if s.Done() {
		t.Error("Done() = true after Enter on a whitespace-only required seed; want false")
	}
}

func TestSeedInputScreen_Required_TypingClearsPendingConfirmation(t *testing.T) {
	s := requiredScreen("Requirements.md")
	pressEnter(s) // pending confirmation
	typeText(s, "x")
	s.Update(tea.KeyMsg{Type: tea.KeyBackspace}) // blank again

	if strings.Contains(flat(s.View()), "No seed input given") {
		t.Error("error line still shown after the value changed")
	}
	pressEnter(s)
	if s.Done() {
		t.Error("Done() = true on the first Enter after the value changed; confirmation must restart")
	}
}

func TestSeedInputScreen_Required_EscClearsPendingConfirmation(t *testing.T) {
	s := requiredScreen("Requirements.md")
	pressEnter(s)
	s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !s.Back() {
		t.Fatal("precondition: Back() = false after Esc")
	}
	s.Reset()

	pressEnter(s)

	if s.Done() {
		t.Error("Done() = true on the first Enter after Esc and re-entry; confirmation must restart")
	}
}

func TestSeedInputScreen_Required_ResetClearsPendingConfirmation(t *testing.T) {
	s := requiredScreen("Requirements.md")
	pressEnter(s)
	s.Reset()

	if strings.Contains(flat(s.View()), "No seed input given") {
		t.Error("error line still shown after Reset")
	}
	pressEnter(s)
	if s.Done() {
		t.Error("Done() = true on the first Enter after Reset; confirmation must restart")
	}
}

func TestSeedInputScreen_SetRequiredInputs_ClearsPendingConfirmationAndKeepsValue(t *testing.T) {
	s := requiredScreen("Requirements.md")
	pressEnter(s) // pending confirmation on blank

	s.SetRequiredInputs([]string{"Plan.md"})

	if strings.Contains(flat(s.View()), "No seed input given") {
		t.Error("error line survived SetRequiredInputs")
	}
	pressEnter(s)
	if s.Done() {
		t.Error("blank confirmation survived SetRequiredInputs; first Enter must warn again")
	}

	s2 := requiredScreen("Requirements.md")
	typeText(s2, "/kept/path")
	s2.SetRequiredInputs([]string{"Plan.md"})
	pressEnter(s2)
	if !s2.Done() || s2.SeedInput() != "/kept/path" {
		t.Errorf("entered value not kept: Done()=%v SeedInput()=%q", s2.Done(), s2.SeedInput())
	}
}

func TestSeedInputScreen_Optional_BlankEnterStillDoneImmediately(t *testing.T) {
	s := requiredScreen()

	pressEnter(s)

	if !s.Done() || s.SeedInput() != "" {
		t.Errorf("optional blank Enter: Done()=%v SeedInput()=%q, want done and empty", s.Done(), s.SeedInput())
	}
}

// flat collapses the wrapping and padding of a rendered view into single spaces
// so wording can be asserted independent of the terminal width.
func flat(view string) string { return strings.Join(strings.Fields(view), " ") }
