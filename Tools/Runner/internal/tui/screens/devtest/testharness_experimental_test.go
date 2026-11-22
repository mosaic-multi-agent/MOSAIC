package devtest_test

// Tests that the test-mode harness picker marks an experimental harness and
// shows its reason, and renders unmarked harnesses as before.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/harness"
	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/devtest"
)

const testModeExperimentalReason = "reason-token-4417"

func TestTestHarnessScreen_MarkedHarnessShowsMarkerAndReason(t *testing.T) {
	sels := harness.CLISelections()
	restore := harness.MarkExperimentalForTest(sels[0].ID, testModeExperimentalReason)
	defer restore()

	view := devtest.NewTestHarnessScreen(120, 30, screens.Styles{}).View()

	if !strings.Contains(view, harness.ExperimentalLabel) {
		t.Errorf("view lacks the %q marker; view:\n%s", harness.ExperimentalLabel, view)
	}
	if !strings.Contains(view, testModeExperimentalReason) {
		t.Errorf("view lacks the experimental reason; view:\n%s", view)
	}
	for _, s := range sels {
		if !strings.Contains(view, s.Label) {
			t.Errorf("harness %q (%q) no longer listed", s.ID, s.Label)
		}
	}
}

// Codex is marked experimental in production, so the test-mode picker shows
// the marker and still lists every harness.
func TestTestHarnessScreen_ProductionMarkedCodexShowsMarker(t *testing.T) {
	if reason, ok := harness.ExperimentalReason(commonharness.HarnessIDCodex); !ok || reason == "" {
		t.Fatalf("codex is not marked experimental in the Runner")
	}

	view := devtest.NewTestHarnessScreen(120, 30, screens.Styles{}).View()

	if !strings.Contains(view, harness.ExperimentalLabel) {
		t.Errorf("view lacks the %q marker for codex; view:\n%s", harness.ExperimentalLabel, view)
	}
	for _, s := range harness.CLISelections() {
		if !strings.Contains(view, s.Label) {
			t.Errorf("harness %q (%q) not listed; view:\n%s", s.ID, s.Label, view)
		}
	}
}

func TestTestHarnessScreen_MarkedHarnessStaysSelectable(t *testing.T) {
	sels := harness.CLISelections()
	restore := harness.MarkExperimentalForTest(sels[0].ID, testModeExperimentalReason)
	defer restore()
	s := devtest.NewTestHarnessScreen(120, 30, screens.Styles{})

	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	got := s.SelectedHarnesses()
	if !s.Done() || len(got) != 1 || got[0] != sels[0].ID {
		t.Errorf("Done=%v selected=%v, want only the marked harness %q", s.Done(), got, sels[0].ID)
	}
}
