package domain_test

// Tests for the opencode-v2 harness identity, built-in tools, default tool
// lists and subagent launch tool.
//
// Rules under test:
//   - opencode-v2 is a recognised harness; an unknown value still is not.
//   - Its built-in tools are the V2 list in the documented order, and the
//     V1 opencode list is unchanged.
//   - Its default write tools are write, edit, patch and its default
//     user-interaction tool is question, through the defaults and the settings.
//   - The unknown-harness unions gain the V2 names.
//   - "subagent" launches a subagent, in any ASCII case.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func equalToolNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHarnessIsRecognised_OpenCodeV2_IsRecognised(t *testing.T) {
	if !domain.HarnessOpenCodeV2.IsRecognised() {
		t.Error("opencode-v2 must be a recognised harness")
	}
	if domain.Harness("opencode-v3").IsRecognised() {
		t.Error("an unknown harness value must stay unrecognised")
	}
}

func TestHarnessBuiltinTools_OpenCodeV2_ListsTheV2ToolsInOrder(t *testing.T) {
	want := []string{
		"shell", "read", "write", "edit", "patch", "glob", "grep", "webfetch",
		"websearch", "question", "skill", "subagent", "execute", "todowrite",
	}

	got := domain.HarnessBuiltinTools(domain.HarnessOpenCodeV2)

	if !equalToolNames(got, want) {
		t.Errorf("opencode-v2 built-in tools = %v, want %v", got, want)
	}
}

func TestHarnessBuiltinTools_OpenCodeV1_IsUnchangedByV2(t *testing.T) {
	want := []string{
		"bash", "read", "write", "edit", "patch", "apply_patch", "glob", "grep",
		"list", "todowrite", "todoread", "webfetch", "task", "question", "skill",
	}

	got := domain.HarnessBuiltinTools(domain.HarnessOpenCode)

	if !equalToolNames(got, want) {
		t.Errorf("opencode built-in tools = %v, want the unchanged V1 list %v", got, want)
	}
}

func TestHarnessBuiltinTools_UnknownHarness_UnionIncludesV2Names(t *testing.T) {
	for _, tool := range []string{"shell", "subagent", "execute"} {
		if !builtinContains("", tool) {
			t.Errorf("unknown-harness union lacks the opencode-v2 tool %q", tool)
		}
	}
}

func TestDefaultTools_OpenCodeV2_WriteAndUserInteractionTools(t *testing.T) {
	if got := domain.DefaultWriteTools(domain.HarnessOpenCodeV2); !equalToolNames(got, []string{"write", "edit", "patch"}) {
		t.Errorf("DefaultWriteTools(opencode-v2) = %v, want [write edit patch]", got)
	}
	if got := domain.DefaultUserInteractionTools(domain.HarnessOpenCodeV2); !equalToolNames(got, []string{"question"}) {
		t.Errorf("DefaultUserInteractionTools(opencode-v2) = %v, want [question]", got)
	}
}

func TestDefaultAnalysisSettings_OpenCodeV2_CarriesBothDefaults(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	for _, tool := range []string{"write", "edit", "patch"} {
		if !s.IsWriteTool(domain.HarnessOpenCodeV2, tool) {
			t.Errorf("IsWriteTool(opencode-v2, %q) = false, want true by default", tool)
		}
	}
	if !s.IsUserInteractionTool(domain.HarnessOpenCodeV2, "question") {
		t.Error("IsUserInteractionTool(opencode-v2, question) = false, want true by default")
	}
}

func TestDefaultAnalysisSettings_OpenCodeV2_DoesNotChangeV1Defaults(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	if s.IsWriteTool(domain.HarnessOpenCode, "write") {
		t.Error("opencode (V1) must keep only its own write tools; write is a V2 default")
	}
	if !s.IsWriteTool(domain.HarnessOpenCode, "apply_patch") {
		t.Error("opencode (V1) must still treat apply_patch as a write tool")
	}
	if s.IsWriteTool(domain.HarnessOpenCodeV2, "apply_patch") {
		t.Error("opencode-v2 must not inherit the V1 write tool apply_patch")
	}
}

func TestIsWriteTool_UnknownHarness_UnionIncludesV2Names(t *testing.T) {
	s := domain.DefaultAnalysisSettings()

	for _, tool := range []string{"write", "patch"} {
		if !s.IsWriteTool("", tool) {
			t.Errorf("IsWriteTool(\"\", %q) = false, want true (union includes opencode-v2)", tool)
		}
	}
}

func TestIsSubagentLaunchTool_Subagent_AnyAsciiCase(t *testing.T) {
	for _, name := range []string{"subagent", "Subagent", "SUBAGENT"} {
		if !domain.IsSubagentLaunchTool(name) {
			t.Errorf("IsSubagentLaunchTool(%q) = false, want true", name)
		}
	}
}

func TestIsSubagentLaunchTool_NearMissesOfSubagent_False(t *testing.T) {
	for _, name := range []string{"subagents", "sub_agent", "subagent ", "mcp__server__subagent"} {
		if domain.IsSubagentLaunchTool(name) {
			t.Errorf("IsSubagentLaunchTool(%q) = true, want false", name)
		}
	}
}
