package domain_test

// Tests for the per-harness built-in tool names used by the missing-contact-tools hint.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func builtinContains(h domain.Harness, tool string) bool {
	for _, name := range domain.HarnessBuiltinTools(h) {
		if name == tool {
			return true
		}
	}
	return false
}

func TestHarnessBuiltinTools_ClaudeCode_ListsItsOwnTools(t *testing.T) {
	for _, tool := range []string{"Bash", "Read", "Write", "Edit", "Agent", "AskUserQuestion", "SubagentHandback"} {
		if !builtinContains(domain.HarnessClaudeCode, tool) {
			t.Errorf("claude-code built-in tools lack %q", tool)
		}
	}
}

func TestHarnessBuiltinTools_ClaudeCode_DoesNotListMcpTools(t *testing.T) {
	if builtinContains(domain.HarnessClaudeCode, "mcp__contact-user__ask_user_questions") {
		t.Error("an MCP tool must not be a built-in tool")
	}
}

func TestHarnessBuiltinTools_OpenCode_IsNotEmptyAndLacksClaudeCodeTools(t *testing.T) {
	if len(domain.HarnessBuiltinTools(domain.HarnessOpenCode)) == 0 {
		t.Error("opencode built-in tools are empty, want its captured list")
	}
	if builtinContains(domain.HarnessOpenCode, "Bash") {
		t.Error("opencode built-in tools list claude-code's Bash")
	}
}

func TestHarnessBuiltinTools_VSCodeGHCP_IsEmpty(t *testing.T) {
	if got := domain.HarnessBuiltinTools(domain.HarnessVSCodeGHCP); len(got) != 0 {
		t.Errorf("vscode-ghcp built-in tools = %v, want none", got)
	}
}

func TestHarnessBuiltinTools_UnknownHarness_IsTheUnionOfAllLists(t *testing.T) {
	if !builtinContains("", "Bash") {
		t.Error("unknown harness built-in tools lack claude-code's Bash")
	}
}
