package harness_test

// catalog_codex_test.go pins the Codex entry of the shared CLI harness catalog
// and its model data. The values mirror Deployment's Codex descriptor; the
// drift guards in Tools/Deployment keep the two in sync.

import (
	"testing"

	"mosaic-common/agentformat/formatid"
	"mosaic-common/harness"
)

func TestCLIHarnesses_CodexEntryData(t *testing.T) {
	entry, ok := harness.LookupCLIHarness(harness.HarnessIDCodex)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found in the catalog", harness.HarnessIDCodex)
	}
	if harness.HarnessIDCodex != "codex" {
		t.Errorf("HarnessIDCodex = %q, want %q", harness.HarnessIDCodex, "codex")
	}
	if entry.ID != "codex" {
		t.Errorf("ID = %q, want %q", entry.ID, "codex")
	}
	if entry.Label != "Codex CLI" {
		t.Errorf("Label = %q, want %q", entry.Label, "Codex CLI")
	}
	if entry.AgentsDir != ".codex/agents" {
		t.Errorf("AgentsDir = %q, want %q", entry.AgentsDir, ".codex/agents")
	}
	if entry.AgentFormatID != string(formatid.CodexTOML) {
		t.Errorf("AgentFormatID = %q, want %q", entry.AgentFormatID, formatid.CodexTOML)
	}
	if entry.AgentExtension != ".toml" {
		t.Errorf("AgentExtension = %q, want %q", entry.AgentExtension, ".toml")
	}
}

func TestCLIHarnesses_CodexIsRunnerLoaded(t *testing.T) {
	entry, ok := harness.LookupCLIHarness(harness.HarnessIDCodex)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found in the catalog", harness.HarnessIDCodex)
	}
	if entry.LoadingMechanism != harness.LoadingMechanismRunnerLoaded {
		t.Errorf("LoadingMechanism = %v, want LoadingMechanismRunnerLoaded", entry.LoadingMechanism)
	}
}

func TestCLIHarnesses_CodexIsTheLastCatalogEntry(t *testing.T) {
	entries := harness.CLIHarnesses()
	if got := entries[len(entries)-1].ID; got != harness.HarnessIDCodex {
		t.Errorf("last catalog entry = %q, want %q (declared after the existing harnesses)", got, harness.HarnessIDCodex)
	}
}

func TestLoadingMechanism_RunnerLoadedStringAndDistinct(t *testing.T) {
	if got := harness.LoadingMechanismRunnerLoaded.String(); got != "runner-loaded" {
		t.Errorf("String() = %q, want %q", got, "runner-loaded")
	}
	if harness.LoadingMechanismRunnerLoaded == harness.LoadingMechanismName ||
		harness.LoadingMechanismRunnerLoaded == harness.LoadingMechanismUnset {
		t.Errorf("LoadingMechanismRunnerLoaded must differ from Unset and Name")
	}
}

func TestLookupModelCatalog_CodexModelIDsInOrder(t *testing.T) {
	cat, ok := harness.LookupModelCatalog(harness.HarnessIDCodex)
	if !ok {
		t.Fatalf("LookupModelCatalog(%q): not found", harness.HarnessIDCodex)
	}
	want := []string{"gpt-6-astra", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"}
	if len(cat.IDs) != len(want) {
		t.Fatalf("IDs = %v, want %v", cat.IDs, want)
	}
	for i := range want {
		if cat.IDs[i] != want[i] {
			t.Errorf("IDs[%d] = %q, want %q", i, cat.IDs[i], want[i])
		}
	}
	if cat.FormatHint == "" {
		t.Errorf("want a non-empty FormatHint for %q", harness.HarnessIDCodex)
	}
}

func TestIsModelID_CodexModelAcceptedForCodexOnly(t *testing.T) {
	if !harness.IsModelID(harness.HarnessIDCodex, "gpt-5.6-sol") {
		t.Errorf("IsModelID(codex, gpt-5.6-sol) = false, want true")
	}
	if harness.IsModelID(harness.HarnessIDClaudeCode, "gpt-5.6-luna") {
		t.Errorf("IsModelID(claude-code, gpt-5.6-luna) = true, want false")
	}
}
