package harness_test

// catalog_format_test.go covers the agent file format data of the CLI harness
// catalog: every entry names a known agent format ID and a non-empty agent file
// extension, and the three existing harnesses carry their expected values.

import (
	"strings"
	"testing"

	"mosaic-common/agentformat/formatid"
	"mosaic-common/harness"
)

func TestCatalogEntries_AgentFormatIDIsKnown(t *testing.T) {
	for _, e := range harness.CLIHarnesses() {
		if !formatid.IsKnown(formatid.ID(e.AgentFormatID)) {
			t.Errorf("catalog entry %q: AgentFormatID %q is not a known format ID", e.ID, e.AgentFormatID)
		}
	}
}

func TestCatalogEntries_AgentExtensionIsDotted(t *testing.T) {
	for _, e := range harness.CLIHarnesses() {
		if e.AgentExtension == "" {
			t.Errorf("catalog entry %q: AgentExtension is empty", e.ID)
			continue
		}
		if !strings.HasPrefix(e.AgentExtension, ".") {
			t.Errorf("catalog entry %q: AgentExtension %q does not start with a dot", e.ID, e.AgentExtension)
		}
	}
}

func TestCatalogEntries_ExistingHarnessFormatData(t *testing.T) {
	want := map[string]struct{ format, ext string }{
		harness.HarnessIDClaudeCode: {string(formatid.Markdown), ".md"},
		harness.HarnessIDOpenCode:   {string(formatid.Markdown), ".md"},
		harness.HarnessIDGHCPCLI:    {string(formatid.Markdown), ".agent.md"},
		harness.HarnessIDCodex:      {string(formatid.CodexTOML), ".toml"},
		harness.HarnessIDOpenCodeV2: {string(formatid.Markdown), ".md"},
	}
	for id, w := range want {
		e, ok := harness.LookupCLIHarness(id)
		if !ok {
			t.Errorf("LookupCLIHarness(%q): not found", id)
			continue
		}
		if e.AgentFormatID != w.format {
			t.Errorf("%s AgentFormatID = %q, want %q", id, e.AgentFormatID, w.format)
		}
		if e.AgentExtension != w.ext {
			t.Errorf("%s AgentExtension = %q, want %q", id, e.AgentExtension, w.ext)
		}
	}
}
