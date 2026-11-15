package codex_test

// Tests for the Codex module's capability settings: web search level, network access for
// fetching, and the notice for a read-only agent whose web_fetch is withheld.

import (
	"slices"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/codex"
)

const (
	webSearchKey = "web_search"
	networkKey   = "sandbox_workspace_write"
	agentKey     = "test-agent"
)

// capabilityTools runs the module's Tools for the given generic tools.
func capabilityTools(t *testing.T, generic ...string) domain.ToolResult {
	t.Helper()
	res, err := newModuleFromFrozen(t).Tools(domain.ToolRequest{AgentKey: agentKey, Generic: generic})
	if err != nil {
		t.Fatalf("Tools(%v): %v", generic, err)
	}
	return res
}

// fieldByKey returns the emitted field with the given key.
func fieldByKey(res domain.ToolResult, key string) (domain.FrontmatterField, bool) {
	for _, f := range res.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return domain.FrontmatterField{}, false
}

// networkAccess reports whether the result carries a sandbox_workspace_write table that
// sets network_access to true.
func networkAccess(res domain.ToolResult) bool {
	f, ok := fieldByKey(res, networkKey)
	if !ok || f.Value.Kind != domain.KindMapping {
		return false
	}
	for _, p := range f.Value.Pairs {
		if p.Key == "network_access" {
			return p.Value.Kind == domain.KindScalar && p.Value.Scalar == "true"
		}
	}
	return false
}

func withheldGaps(res domain.ToolResult) []domain.Gap {
	var out []domain.Gap
	for _, g := range res.Gaps {
		if g.Kind == domain.GapToolCapabilityWithheld {
			out = append(out, g)
		}
	}
	return out
}

func TestWebSearch_WithWebSearchTool_IsLive(t *testing.T) {
	for _, generic := range [][]string{
		{"web_search"},
		{"file_read", "web_search", "terminal"},
	} {
		// Act
		res := capabilityTools(t, generic...)

		// Assert
		f, ok := fieldByKey(res, webSearchKey)
		if !ok {
			t.Fatalf("tools %v: web_search field absent, want live", generic)
		}
		if f.Value.Kind != domain.KindScalar || f.Value.Scalar != "live" {
			t.Errorf("tools %v: web_search = %+v, want scalar live", generic, f.Value)
		}
	}
}

func TestWebSearch_WithoutWebSearchTool_IsDisabled(t *testing.T) {
	for _, generic := range [][]string{
		nil,
		{"file_read"},
		{"web_fetch", "terminal"},
		{"unknown_tool"},
	} {
		// Act
		res := capabilityTools(t, generic...)

		// Assert
		f, ok := fieldByKey(res, webSearchKey)
		if !ok {
			t.Fatalf("tools %v: web_search field absent, want disabled", generic)
		}
		if f.Value.Kind != domain.KindScalar || f.Value.Scalar != "disabled" {
			t.Errorf("tools %v: web_search = %+v, want scalar disabled", generic, f.Value)
		}
	}
}

func TestWebSearch_PlaceholderRequest_IsDisabled(t *testing.T) {
	// Arrange / Act
	res, err := newModuleFromFrozen(t).Tools(domain.ToolRequest{AgentKey: agentKey, Placeholder: "{tool-permissions}"})
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}

	// Assert
	f, ok := fieldByKey(res, webSearchKey)
	if !ok || f.Value.Scalar != "disabled" {
		t.Errorf("placeholder request: web_search = %+v (present=%v), want disabled", f.Value, ok)
	}
}

func TestNetwork_WriteCapableAgentWithWebFetch_GetsNetworkAccess(t *testing.T) {
	for _, generic := range [][]string{
		{"file_write", "web_fetch"},
		{"file_edit", "web_fetch"},
		{"web_fetch", "terminal"}, // sandbox is workspace-write because of terminal alone
		{"web_fetch", "subagent"},
	} {
		// Act
		res := capabilityTools(t, generic...)

		// Assert
		if mode, _ := sandboxModeFromResult(res); mode != sandboxModeWorkspaceWrite {
			t.Fatalf("tools %v: sandbox_mode = %q, want %q", generic, mode, sandboxModeWorkspaceWrite)
		}
		if !networkAccess(res) {
			t.Errorf("tools %v: no sandbox_workspace_write.network_access = true; fields: %+v", generic, res.Fields)
		}
		if gaps := withheldGaps(res); len(gaps) != 0 {
			t.Errorf("tools %v: got withheld-capability gaps %+v, want none", generic, gaps)
		}
	}
}

func TestNetwork_ReadOnlyAgentWithWebFetch_GetsNoNetworkAndOneGap(t *testing.T) {
	// Arrange / Act
	res := capabilityTools(t, "file_read", "web_fetch")

	// Assert
	if mode, _ := sandboxModeFromResult(res); mode != sandboxModeReadOnly {
		t.Fatalf("sandbox_mode = %q, want %q", mode, sandboxModeReadOnly)
	}
	if _, ok := fieldByKey(res, networkKey); ok {
		t.Error("read-only agent has a sandbox_workspace_write field, want none")
	}
	gaps := withheldGaps(res)
	if len(gaps) != 1 {
		t.Fatalf("withheld-capability gaps = %+v, want exactly one", gaps)
	}
	if gaps[0].Subject != "web_fetch" || gaps[0].Owner != agentKey {
		t.Errorf("gap subject/owner = %q/%q, want web_fetch/%q", gaps[0].Subject, gaps[0].Owner, agentKey)
	}
	if gaps[0].Detail == "" {
		t.Error("gap Detail is empty, want an explanation")
	}
}

func TestNetwork_TerminalAlone_GrantsNoNetwork(t *testing.T) {
	// Arrange / Act
	res := capabilityTools(t, "terminal", "file_read")

	// Assert
	if mode, _ := sandboxModeFromResult(res); mode != sandboxModeWorkspaceWrite {
		t.Fatalf("sandbox_mode = %q, want %q", mode, sandboxModeWorkspaceWrite)
	}
	if _, ok := fieldByKey(res, networkKey); ok {
		t.Error("terminal alone produced a sandbox_workspace_write field, want none")
	}
	if len(res.Gaps) != 0 {
		t.Errorf("terminal alone produced gaps %+v, want none", res.Gaps)
	}
}

func TestNetwork_NoWebFetch_NoNetworkNoGap(t *testing.T) {
	for _, generic := range [][]string{nil, {"file_write"}, {"file_read", "web_search"}} {
		// Act
		res := capabilityTools(t, generic...)

		// Assert
		if _, ok := fieldByKey(res, networkKey); ok {
			t.Errorf("tools %v: sandbox_workspace_write present, want none", generic)
		}
		if len(res.Gaps) != 0 {
			t.Errorf("tools %v: gaps %+v, want none", generic, res.Gaps)
		}
	}
}

func TestNetwork_PlaceholderRequest_GetsNoNetwork(t *testing.T) {
	// Arrange / Act
	res, err := newModuleFromFrozen(t).Tools(domain.ToolRequest{AgentKey: agentKey, Placeholder: "{tool-permissions}"})
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}

	// Assert
	if _, ok := fieldByKey(res, networkKey); ok {
		t.Error("placeholder request produced sandbox_workspace_write, want none (web_fetch not requested)")
	}
}

func TestCapabilityFields_AreEmittedInDeclaredOrder(t *testing.T) {
	// Arrange / Act
	res := capabilityTools(t, "file_write", "web_fetch", "web_search")

	// Assert
	var keys []string
	for _, f := range res.Fields {
		keys = append(keys, f.Key)
	}
	want := []string{"sandbox_mode", webSearchKey, networkKey}
	if !slices.Equal(keys, want) {
		t.Errorf("field keys = %v, want %v", keys, want)
	}
}

func TestFrontmatter_KeyOrder_PlacesCapabilityKeysAfterSandboxMode(t *testing.T) {
	// Arrange
	order := codex.DescriptorForTesting(t).Frontmatter.KeyOrder

	// Act
	sandbox := slices.Index(order, "sandbox_mode")
	search := slices.Index(order, webSearchKey)
	network := slices.Index(order, networkKey)

	// Assert
	if sandbox < 0 || search <= sandbox || network <= search {
		t.Errorf("key_order = %v, want sandbox_mode, web_search, sandbox_workspace_write in that order", order)
	}
}

func TestCapabilities_OtherToolResultsKeepNoGapsForUnmapped(t *testing.T) {
	// Arrange / Act: an unknown tool is reported through Resolutions, not as a capability gap.
	res := capabilityTools(t, "unknown_tool")

	// Assert
	if len(withheldGaps(res)) != 0 {
		t.Errorf("unknown tool produced withheld-capability gaps %+v, want none", res.Gaps)
	}
}
