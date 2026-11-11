package app_test

// deployed_tools_mcp_forms_test.go verifies that the deployed tool conversion reads a
// whole-server MCP name written in the source harness's own syntax (as Deploy writes it)
// and writes it in the target harness's syntax, for every built-in source harness.

import (
	"slices"
	"testing"
)

func TestConvertDeployedTools_WholeServerNameInSourceSyntaxIsWrittenInTargetSyntax(t *testing.T) {
	cases := []struct {
		name string
		src  string
		in   string
	}{
		{"claude-code", harnessClaude, "mcp__alpha__*"},
		{"opencode", harnessOpen, "alpha_*"},
		{"opencode loose glob", harnessOpen, "alpha*"},
		{"ghcp-cli", harnessCLI, "alpha/*"},
		{"vscode-ghcp", harnessVSCode, "alpha/*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Every source converts to Claude Code (Claude Code itself to OpenCode), whose own
			// form differs from the source forms, so a pass-through cannot satisfy the test.
			target, want := harnessClaude, "mcp__alpha__*"
			if tc.src == harnessClaude {
				target, want = harnessOpen, "alpha_*"
			}
			got := convertTools(t, tc.src, target, []string{tc.in})

			assertStrings(t, "TargetTools", []string{want}, got.TargetTools)
			assertStrings(t, "Dropped", nil, droppedNames(got))
			assertCustomTools(t, nil, got.CustomTools)
			if slices.Contains(got.Generic, "alpha") {
				t.Errorf("the server must not be a generic tool, got Generic %v", got.Generic)
			}
		})
	}
}

func TestConvertDeployedTools_OpenCodeNormalisedServerNameIsKeptWhenWrittenForClaude(t *testing.T) {
	// "my server" is written by OpenCode as my_server_*; read back it is the server my_server.
	got := convertTools(t, harnessOpen, harnessClaude, []string{"my_server_*"})

	assertStrings(t, "TargetTools", []string{"mcp__my_server__*"}, got.TargetTools)
}
