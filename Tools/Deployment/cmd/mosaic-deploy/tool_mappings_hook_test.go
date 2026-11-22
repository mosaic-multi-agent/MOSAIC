package main

// tool_mappings_hook_test.go verifies that the registry ToolMappings hook of the composition
// root reads the tool-config and user-config files each time it is called, so a tool_destinations
// entry saved after startup is seen by registries built later in the same process, and that it
// falls back to the configs loaded at startup when a file cannot be read.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
)

const hookHarness = "claude-code"

func hookDeclared() []domain.ToolMapping {
	return []domain.ToolMapping{{
		Generic:      "read",
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"Read"}}},
	}}
}

func hookCustomMapping() domain.ToolMapping {
	return domain.ToolMapping{
		Generic:      "bash",
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"mcp__shell__*"}}},
	}
}

func findHookMapping(list []domain.ToolMapping, generic string) (domain.ToolMapping, bool) {
	for _, m := range list {
		if m.Generic == generic {
			return m, true
		}
	}
	return domain.ToolMapping{}, false
}

func newHookStores(t *testing.T) (config.ToolConfigStore, config.UserConfigStore) {
	t.Helper()
	root := t.TempDir()
	tool := config.NewToolConfigStore(root)
	user := config.NewUserConfigStore(root)
	if err := os.MkdirAll(filepath.Dir(user.Path()), 0o755); err != nil {
		t.Fatalf("creating the config folder: %v", err)
	}
	return tool, user
}

func TestToolMappingsHook_EntrySavedAfterTheHookWasBuilt_IsSeenAtTheNextCall(t *testing.T) {
	tool, user := newHookStores(t)
	hook := toolMappingsHook(tool, user, config.ToolConfig{}, config.UserConfig{})
	before := hook(hookHarness, hookDeclared())
	if _, ok := findHookMapping(before, "bash"); ok {
		t.Fatal("the hook returned a mapping for bash before any was saved")
	}
	if err := user.WithLock(func() error { return user.PutToolDestination(hookHarness, hookCustomMapping()) }); err != nil {
		t.Fatalf("saving the entry: %v", err)
	}

	after := hook(hookHarness, hookDeclared())

	got, ok := findHookMapping(after, "bash")
	if !ok || len(got.Destinations) != 1 || len(got.Destinations[0].Names) != 1 || got.Destinations[0].Names[0] != "mcp__shell__*" {
		t.Errorf("mapping for bash after the save = %+v (found %v), want the saved entry", got, ok)
	}
	if _, ok := findHookMapping(after, "read"); !ok {
		t.Error("the declared mapping for read was lost")
	}
}

func TestToolMappingsHook_EntryForAnotherHarness_IsNotApplied(t *testing.T) {
	tool, user := newHookStores(t)
	hook := toolMappingsHook(tool, user, config.ToolConfig{}, config.UserConfig{})
	other := domain.ToolMapping{
		Generic:      "grep",
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"Grep"}}},
	}
	if err := user.WithLock(func() error {
		if err := user.PutToolDestination("opencode", hookCustomMapping()); err != nil {
			return err
		}
		return user.PutToolDestination(hookHarness, other)
	}); err != nil {
		t.Fatalf("saving the entries: %v", err)
	}

	got := hook(hookHarness, hookDeclared())

	if _, ok := findHookMapping(got, "bash"); ok {
		t.Error("an entry saved for opencode was applied to claude-code")
	}
	if _, ok := findHookMapping(got, "grep"); !ok {
		t.Error("the entry saved for claude-code was not returned; the hook must merge entries per harness")
	}
	if _, ok := findHookMapping(got, "read"); !ok {
		t.Error("the declared mapping for read was lost")
	}
}

func TestToolMappingsHook_MissingUserConfig_IgnoresStaleStartupEntries(t *testing.T) {
	tool, user := newHookStores(t)
	startup := config.UserConfig{ToolDestinations: config.ToolDestinationsByHarness{
		hookHarness: {hookCustomMapping()},
	}}
	hook := toolMappingsHook(tool, user, config.ToolConfig{}, startup)

	got := hook(hookHarness, hookDeclared())

	if _, ok := findHookMapping(got, "read"); !ok {
		t.Error("the declared mapping for read was lost")
	}
	if _, ok := findHookMapping(got, "bash"); ok {
		t.Error("a missing user-config file fell back to stale startup entries")
	}
}

func TestToolMappingsHook_UnreadableUserConfig_FallsBackToTheStartupConfig(t *testing.T) {
	tool, user := newHookStores(t)
	if err := os.WriteFile(user.Path(), []byte("tool_destinations: [unterminated"), 0o644); err != nil {
		t.Fatalf("writing the broken file: %v", err)
	}
	startup := config.UserConfig{ToolDestinations: config.ToolDestinationsByHarness{
		hookHarness: {hookCustomMapping()},
	}}
	hook := toolMappingsHook(tool, user, config.ToolConfig{}, startup)

	got := hook(hookHarness, hookDeclared())

	if _, ok := findHookMapping(got, "bash"); !ok {
		t.Errorf("mappings = %+v, want the startup config's entry for bash when the file cannot be read", got)
	}
}
