package harness_test

// OpenCode V2 in the Runner's harness selection: accepted by the --harness
// flag, offered by every selection UI as a CLI-backed harness, marked
// experimental with the shared descriptor's reason, and its orchestrator is
// discovered in the V2 agents folder.

import (
	"strings"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/harness"
)

func TestSelections_OpenCodeV2IsAcceptedAndCLIBacked(t *testing.T) {
	if !harness.Accepts(commonharness.HarnessIDOpenCodeV2) {
		t.Errorf("Accepts(%q) = false, want true", commonharness.HarnessIDOpenCodeV2)
	}
	for name, sels := range map[string][]harness.Selection{
		"Selections":    harness.Selections(),
		"CLISelections": harness.CLISelections(),
	} {
		got := findSelection(t, sels, commonharness.HarnessIDOpenCodeV2)
		if !got.CLIBacked {
			t.Errorf("%s: opencode-v2 must be CLIBacked", name)
		}
		if got.Label != "OpenCode V2 CLI" {
			t.Errorf("%s: opencode-v2 Label = %q, want %q", name, got.Label, "OpenCode V2 CLI")
		}
	}
}

func TestFlagValues_ListOpenCodeV2(t *testing.T) {
	if got := harness.FlagValues(); !strings.Contains(got, commonharness.HarnessIDOpenCodeV2) {
		t.Errorf("FlagValues() = %q: want it to contain opencode-v2", got)
	}
	if got := harness.FlagValueList(); !strings.Contains(got, commonharness.HarnessIDOpenCodeV2) {
		t.Errorf("FlagValueList() = %q: want it to contain opencode-v2", got)
	}
}

func TestExperimentalReason_OpenCodeV2UsesTheDescriptorReason(t *testing.T) {
	entry, ok := commonharness.LookupCLIHarness(commonharness.HarnessIDOpenCodeV2)
	if !ok {
		t.Fatalf("catalog has no %q entry", commonharness.HarnessIDOpenCodeV2)
	}

	reason, marked := harness.ExperimentalReason(commonharness.HarnessIDOpenCodeV2)

	if !marked || strings.TrimSpace(reason) == "" {
		t.Fatalf("ExperimentalReason(opencode-v2) = (%q, %v), want a marked harness with a reason", reason, marked)
	}
	if reason != entry.ExperimentalReason {
		t.Errorf("reason = %q, want the descriptor's %q", reason, entry.ExperimentalReason)
	}
}

func TestExperimentalReason_CodexKeepsItsRunnerOwnReason(t *testing.T) {
	reason, marked := harness.ExperimentalReason(commonharness.HarnessIDCodex)

	if !marked || reason != "Codex support is not yet proven in real runs." {
		t.Errorf("ExperimentalReason(codex) = (%q, %v), want the Runner's own entry unchanged", reason, marked)
	}
}

func TestExperimentalReason_MarkForTestOverridesTheDescriptorReason(t *testing.T) {
	restore := harness.MarkExperimentalForTest(commonharness.HarnessIDOpenCodeV2, testExperimentalReason)

	reason, _ := harness.ExperimentalReason(commonharness.HarnessIDOpenCodeV2)
	restore()

	if reason != testExperimentalReason {
		t.Errorf("reason while marked for test = %q, want %q", reason, testExperimentalReason)
	}
	entry, _ := commonharness.LookupCLIHarness(commonharness.HarnessIDOpenCodeV2)
	if after, _ := harness.ExperimentalReason(commonharness.HarnessIDOpenCodeV2); after != entry.ExperimentalReason {
		t.Errorf("after restore reason = %q, want the descriptor's %q", after, entry.ExperimentalReason)
	}
}

func TestSelections_OpenCodeV2CarriesItsExperimentalReasonAndDisplayLabel(t *testing.T) {
	want, _ := harness.ExperimentalReason(commonharness.HarnessIDOpenCodeV2)
	for name, sels := range map[string][]harness.Selection{
		"Selections":    harness.Selections(),
		"CLISelections": harness.CLISelections(),
	} {
		got := findSelection(t, sels, commonharness.HarnessIDOpenCodeV2)
		if got.ExperimentalReason == "" || got.ExperimentalReason != want {
			t.Errorf("%s: opencode-v2 ExperimentalReason = %q, want %q", name, got.ExperimentalReason, want)
		}
		label := got.DisplayLabel()
		if !strings.Contains(label, harness.ExperimentalLabel) || !strings.Contains(label, want) {
			t.Errorf("%s: DisplayLabel() = %q, want the experimental marker and reason", name, label)
		}
	}
}

func TestDiscoverOrchestrator_OpenCodeV2_FindsTheV2AgentsFolder(t *testing.T) {
	workDir := writeTempOrchestrator(t, commonharness.HarnessIDOpenCodeV2, ".opencode/agents")

	got, err := harness.DiscoverOrchestrator(workDir, commonharness.HarnessIDOpenCodeV2)

	if err != nil {
		t.Fatalf("DiscoverOrchestrator opencode-v2: unexpected error: %v", err)
	}
	if !strings.HasSuffix(strings.ReplaceAll(got, "\\", "/"), ".opencode/agents/orchestrator-script.md") {
		t.Errorf("got %q, want a path ending in .opencode/agents/orchestrator-script.md", got)
	}
}
