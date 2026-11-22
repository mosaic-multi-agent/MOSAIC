package main

// tool_mappings_main_scan_test.go verifies that main.go builds its registry options through
// registryOptions, so the real program uses the refreshing ToolMappings hook rather than a
// one-time merge of the configs loaded at startup.

import (
	"os"
	"strings"
	"testing"
)

func TestMain_BuildsRegistryOptionsThroughRegistryOptions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, "registryOptions(") {
		t.Error("main.go does not call registryOptions, so the refreshing ToolMappings hook is not wired")
	}
	if strings.Contains(text, "config.EffectiveToolMappings(") {
		t.Error("main.go still merges the tool mappings inline (a one-time merge of the startup configs)")
	}
}
