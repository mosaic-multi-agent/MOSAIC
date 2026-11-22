package screens_test

// models_ctrlkeys_test.go verifies the ctrl+k and ctrl+x aliases of the model selection
// dialog. In list mode ctrl+k behaves as 's' (skip this one, only when AllowSkip) and ctrl+x
// as 'S' (skip all, only when AllowSkipAll); in custom-entry mode keys go to the text input;
// the help line names both keys under the same conditions.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

var (
	ctrlK = tea.KeyMsg{Type: tea.KeyCtrlK}
	ctrlX = tea.KeyMsg{Type: tea.KeyCtrlX}
)

func newModelScreen(allowSkip, allowSkipAll, allowCustom bool) *screens.ModelSelectScreen {
	q := tierModelQuestion([]string{"opus", "sonnet"}, allowSkip, allowSkipAll, allowCustom, "")
	return screens.NewModelSelectScreen(q, 100, 30, plainStyles())
}

func TestModelSelectScreen_CtrlK_SkipsThisOneWhenAllowed(t *testing.T) {
	s := newModelScreen(true, true, false)

	done := s.Update(ctrlK)

	if !done || !s.Done() {
		t.Fatal("ctrl+k with AllowSkip did not finish the dialog")
	}
	if got := s.Answer().Status; got != domain.SkippedOne {
		t.Errorf("Answer().Status = %q after ctrl+k; want %q", got, domain.SkippedOne)
	}
}

func TestModelSelectScreen_CtrlX_SkipsAllWhenAllowed(t *testing.T) {
	s := newModelScreen(true, true, false)

	done := s.Update(ctrlX)

	if !done || !s.Done() {
		t.Fatal("ctrl+x with AllowSkipAll did not finish the dialog")
	}
	if got := s.Answer().Status; got != domain.SkippedAll {
		t.Errorf("Answer().Status = %q after ctrl+x; want %q", got, domain.SkippedAll)
	}
}

func TestModelSelectScreen_CtrlKeys_AreInertWhenTheSkipIsNotAllowed(t *testing.T) {
	s := newModelScreen(false, false, false)

	if s.Update(ctrlK) || s.Update(ctrlX) || s.Done() {
		t.Error("ctrl+k or ctrl+x finished the dialog although no skip is allowed")
	}
}

func TestModelSelectScreen_CtrlK_WithOnlySkipAllAllowed_IsInert(t *testing.T) {
	s := newModelScreen(false, true, false)

	if s.Update(ctrlK) || s.Done() {
		t.Error("ctrl+k finished the dialog although AllowSkip is false")
	}
	if !s.Update(ctrlX) {
		t.Error("ctrl+x did not skip all although AllowSkipAll is true")
	}
}

func TestModelSelectScreen_CtrlX_WithOnlySkipAllowed_IsInert(t *testing.T) {
	s := newModelScreen(true, false, false)

	if s.Update(ctrlX) || s.Done() {
		t.Error("ctrl+x finished the dialog although AllowSkipAll is false")
	}
	if !s.Update(ctrlK) {
		t.Error("ctrl+k did not skip although AllowSkip is true")
	}
}

func TestModelSelectScreen_CustomEntry_CtrlKeysDoNotSkip(t *testing.T) {
	s := newModelScreen(true, true, true)
	s.Update(modelDownKey()) // opus -> sonnet
	s.Update(modelDownKey()) // sonnet -> custom entry row
	s.Update(modelEnterKey())
	if s.Done() || !strings.Contains(s.View(), "Model ID") {
		t.Fatalf("precondition: the custom entry is not active:\n%s", s.View())
	}

	s.Update(ctrlK)
	s.Update(ctrlX)

	if s.Done() {
		t.Error("ctrl+k or ctrl+x skipped while the custom entry was active; keys must go to the text input")
	}
}

func TestModelSelectScreen_HelpLine_NamesTheCtrlAliases(t *testing.T) {
	view := collapseWhitespace(newModelScreen(true, true, false).View())

	for _, want := range []string{"s/ctrl+k skip", "S/ctrl+x skip all"} {
		if !strings.Contains(view, want) {
			t.Errorf("help line does not contain %q:\n%s", want, view)
		}
	}
}

func TestModelSelectScreen_HelpLine_OmitsSkipKeysThatAreNotAllowed(t *testing.T) {
	view := collapseWhitespace(newModelScreen(true, false, false).View())

	if !strings.Contains(view, "s/ctrl+k skip") {
		t.Errorf("help line does not offer the skip key:\n%s", view)
	}
	if strings.Contains(view, "ctrl+x") {
		t.Errorf("help line offers skip all although AllowSkipAll is false:\n%s", view)
	}
}
