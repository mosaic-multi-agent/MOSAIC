package main

// tool_mappings_hook_toolconfig_test.go verifies that the registry ToolMappings hook also reads
// tool-config.yaml at each call, and falls back to the startup tool config when that file cannot
// be read.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/config"
)

const hookToolConfigWithBash = "tool_destinations:\n" +
	"  claude-code:\n" +
	"    - generic: bash\n" +
	"      destinations:\n" +
	"        - to: main\n" +
	"          names: [\"mcp__shell__*\"]\n"

func TestToolMappingsHook_ToolConfigEntrySavedAfterTheHookWasBuilt_IsSeenAtTheNextCall(t *testing.T) {
	tool, user := newHookStores(t)
	hook := toolMappingsHook(tool, user, config.ToolConfig{}, config.UserConfig{})
	if _, ok := findHookMapping(hook(hookHarness, hookDeclared()), "bash"); ok {
		t.Fatal("the hook returned a mapping for bash before any was saved")
	}
	if err := os.WriteFile(tool.Path(), []byte(hookToolConfigWithBash), 0o644); err != nil {
		t.Fatalf("writing the tool config: %v", err)
	}

	got := hook(hookHarness, hookDeclared())

	m, ok := findHookMapping(got, "bash")
	if !ok || len(m.Destinations) != 1 || len(m.Destinations[0].Names) != 1 || m.Destinations[0].Names[0] != "mcp__shell__*" {
		t.Errorf("mapping for bash after the tool config was written = %+v (found %v), want the entry", m, ok)
	}
	if _, ok := findHookMapping(got, "read"); !ok {
		t.Error("the declared mapping for read was lost")
	}
}

func TestToolMappingsHook_UnreadableToolConfig_FallsBackToTheStartupConfig(t *testing.T) {
	tool, user := newHookStores(t)
	if err := os.MkdirAll(filepath.Dir(tool.Path()), 0o755); err != nil {
		t.Fatalf("creating the config folder: %v", err)
	}
	if err := os.WriteFile(tool.Path(), []byte("tool_destinations: [unterminated"), 0o644); err != nil {
		t.Fatalf("writing the broken file: %v", err)
	}
	startup := config.ToolConfig{ToolDestinations: config.ToolDestinationsByHarness{
		hookHarness: {hookCustomMapping()},
	}}
	hook := toolMappingsHook(tool, user, startup, config.UserConfig{})

	got := hook(hookHarness, hookDeclared())

	if _, ok := findHookMapping(got, "bash"); !ok {
		t.Errorf("mappings = %+v, want the startup tool config's entry for bash when the file cannot be read", got)
	}
}
