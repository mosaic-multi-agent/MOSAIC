package analysis_test

// Tests for what counts as waiting on the user: user-interaction tool matching
// per harness (exact and glob), the counted tool names, open-ended waits and
// notifications.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func wtCountedKeys(got domain.WallTimeSplit) []string {
	out := make([]string, len(got.CountedTools))
	for i, c := range got.CountedTools {
		out[i] = c.Tool + "|" + string(c.Harness)
	}
	return out
}

func TestWallTime_NoUserInteractionCalls_WaitingIsZeroWithNoCountedTools(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("c1", "Bash", 10, 20)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.CountedTools) != 0 || len(got.WaitIntervals) != 0 || len(got.OpenWaits) != 0 {
		t.Errorf("counted %v, wait intervals %v, open waits %v, want none", got.CountedTools, got.WaitIntervals, got.OpenWaits)
	}
}

func TestWallTime_ExactToolName_IsMatchedPerActorHarness(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessClaudeCode, domain.HarnessOpenCode}
	spec.orchHarness = domain.HarnessClaudeCode
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 10, 20), wtTool("n1", "question", 30, 40)}
	spec.agents = []wtAgent{{id: "a#1", start: 50, end: 90, harness: domain.HarnessOpenCode, tools: []domain.ToolCall{
		wtTool("w2", "question", 60, 70), wtTool("n2", ask, 75, 85),
	}}}

	got := wtSplit(t, spec)

	// claude-code counts AskUserQuestion, not question; opencode the reverse.
	wtAssertIntervals(t, "wait intervals", got.WaitIntervals, [][2]int{{10, 20}, {60, 70}})
	want := []string{ask + "|claude-code", "question|opencode"}
	if keys := wtCountedKeys(got); !rmEqualStrings(keys, want) {
		t.Errorf("counted tools = %v, want %v", keys, want)
	}
}

func TestWallTime_ActorWithoutUsageHarness_TakesTheRunsOnlyHarness(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessOpenCode}
	spec.orchTools = []domain.ToolCall{wtTool("w1", "question", 10, 20), wtTool("n1", ask, 30, 40)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 10, 0, 90, 0)
	if keys := wtCountedKeys(got); !rmEqualStrings(keys, []string{"question|opencode"}) {
		t.Errorf("counted tools = %v, want question for opencode only", keys)
	}
}

func TestWallTime_GlobPatternFromSettings_MatchesMcpContactTools(t *testing.T) {
	settings := domain.DefaultAnalysisSettings()
	settings.UserInteractionTools = map[domain.Harness][]string{
		domain.HarnessClaudeCode: {ask, "mcp__contact-user__*"},
	}
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{
		wtTool("w1", "mcp__contact-user__ask_user_questions", 10, 20),
		wtTool("n1", "mcp__other__ask", 30, 40),
	}

	got := wtSplitWith(t, spec, settings)

	wtAssertSplit(t, got, 10, 0, 90, 0)
	want := []string{"mcp__contact-user__ask_user_questions|claude-code"}
	if keys := wtCountedKeys(got); !rmEqualStrings(keys, want) {
		t.Errorf("counted tools = %v, want %v", keys, want)
	}
}

func TestWallTime_CountedTools_ListEachNameWithItsCallCountSorted(t *testing.T) {
	settings := domain.DefaultAnalysisSettings()
	settings.UserInteractionTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {ask, "mcp__c__*"}}
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{
		wtTool("w1", "mcp__c__zed", 5, 6), wtTool("w2", ask, 10, 20),
		wtTool("w3", ask, 30, 40), wtOpenTool("w4", ask, 50),
	}

	got := wtSplitWith(t, spec, settings)

	if len(got.CountedTools) != 2 {
		t.Fatalf("counted tools = %+v, want 2 entries", got.CountedTools)
	}
	first, second := got.CountedTools[0], got.CountedTools[1]
	if first.Tool != ask || first.Calls != 3 || first.Harness != domain.HarnessClaudeCode {
		t.Errorf("first counted tool = %+v, want %s with 3 calls (the open call included)", first, ask)
	}
	if second.Tool != "mcp__c__zed" || second.Calls != 1 {
		t.Errorf("second counted tool = %+v, want mcp__c__zed with 1 call", second)
	}
}

func TestWallTime_OpenUserInteractionCall_IsExcludedFromWaitingAndMarkedOpen(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 10, 20), wtOpenTool("w2", ask, 60)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 10, 0, 90, 0)
	if len(got.OpenWaits) != 1 {
		t.Fatalf("open waits = %+v, want 1", got.OpenWaits)
	}
	w := got.OpenWaits[0]
	if w.Kind != domain.OpenWaitToolCall || w.Tool != ask || !w.At.Equal(ctxAt(60)) || w.Actor.Kind != domain.ActorOrchestrator {
		t.Errorf("open wait = %+v, want an open %s tool call at 60s by the orchestrator", w, ask)
	}
}

func TestWallTime_UserInteractionCallEndingBeforeItStarts_IsOpenEnded(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 40, 30)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.OpenWaits) != 1 || got.OpenWaits[0].Kind != domain.OpenWaitToolCall {
		t.Errorf("open waits = %+v, want one open tool-call wait: the call has no usable duration", got.OpenWaits)
	}
}

func TestWallTime_OpenWaits_AreSortedByTimeWithZeroLast(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtOpenTool("w1", ask, 60)}
	spec.notifs = []domain.Notification{
		wtNotification(domain.Orchestrator(), wtNone, "idle_prompt", ""),
		wtNotification(domain.Orchestrator(), 20, "permission_prompt", ""),
	}

	got := wtSplit(t, spec)

	if len(got.OpenWaits) != 3 {
		t.Fatalf("open waits = %+v, want 3", got.OpenWaits)
	}
	if !got.OpenWaits[0].At.Equal(ctxAt(20)) || !got.OpenWaits[1].At.Equal(ctxAt(60)) || !got.OpenWaits[2].At.IsZero() {
		t.Errorf("open wait times = %v, %v, %v, want 20s, 60s, zero",
			got.OpenWaits[0].At, got.OpenWaits[1].At, got.OpenWaits[2].At)
	}
}

func TestWallTime_PermissionNotificationWithoutId_IsOpenEndedAndNotCounted(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{wtNotification(domain.Orchestrator(), 30, "permission_prompt", "")}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.OpenWaits) != 1 || got.OpenWaits[0].Kind != domain.OpenWaitPermission || !got.OpenWaits[0].At.Equal(ctxAt(30)) {
		t.Errorf("open waits = %+v, want one open permission wait at 30s", got.OpenWaits)
	}
}

func TestWallTime_PermissionNotificationWithResolutionButNoId_IsStillOpenEnded(t *testing.T) {
	n := wtNotification(domain.Orchestrator(), 30, "permission_prompt", "")
	n.Resolution = "allow"
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{n, wtNotification(domain.Orchestrator(), 45, "permission_prompt", "")}

	got := wtSplit(t, spec)

	// Pairing needs the shared notification id; a resolution text alone gives no
	// partner and there is no time-based pairing.
	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.OpenWaits) != 2 || len(got.WaitIntervals) != 0 {
		t.Errorf("open waits = %+v, wait intervals = %v, want 2 open permission waits and no interval", got.OpenWaits, got.WaitIntervals)
	}
}

func TestWallTime_OpenUserInteractionCallOfASubagent_IsOpenWaitOfThatAgentAndNotCounted(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 60, tools: []domain.ToolCall{wtOpenTool("w1", ask, 20)}}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 50, 50, 0)
	if len(got.OpenWaits) != 1 {
		t.Fatalf("open waits = %+v, want 1", got.OpenWaits)
	}
	if w := got.OpenWaits[0]; w.Actor.Kind != domain.ActorAgentInstance || w.Actor.Instance != "a#1" || !w.At.Equal(ctxAt(20)) {
		t.Errorf("open wait = %+v, want an open call of agent a#1 at 20s", w)
	}
}

func TestWallTime_PairedNotificationsOfASubagent_CountAsWaitingNotSubagentTime(t *testing.T) {
	actor := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "a#1", Type: "coder"}
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 60}}
	spec.notifs = []domain.Notification{
		wtNotification(actor, 20, "permission_prompt", "n-1"),
		wtNotification(actor, 30, "permission_resolved", "n-1"),
	}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 10, 40, 50, 0)
}

func TestWallTime_NotificationRequiringResponse_IsAnOpenPermissionWait(t *testing.T) {
	n := wtNotification(domain.Orchestrator(), 30, "elicitation_dialog", "")
	n.RequiresResponse = domain.BoolTrue
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{n}

	got := wtSplit(t, spec)

	if len(got.OpenWaits) != 1 || got.OpenWaits[0].Kind != domain.OpenWaitPermission {
		t.Errorf("open waits = %+v, want one open permission wait", got.OpenWaits)
	}
}

func TestWallTime_NotificationsWithTheSameId_FormAWaitInterval(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{
		wtNotification(domain.Orchestrator(), 10, "permission_prompt", "n-1"),
		wtNotification(domain.Orchestrator(), 25, "permission_resolved", "n-1"),
	}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 15, 0, 85, 0)
	wtAssertIntervals(t, "wait intervals", got.WaitIntervals, [][2]int{{10, 25}})
	if len(got.OpenWaits) != 0 {
		t.Errorf("open waits = %+v, want none: the pair resolves the wait", got.OpenWaits)
	}
}

func TestWallTime_NotificationsWithTheSameId_UseFirstAndLastTimes(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{
		wtNotification(domain.Orchestrator(), 30, "permission_prompt", "n-1"),
		wtNotification(domain.Orchestrator(), 10, "permission_prompt", "n-1"),
		wtNotification(domain.Orchestrator(), 20, "permission_resolved", "n-1"),
	}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 20, 0, 80, 0)
	wtAssertIntervals(t, "wait intervals", got.WaitIntervals, [][2]int{{10, 30}})
}

func TestWallTime_PairedNotificationsOfAnyType_FormAWaitInterval(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{
		wtNotification(domain.Orchestrator(), 40, "elicitation_dialog", "e-1"),
		wtNotification(domain.Orchestrator(), 55, "elicitation_complete", "e-1"),
	}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 15, 0, 85, 0)
	wtAssertIntervals(t, "wait intervals", got.WaitIntervals, [][2]int{{40, 55}})
}

func TestWallTime_GlobStar_MatchesAnyRunOfCharactersIncludingEmpty(t *testing.T) {
	settings := domain.DefaultAnalysisSettings()
	settings.UserInteractionTools = map[domain.Harness][]string{domain.HarnessClaudeCode: {"mcp__x__*"}}
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{
		wtTool("w1", "mcp__x__", 10, 20),
		wtTool("w2", "mcp__x__a__b", 30, 40),
		wtTool("n1", "mcp__y__a", 50, 60),
		wtTool("n2", "mcp__x_a", 70, 80),
	}

	got := wtSplitWith(t, spec, settings)

	wtAssertSplit(t, got, 20, 0, 80, 0)
	want := []string{"mcp__x__|claude-code", "mcp__x__a__b|claude-code"}
	if keys := wtCountedKeys(got); !rmEqualStrings(keys, want) {
		t.Errorf("counted tools = %v, want %v", keys, want)
	}
}

func TestWallTime_PermissionNotificationWithUnmatchedId_IsOpenEnded(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{
		wtNotification(domain.Orchestrator(), 10, "permission_prompt", "n-1"),
		wtNotification(domain.Orchestrator(), 40, "permission_prompt", "n-2"),
	}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.OpenWaits) != 2 {
		t.Errorf("open waits = %+v, want 2: different ids never pair, and there is no time-based pairing", got.OpenWaits)
	}
}

func TestWallTime_IdlePrompt_IsOpenEndedAndNotCounted(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{wtNotification(domain.Orchestrator(), 50, "idle_prompt", "")}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.OpenWaits) != 1 || got.OpenWaits[0].Kind != domain.OpenWaitIdlePrompt {
		t.Errorf("open waits = %+v, want one idle-prompt wait", got.OpenWaits)
	}
}

func TestWallTime_OtherNotificationTypes_AreIgnored(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{wtNotification(domain.Orchestrator(), 50, "auth_success", "")}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.OpenWaits) != 0 {
		t.Errorf("open waits = %+v, want none", got.OpenWaits)
	}
}
