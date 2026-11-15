package runconfig

// Tests that the run configuration harness step marks an experimental harness
// and shows its reason, and renders unmarked harnesses as before.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/harness"
)

const stepExperimentalReason = "reason-token-4417"

func TestConfigScreen_HarnessStep_MarkedHarnessShowsMarkerAndReason(t *testing.T) {
	sels := harness.CLISelections()
	restore := harness.MarkExperimentalForTest(sels[0].ID, stepExperimentalReason)
	defer restore()
	s := newConfigScreenAtHarnessStep()
	if s.step != configStepHarness {
		t.Fatalf("fixture not at the harness step (step=%v)", s.step)
	}

	view := s.View()

	if !strings.Contains(view, harness.ExperimentalLabel) {
		t.Errorf("harness step lacks the %q marker; view:\n%s", harness.ExperimentalLabel, view)
	}
	if !strings.Contains(view, stepExperimentalReason) {
		t.Errorf("harness step lacks the experimental reason; view:\n%s", view)
	}
	for _, sel := range sels {
		if !strings.Contains(view, sel.Label) {
			t.Errorf("harness %q (%q) no longer listed; view:\n%s", sel.ID, sel.Label, view)
		}
	}
}

// Codex is marked experimental in production, so the harness step shows the
// marker and still lists every harness.
func TestConfigScreen_HarnessStep_ProductionMarkedCodexShowsMarker(t *testing.T) {
	if reason, ok := harness.ExperimentalReason(commonharness.HarnessIDCodex); !ok || reason == "" {
		t.Fatalf("codex is not marked experimental in the Runner")
	}
	s := newConfigScreenAtHarnessStep()

	view := s.View()

	if !strings.Contains(view, harness.ExperimentalLabel) {
		t.Errorf("harness step lacks the %q marker for codex; view:\n%s", harness.ExperimentalLabel, view)
	}
	for _, sel := range harness.CLISelections() {
		if !strings.Contains(view, sel.Label) {
			t.Errorf("harness %q (%q) not listed; view:\n%s", sel.ID, sel.Label, view)
		}
	}
}

func TestConfigScreen_HarnessStep_MarkedHarnessStaysSelectable(t *testing.T) {
	sels := harness.CLISelections()
	restore := harness.MarkExperimentalForTest(sels[0].ID, stepExperimentalReason)
	defer restore()
	s := newConfigScreenAtHarnessStep()

	pressKey(s, tea.KeyEnter)

	if s.sel.Harness != sels[0].ID {
		t.Errorf("sel.Harness = %q, want the marked harness %q", s.sel.Harness, sels[0].ID)
	}
}
