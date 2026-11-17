package harness_test

// Codex in the Runner's harness selection: accepted by the --harness flag,
// offered by every selection UI as a CLI-backed harness, and marked
// experimental with a reason in production.

import (
	"strings"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/harness"
)

func TestSelections_CodexIsAcceptedAndCLIBacked(t *testing.T) {
	if !harness.Accepts(commonharness.HarnessIDCodex) {
		t.Errorf("Accepts(%q) = false, want true", commonharness.HarnessIDCodex)
	}
	for name, sels := range map[string][]harness.Selection{
		"Selections":    harness.Selections(),
		"CLISelections": harness.CLISelections(),
	} {
		got := findSelection(t, sels, commonharness.HarnessIDCodex)
		if !got.CLIBacked {
			t.Errorf("%s: codex must be CLIBacked", name)
		}
		if got.Label != "Codex CLI" {
			t.Errorf("%s: codex Label = %q, want %q", name, got.Label, "Codex CLI")
		}
	}
}

func TestFlagValues_ListCodex(t *testing.T) {
	if got := harness.FlagValues(); !strings.Contains(got, commonharness.HarnessIDCodex) {
		t.Errorf("FlagValues() = %q: want it to contain codex", got)
	}
	if got := harness.FlagValueList(); !strings.Contains(got, commonharness.HarnessIDCodex) {
		t.Errorf("FlagValueList() = %q: want it to contain codex", got)
	}
}

func TestExperimentalReason_CodexIsMarkedInProductionWithAReason(t *testing.T) {
	reason, ok := harness.ExperimentalReason(commonharness.HarnessIDCodex)

	if !ok || strings.TrimSpace(reason) == "" {
		t.Fatalf("ExperimentalReason(codex) = (%q, %v), want a marked harness with a non-empty reason", reason, ok)
	}
}

func TestSelections_CodexCarriesItsExperimentalReason(t *testing.T) {
	want, _ := harness.ExperimentalReason(commonharness.HarnessIDCodex)
	for name, sels := range map[string][]harness.Selection{
		"Selections":    harness.Selections(),
		"CLISelections": harness.CLISelections(),
	} {
		got := findSelection(t, sels, commonharness.HarnessIDCodex)
		if got.ExperimentalReason == "" || got.ExperimentalReason != want {
			t.Errorf("%s: codex ExperimentalReason = %q, want %q", name, got.ExperimentalReason, want)
		}
	}
}

func TestExperimentalReason_OnlyCodexAndOpenCodeV2AreMarkedInProduction(t *testing.T) {
	for _, e := range commonharness.CLIHarnesses() {
		_, marked := harness.ExperimentalReason(e.ID)
		want := e.ID == commonharness.HarnessIDCodex || e.ID == commonharness.HarnessIDOpenCodeV2
		if marked != want {
			t.Errorf("ExperimentalReason(%q) marked = %v, want %v", e.ID, marked, want)
		}
	}
}
