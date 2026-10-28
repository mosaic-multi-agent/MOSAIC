package domain_test

// Tests for the built-in analysis settings and tool-name matching.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestDefaultAnalysisSettings_CompactionDropPercentIs30(t *testing.T) {
	got := domain.DefaultAnalysisSettings().CompactionDropPercent
	if got != 30 {
		t.Errorf("CompactionDropPercent = %d, want 30", got)
	}
}

func TestDefaultAnalysisSettings_WriteToolsPerHarness(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	cases := []struct {
		harness domain.Harness
		tools   []string
	}{
		{domain.HarnessClaudeCode, []string{"Write", "Edit", "MultiEdit", "NotebookEdit"}},
		{domain.HarnessGHCPCLI, []string{"create", "edit"}},
		{domain.HarnessOpenCode, []string{"apply_patch"}},
		{domain.HarnessVSCodeGHCP, []string{
			"create_file", "replace_string_in_file", "multi_replace_string_in_file",
			"insert_edit_into_file", "apply_patch", "edit_notebook_file",
			"edit/createFile", "edit/editFiles",
		}},
	}
	for _, c := range cases {
		for _, tool := range c.tools {
			if !s.IsWriteTool(c.harness, tool) {
				t.Errorf("IsWriteTool(%q, %q) = false, want true by default", c.harness, tool)
			}
		}
	}
}

func TestDefaultAnalysisSettings_VSCodeDirectoryCreationIsNotWriteClass(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	if !s.IsWriteTool(domain.HarnessVSCodeGHCP, "create_file") {
		t.Fatal("precondition: vscode-ghcp defaults must contain create_file")
	}
	for _, tool := range []string{"edit/createDirectory", "create_directory"} {
		if s.IsWriteTool(domain.HarnessVSCodeGHCP, tool) {
			t.Errorf("IsWriteTool(vscode-ghcp, %q) = true, want false (creates no file content)", tool)
		}
	}
}

func TestDefaultAnalysisSettings_UserInteractionToolsPerHarness(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	cases := []struct {
		harness domain.Harness
		tools   []string
	}{
		{domain.HarnessClaudeCode, []string{"AskUserQuestion"}},
		{domain.HarnessGHCPCLI, []string{"ask_user"}},
		{domain.HarnessOpenCode, []string{"question"}},
		{domain.HarnessVSCodeGHCP, []string{"vscode/askQuestions", "askQuestions", "vscode_askQuestions"}},
	}
	for _, c := range cases {
		for _, tool := range c.tools {
			if !s.IsUserInteractionTool(c.harness, tool) {
				t.Errorf("IsUserInteractionTool(%q, %q) = false, want true by default", c.harness, tool)
			}
		}
	}
}

func TestIsWriteTool_UsesOnlyTheNamedHarnessList(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	if !s.IsWriteTool(domain.HarnessGHCPCLI, "create") {
		t.Fatal("precondition: ghcp-cli create must be write-class by default")
	}
	if s.IsWriteTool(domain.HarnessClaudeCode, "create") {
		t.Error("claude-code must not match a ghcp-cli tool name")
	}
}

func TestIsWriteTool_UnknownHarnessUsesUnionOfAllLists(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	for _, tool := range []string{"Write", "create", "apply_patch", "edit/createFile"} {
		if !s.IsWriteTool("", tool) {
			t.Errorf("IsWriteTool(\"\", %q) = false, want true (union of all harness lists)", tool)
		}
	}
	if s.IsWriteTool("", "Bash") {
		t.Error("IsWriteTool(\"\", Bash) = true, want false")
	}
}

func TestIsUserInteractionTool_UnknownHarnessUsesUnionOfAllLists(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	for _, tool := range []string{"AskUserQuestion", "ask_user", "question"} {
		if !s.IsUserInteractionTool("", tool) {
			t.Errorf("IsUserInteractionTool(\"\", %q) = false, want true", tool)
		}
	}
}

func TestToolMatching_EmptyToolNameNeverMatches(t *testing.T) {
	s := domain.AnalysisSettings{
		WriteTools:           map[domain.Harness][]string{domain.HarnessClaudeCode: {"*"}},
		UserInteractionTools: map[domain.Harness][]string{domain.HarnessClaudeCode: {"*"}},
	}
	if !s.IsWriteTool(domain.HarnessClaudeCode, "anything") {
		t.Fatal("precondition: \"*\" must match a non-empty name")
	}
	if s.IsWriteTool(domain.HarnessClaudeCode, "") || s.IsWriteTool("", "") {
		t.Error("IsWriteTool must never match an empty tool name, even against \"*\"")
	}
	if s.IsUserInteractionTool(domain.HarnessClaudeCode, "") || s.IsUserInteractionTool("", "") {
		t.Error("IsUserInteractionTool must never match an empty tool name, even against \"*\"")
	}
}

func TestToolMatching_GlobAndExactRules(t *testing.T) {
	s := domain.AnalysisSettings{
		UserInteractionTools: map[domain.Harness][]string{
			domain.HarnessClaudeCode: {"mcp__contact-user__*", "a*c", "x.y", "Exact"},
		},
	}
	cases := []struct {
		tool string
		want bool
	}{
		{"mcp__contact-user__ask_user_questions", true},
		{"mcp__contact-user__", true},      // '*' matches an empty run
		{"xmcp__contact-user__ask", false}, // whole-name match, not substring
		{"abc", true},
		{"ac", true},
		{"abXc", true},
		{"abcd", false},
		{"x.y", true},
		{"xzy", false}, // '.' is literal, not a regular-expression wildcard
		{"Exact", true},
		{"exact", false}, // case-sensitive
		{"ExactX", false},
	}
	for _, c := range cases {
		if got := s.IsUserInteractionTool(domain.HarnessClaudeCode, c.tool); got != c.want {
			t.Errorf("IsUserInteractionTool(claude-code, %q) = %v, want %v", c.tool, got, c.want)
		}
	}
}

func TestToolMatching_HarnessWithoutListMatchesNothing(t *testing.T) {
	s := domain.DefaultAnalysisSettings()
	if !s.IsWriteTool(domain.HarnessClaudeCode, "Write") {
		t.Fatal("precondition: claude-code Write must be write-class by default")
	}
	if s.IsWriteTool("some-other-harness", "Write") {
		t.Error("a harness with no list must not match any tool")
	}
}
