package opencode_test

// agents_format_test.go is a cross-module consistency test. It asserts that the
// agent_format_id and extensions.agent values of the embedded opencode.yaml
// descriptor match the AgentFormatID and AgentExtension fields of the shared
// CLIHarness catalog entry for "opencode".
//
// This test guards against drift between the Deployment tool's YAML manifests
// and the Go-code catalog in Tools/Common/harness/catalog.go.

import (
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/opencode"
)

// TestAgentFormatMatchesDescriptor asserts catalog and descriptor name the same
// agent format ID (an absent descriptor value means markdown).
func TestAgentFormatMatchesDescriptor(t *testing.T) {
	desc := opencode.DescriptorForTesting(t)

	entry, ok := commonharness.LookupCLIHarness(commonharness.HarnessIDOpenCode)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found in shared catalog", commonharness.HarnessIDOpenCode)
	}

	want := desc.AgentFormatID
	if want == "" {
		want = "markdown"
	}
	if entry.AgentFormatID != want {
		t.Errorf("catalog/YAML drift for opencode agent format:\n  shared catalog AgentFormatID = %q\n  YAML agent_format_id = %q\n  Update catalog.go to match the YAML (or vice versa).",
			entry.AgentFormatID, want)
	}
}

// TestAgentExtensionMatchesDescriptor asserts catalog and descriptor name the same
// agent file extension.
func TestAgentExtensionMatchesDescriptor(t *testing.T) {
	desc := opencode.DescriptorForTesting(t)

	entry, ok := commonharness.LookupCLIHarness(commonharness.HarnessIDOpenCode)
	if !ok {
		t.Fatalf("LookupCLIHarness(%q): not found in shared catalog", commonharness.HarnessIDOpenCode)
	}

	yamlValue := desc.Extensions[domain.ArtifactAgent]
	if yamlValue == "" {
		t.Fatal("opencode descriptor extensions.agent is empty: descriptor may be malformed")
	}
	if entry.AgentExtension != yamlValue {
		t.Errorf("catalog/YAML drift for opencode agent extension:\n  shared catalog AgentExtension = %q\n  YAML extensions.agent = %q\n  Update catalog.go to match the YAML (or vice versa).",
			entry.AgentExtension, yamlValue)
	}
}
