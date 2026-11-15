package codex_test

// catalog_drift_test.go is a cross-module consistency test. It asserts that the
// embedded codex.yaml descriptor and the shared CLIHarness catalog entry for
// "codex" agree on the agents directory, the agent format ID, the agent file
// extension, and the model ids.
//
// This test guards against drift between the Deployment tool's YAML manifest
// and the Go-code catalog in Tools/Common/harness.

import (
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/codex"
)

func codexCatalogEntry(t *testing.T) commonharness.CLIHarness {
	t.Helper()
	entry, ok := commonharness.LookupCLIHarness(commonharness.HarnessIDCodex)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found in shared catalog", commonharness.HarnessIDCodex)
	}
	return entry
}

func TestAgentsDirMatchesDescriptor(t *testing.T) {
	desc := codex.DescriptorForTesting(t)
	entry := codexCatalogEntry(t)

	yamlValue := desc.Paths.Agents.Project
	if yamlValue == "" {
		t.Fatal("codex descriptor paths.agents.project is empty: descriptor may be malformed")
	}
	if entry.AgentsDir != yamlValue {
		t.Errorf("catalog/YAML drift for codex agents directory:\n  shared catalog AgentsDir = %q\n  YAML paths.agents.project = %q",
			entry.AgentsDir, yamlValue)
	}
}

func TestAgentFormatMatchesDescriptor(t *testing.T) {
	desc := codex.DescriptorForTesting(t)
	entry := codexCatalogEntry(t)

	want := desc.AgentFormatID
	if want == "" {
		want = "markdown"
	}
	if entry.AgentFormatID != want {
		t.Errorf("catalog/YAML drift for codex agent format:\n  shared catalog AgentFormatID = %q\n  YAML agent_format_id = %q",
			entry.AgentFormatID, want)
	}
}

func TestAgentExtensionMatchesDescriptor(t *testing.T) {
	desc := codex.DescriptorForTesting(t)
	entry := codexCatalogEntry(t)

	yamlValue := desc.Extensions[domain.ArtifactAgent]
	if yamlValue == "" {
		t.Fatal("codex descriptor extensions.agent is empty: descriptor may be malformed")
	}
	if entry.AgentExtension != yamlValue {
		t.Errorf("catalog/YAML drift for codex agent extension:\n  shared catalog AgentExtension = %q\n  YAML extensions.agent = %q",
			entry.AgentExtension, yamlValue)
	}
}

func TestModelIDsMatchDescriptor(t *testing.T) {
	desc := codex.DescriptorForTesting(t)

	cat, ok := commonharness.LookupModelCatalog(commonharness.HarnessIDCodex)
	if !ok {
		t.Fatalf("LookupModelCatalog(%q): not found in shared catalog", commonharness.HarnessIDCodex)
	}
	want := desc.Models.IDs
	if len(want) == 0 {
		t.Fatal("codex descriptor models.ids is empty: descriptor may be malformed")
	}
	if len(cat.IDs) != len(want) {
		t.Fatalf("catalog/YAML drift for codex model ids:\n  shared catalog IDs = %v\n  YAML models.ids = %v", cat.IDs, want)
	}
	for i := range want {
		if cat.IDs[i] != want[i] {
			t.Errorf("catalog/YAML drift for codex model id %d: catalog %q, YAML %q", i, cat.IDs[i], want[i])
		}
	}
}
