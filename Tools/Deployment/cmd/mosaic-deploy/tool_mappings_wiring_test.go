package main

// tool_mappings_wiring_test.go verifies that the registry options built by the composition root
// carry the refreshing ToolMappings hook: an entry saved after the options were built is applied
// by the hook of those options, so a registry rebuilt later in the same process sees it.

import (
	"testing"

	"mosaic-deploy/internal/config"
)

func TestRegistryOptions_ToolMappings_SeesAnEntrySavedAfterTheOptionsWereBuilt(t *testing.T) {
	tool, user := newHookStores(t) // the user config file does not exist yet
	opts := registryOptions(t.TempDir(), false, tool, user, config.ToolConfig{}, config.UserConfig{})
	if opts.ToolMappings == nil {
		t.Fatal("the registry options carry no ToolMappings hook")
	}
	if _, ok := findHookMapping(opts.ToolMappings(hookHarness, hookDeclared()), "bash"); ok {
		t.Fatal("the hook returned a mapping for bash before any was saved")
	}
	if err := user.WithLock(func() error { return user.PutToolDestination(hookHarness, hookCustomMapping()) }); err != nil {
		t.Fatalf("saving the first entry: %v", err)
	}

	got := opts.ToolMappings(hookHarness, hookDeclared())

	m, ok := findHookMapping(got, "bash")
	if !ok || len(m.Destinations) != 1 || len(m.Destinations[0].Names) != 1 || m.Destinations[0].Names[0] != "mcp__shell__*" {
		t.Errorf("mapping for bash after the save = %+v (found %v), want the saved entry", m, ok)
	}
	if _, ok := findHookMapping(got, "read"); !ok {
		t.Error("the declared mapping for read was lost")
	}
}
