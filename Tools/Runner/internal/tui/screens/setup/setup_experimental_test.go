package setup

// Tests that the setup harness picker marks an experimental harness and shows
// its reason, and renders unmarked harnesses as before.

import (
	"strings"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/harness"
	"mosaic-run/internal/tui/screens"
)

const pickerExperimentalReason = "reason-token-4417"

func TestHarnessSelectScreen_MarkedHarnessShowsMarkerAndReason(t *testing.T) {
	sels := harness.CLISelections()
	restore := harness.MarkExperimentalForTest(sels[0].ID, pickerExperimentalReason)
	defer restore()

	constructors := map[string]func() *HarnessSelectScreen{
		"normal":  func() *HarnessSelectScreen { return NewHarnessSelectScreen(120, 30, screens.Styles{}) },
		"devmode": func() *HarnessSelectScreen { return NewHarnessSelectScreenDevMode(120, 30, screens.Styles{}) },
	}
	for name, build := range constructors {
		view := build().View()
		if !strings.Contains(view, harness.ExperimentalLabel) {
			t.Errorf("%s: view lacks the %q marker; view:\n%s", name, harness.ExperimentalLabel, view)
		}
		if !strings.Contains(view, pickerExperimentalReason) {
			t.Errorf("%s: view lacks the experimental reason; view:\n%s", name, view)
		}
		for _, s := range sels {
			if !strings.Contains(view, s.Label) {
				t.Errorf("%s: harness %q (%q) no longer listed", name, s.ID, s.Label)
			}
		}
	}
}

// Codex is marked experimental in production, so the picker shows the marker
// and still lists every harness.
func TestHarnessSelectScreen_ProductionMarkedCodexShowsMarker(t *testing.T) {
	codexEntry, ok := harness.ExperimentalReason(commonharness.HarnessIDCodex)
	if !ok || codexEntry == "" {
		t.Fatalf("codex is not marked experimental in the Runner")
	}
	constructors := map[string]func() *HarnessSelectScreen{
		"normal":  func() *HarnessSelectScreen { return NewHarnessSelectScreen(120, 30, screens.Styles{}) },
		"devmode": func() *HarnessSelectScreen { return NewHarnessSelectScreenDevMode(120, 30, screens.Styles{}) },
	}
	for name, build := range constructors {
		view := build().View()
		if !strings.Contains(view, harness.ExperimentalLabel) {
			t.Errorf("%s: view lacks the %q marker for codex; view:\n%s", name, harness.ExperimentalLabel, view)
		}
		for _, s := range harness.CLISelections() {
			if !strings.Contains(view, s.Label) {
				t.Errorf("%s: harness %q (%q) not listed", name, s.ID, s.Label)
			}
		}
	}
}

func TestHarnessSelectScreen_MarkedHarnessStaysSelectable(t *testing.T) {
	sels := harness.CLISelections()
	restore := harness.MarkExperimentalForTest(sels[0].ID, pickerExperimentalReason)
	defer restore()
	s := NewHarnessSelectScreen(120, 30, screens.Styles{})

	s.list.Update(keyEnter())

	if !s.Done() || s.SelectedID() != sels[0].ID {
		t.Errorf("Done=%v SelectedID=%q, want the marked harness %q selected", s.Done(), s.SelectedID(), sels[0].ID)
	}
}
