package analysis_test

// Tests for the hint that MCP contact tools may be missing from the settings:
// set when the run has tool names that are neither configured (user-interaction
// or write-class) nor built in for the actor's harness.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func wtAssertHint(t *testing.T, got domain.WallTimeSplit, wantHint bool, wantTools []string) {
	t.Helper()
	if got.MissingContactToolsHint != wantHint {
		t.Errorf("hint = %v, want %v (unconfigured tools %v)", got.MissingContactToolsHint, wantHint, got.UnconfiguredTools)
	}
	if !rmEqualStrings(got.UnconfiguredTools, wantTools) {
		t.Errorf("unconfigured tools = %v, want %v", got.UnconfiguredTools, wantTools)
	}
}

func TestWallTimeHint_UnconfiguredToolWithNoWaiting_SetsTheHint(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", "mcp__contact-user__ask_user_questions", 10, 20)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, true, []string{"mcp__contact-user__ask_user_questions"})
}

func TestWallTimeHint_UnconfiguredToolBesideWaiting_SetsTheHint(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 10, 20), wtTool("c1", "mcp__x__y", 30, 40)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 10, 0, 90, 0)
	wtAssertHint(t, got, true, []string{"mcp__x__y"})
}

func TestWallTimeHint_OnlyBuiltInAndConfiguredTools_LeavesTheHintOff(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{
		wtTool("c1", "Bash", 1, 2), wtTool("c2", "Read", 3, 4), wtTool("c3", ask, 10, 20),
	}
	spec.agents = []wtAgent{{id: "a#1", start: 30, end: 40, tools: []domain.ToolCall{wtTool("c4", "Write", 31, 32)}}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 10, 10, 80, 0)
	wtAssertHint(t, got, false, nil)
}

// Bash is neither user-interaction nor write-class, so only the claude-code
// built-in list (a non-empty list is required by the domain tests) keeps the
// hint off here.
func TestWallTimeHint_BuiltInToolAlone_LeavesTheHintOff(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", "Bash", 1, 2)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, false, nil)
}

func TestWallTimeHint_DefaultUserInteractionToolRemovedFromSettings_IsStillBuiltIn(t *testing.T) {
	settings := domain.DefaultAnalysisSettings()
	settings.UserInteractionTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {"mcp__c__*"}}
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", ask, 10, 20)}

	got := wtSplitWith(t, spec, settings)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, false, nil)
}

func TestWallTimeHint_ConfiguredPatternRemovedFromSettings_BecomesUnconfigured(t *testing.T) {
	settings := domain.DefaultAnalysisSettings()
	settings.UserInteractionTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {ask}}
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", "mcp__contact-user__ask_user_questions", 10, 20)}

	got := wtSplitWith(t, spec, settings)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, true, []string{"mcp__contact-user__ask_user_questions"})
}

func TestWallTimeHint_ToolMatchedByAUserInteractionPattern_IsNotUnconfigured(t *testing.T) {
	settings := domain.DefaultAnalysisSettings()
	settings.UserInteractionTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {ask, "mcp__contact-user__*"}}
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", "mcp__contact-user__ask_user_questions", 10, 20)}

	got := wtSplitWith(t, spec, settings)

	wtAssertSplit(t, got, 10, 0, 90, 0)
	wtAssertHint(t, got, false, nil)
}

func TestWallTimeHint_ToolMatchedByAWriteToolSetting_IsNotUnconfigured(t *testing.T) {
	settings := domain.DefaultAnalysisSettings()
	settings.WriteTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {"mcp__fs__write_*"}}
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", "mcp__fs__write_file", 10, 20)}

	got := wtSplitWith(t, spec, settings)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, false, nil)
}

func TestWallTimeHint_UnconfiguredNames_AreDistinctAndSortedAcrossActors(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", "mcp__b__tool", 1, 2), wtTool("c2", "mcp__b__tool", 3, 4)}
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 40, tools: []domain.ToolCall{
		wtTool("c3", "mcp__a__tool", 11, 12), wtTool("c4", "mcp__b__tool", 13, 14),
	}}}

	got := wtSplit(t, spec)

	wtAssertHint(t, got, true, []string{"mcp__a__tool", "mcp__b__tool"})
}

func TestWallTimeHint_ToolNameBuiltInForAnotherHarnessOnly_IsUnconfigured(t *testing.T) {
	// Bash is built in for claude-code, but this run's only harness is opencode,
	// whose built-in list is non-empty and lacks Bash (domain built-in tests).
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessOpenCode}
	spec.orchTools = []domain.ToolCall{wtTool("c1", "Bash", 1, 2)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, true, []string{"Bash"})
}

func TestWallTimeHint_HarnessWithEmptyBuiltInList_IsSkipped(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessVSCodeGHCP}
	spec.orchTools = []domain.ToolCall{wtTool("c1", "some_unknown_tool", 1, 2)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, false, nil)
}

func TestWallTimeHint_ToolCallWithoutName_IsIgnored(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{{CallID: "c1", EndedAt: ctxAt(5), Pairing: domain.ToolCallEndOnly}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	wtAssertHint(t, got, false, nil)
}
