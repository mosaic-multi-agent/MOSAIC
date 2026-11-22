package harness_test

// catalog_opencodev2_test.go pins the opencode-v2 entry of the shared CLI
// harness catalog, its model data and the experimental reason every catalog
// entry exposes.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-common/harness"
)

func TestCLIHarnesses_OpenCodeV2EntryData(t *testing.T) {
	entry, ok := harness.LookupCLIHarness(harness.HarnessIDOpenCodeV2)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found in the catalog", harness.HarnessIDOpenCodeV2)
	}
	if harness.HarnessIDOpenCodeV2 != "opencode-v2" {
		t.Errorf("HarnessIDOpenCodeV2 = %q, want %q", harness.HarnessIDOpenCodeV2, "opencode-v2")
	}
	if entry.Label != "OpenCode V2 CLI" {
		t.Errorf("Label = %q, want %q", entry.Label, "OpenCode V2 CLI")
	}
	if entry.AgentsDir != ".opencode/agents" {
		t.Errorf("AgentsDir = %q, want %q", entry.AgentsDir, ".opencode/agents")
	}
	if entry.LoadingMechanism != harness.LoadingMechanismName {
		t.Errorf("LoadingMechanism = %v, want LoadingMechanismName (no snapshot transform for V2)", entry.LoadingMechanism)
	}
	if entry.AgentFormatID != "markdown" || entry.AgentExtension != ".md" {
		t.Errorf("format/extension = %q/%q, want markdown/.md", entry.AgentFormatID, entry.AgentExtension)
	}
}

func TestCLIHarnesses_OpenCodeV2IsTheLastCatalogEntry(t *testing.T) {
	entries := harness.CLIHarnesses()
	if got := entries[len(entries)-1].ID; got != harness.HarnessIDOpenCodeV2 {
		t.Errorf("last catalog entry = %q, want %q", got, harness.HarnessIDOpenCodeV2)
	}
}

func TestCLIHarnesses_ExperimentalReasonOnlyForCodexAndOpenCodeV2(t *testing.T) {
	for _, e := range harness.CLIHarnesses() {
		experimental := e.ID == harness.HarnessIDCodex || e.ID == harness.HarnessIDOpenCodeV2
		if experimental && strings.TrimSpace(e.ExperimentalReason) == "" {
			t.Errorf("%s: ExperimentalReason is empty, want the descriptor's reason", e.ID)
		}
		if !experimental && e.ExperimentalReason != "" {
			t.Errorf("%s: ExperimentalReason = %q, want empty", e.ID, e.ExperimentalReason)
		}
	}
}

func TestCLIHarnesses_OpenCodeV2ReasonStatesSupportedButNotYetTested(t *testing.T) {
	entry, ok := harness.LookupCLIHarness(harness.HarnessIDOpenCodeV2)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found", harness.HarnessIDOpenCodeV2)
	}
	got := strings.ToLower(entry.ExperimentalReason)
	if !strings.Contains(got, "supported") || !strings.Contains(got, "not yet tested") {
		t.Errorf("ExperimentalReason = %q, want it to state everything is supported but not yet tested", entry.ExperimentalReason)
	}
}

func TestCLIHarnesses_CodexExperimentalReasonComesFromItsDescriptor(t *testing.T) {
	entry, ok := harness.LookupCLIHarness(harness.HarnessIDCodex)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found", harness.HarnessIDCodex)
	}
	if !strings.HasPrefix(entry.ExperimentalReason, "Codex agents run through mosaic-run only.") {
		t.Errorf("ExperimentalReason = %q, want the Codex descriptor reason", entry.ExperimentalReason)
	}
}

func TestLookupModelCatalog_OpenCodeV2MirrorsOpenCodeIDsWithVariantHint(t *testing.T) {
	v1, ok := harness.LookupModelCatalog(harness.HarnessIDOpenCode)
	if !ok {
		t.Fatalf("LookupModelCatalog(%q): not found", harness.HarnessIDOpenCode)
	}
	v2, ok := harness.LookupModelCatalog(harness.HarnessIDOpenCodeV2)
	if !ok {
		t.Fatalf("LookupModelCatalog(%q): not found", harness.HarnessIDOpenCodeV2)
	}
	if !reflect.DeepEqual(v2.IDs, v1.IDs) {
		t.Errorf("IDs = %v, want the same list and order as opencode %v", v2.IDs, v1.IDs)
	}
	if v2.FormatHint != "provider/model-id[#variant]" {
		t.Errorf("FormatHint = %q, want %q", v2.FormatHint, "provider/model-id[#variant]")
	}
}

func TestIsModelID_OpenCodeV2AcceptsOpenCodeModels(t *testing.T) {
	cat, ok := harness.LookupModelCatalog(harness.HarnessIDOpenCode)
	if !ok || len(cat.IDs) == 0 {
		t.Fatalf("opencode catalog missing or empty")
	}
	if !harness.IsModelID(harness.HarnessIDOpenCodeV2, cat.IDs[0]) {
		t.Errorf("IsModelID(opencode-v2, %q) = false, want true", cat.IDs[0])
	}
}
