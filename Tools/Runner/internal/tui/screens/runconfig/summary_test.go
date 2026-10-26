package runconfig

// summary_test.go verifies the read-only settings summary screen: it shows the
// shared summary lines verbatim, enter confirms, esc goes back, and nothing else
// changes anything.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens"
)

func summaryFixtureLines() []string {
	return []string{
		"Recorded settings for run 20260701T120000Z-a3f9",
		"  Workflow:           build-flow (version 2.0)",
		"  Mode:               auto",
		"  Checkpoints:        on (cp-b)",
		"  Commits:            on (branch mosaic/run/20260701T120000Z-a3f9, mosaic-owned)",
		"  Review loop limit:  5",
	}
}

func newSummaryFixture() *SummaryScreen {
	return NewSummaryScreen(summaryFixtureLines(), 120, 30, screens.Styles{})
}

func TestSummaryScreen_View_ShowsTitleLinesVerbatimAndHelp(t *testing.T) {
	s := newSummaryFixture()

	view := s.View()

	if !strings.Contains(view, "Resume: recorded settings") {
		t.Errorf("view lacks the title; view:\n%s", view)
	}
	for _, line := range summaryFixtureLines() {
		if !strings.Contains(view, line) {
			t.Errorf("view lacks the summary line %q verbatim; view:\n%s", line, view)
		}
	}
	if !strings.Contains(view, "enter  start run   esc  back") {
		t.Errorf("view lacks the help line; view:\n%s", view)
	}
}

func TestSummaryScreen_Enter_Confirms(t *testing.T) {
	s := newSummaryFixture()

	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !s.Confirmed() {
		t.Error("Confirmed() = false after enter, want true")
	}
	if s.Back() {
		t.Error("Back() = true after enter, want false")
	}
}

func TestSummaryScreen_Esc_GoesBack(t *testing.T) {
	s := newSummaryFixture()

	s.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !s.Back() {
		t.Error("Back() = false after esc, want true")
	}
	if s.Confirmed() {
		t.Error("Confirmed() = true after esc, want false")
	}
}

// The screen is read-only: no other key confirms, goes back or edits anything.
func TestSummaryScreen_OtherKeys_ChangeNothing(t *testing.T) {
	s := newSummaryFixture()
	before := s.View()
	if !strings.Contains(before, "Resume: recorded settings") {
		t.Fatalf("precondition: the screen renders nothing; view:\n%s", before)
	}

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyDown}, {Type: tea.KeyUp}, {Type: tea.KeySpace},
		{Type: tea.KeyRunes, Runes: []rune("y")}, {Type: tea.KeyBackspace},
	} {
		s.Update(msg)
	}

	if s.Confirmed() || s.Back() {
		t.Errorf("Confirmed=%v Back=%v after keys other than enter/esc, want both false", s.Confirmed(), s.Back())
	}
	if after := s.View(); after != before {
		t.Errorf("view changed after keys on a read-only screen:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestSummaryScreen_Reset_ClearsConfirmedAndBack(t *testing.T) {
	s := newSummaryFixture()
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !s.Confirmed() {
		t.Fatal("precondition: enter did not confirm")
	}

	s.Reset()

	if s.Confirmed() || s.Back() {
		t.Errorf("Confirmed=%v Back=%v after Reset, want both false", s.Confirmed(), s.Back())
	}
}

func TestSummaryScreen_Resize_KeepsEveryLine(t *testing.T) {
	s := newSummaryFixture()

	s.Resize(200, 50)

	view := s.View()
	for _, line := range summaryFixtureLines() {
		if !strings.Contains(view, line) {
			t.Errorf("after Resize the view lacks %q; view:\n%s", line, view)
		}
	}
}
