package analysis_test

// Tests for the wall-time split: waiting on the user, subagents working,
// orchestrator only and between sessions, each instant counted once in that
// order of precedence, summing to the run's logged span.

import (
	"fmt"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const ask = "AskUserQuestion"

func TestWallTime_OrchestratorOnly_IsTheWholeSpanWithoutInvocationsOrWaits(t *testing.T) {
	spec := newTestWallSpec(0, 100)

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if !got.Span.Start.Equal(ctxAt(0)) || !got.Span.End.Equal(ctxAt(100)) {
		t.Errorf("span = %v..%v, want 0s..100s", got.Span.Start, got.Span.End)
	}
}

func TestWallTime_SpanEqualsTheTimelineAxis(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 120}}

	run := ctxPrice(t, wtAggregate(spec), domain.DefaultAnalysisSettings())

	if !run.Timeline.HasAxis || !run.WallTime.HasSpan {
		t.Fatalf("has axis %v, has span %v, want both", run.Timeline.HasAxis, run.WallTime.HasSpan)
	}
	if !run.WallTime.Span.Start.Equal(run.Timeline.Axis.Start) || !run.WallTime.Span.End.Equal(run.Timeline.Axis.End) {
		t.Errorf("span %v..%v differs from axis %v..%v", run.WallTime.Span.Start, run.WallTime.Span.End,
			run.Timeline.Axis.Start, run.Timeline.Axis.End)
	}
	if !run.WallTime.Span.End.Equal(ctxAt(120)) {
		t.Errorf("span end = %v, want 120s: the latest logged timestamp, past the run end", run.WallTime.Span.End)
	}
}

func TestWallTime_WaitOutsideSubagents_TakesPrecedenceOverOrchestratorOnly(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 10, 20)}
	spec.agents = []wtAgent{{id: "a#1", start: 15, end: 50}}

	got := wtSplit(t, spec)

	// waiting 10..20; subagents 20..50 only (15..20 is already waiting).
	wtAssertSplit(t, got, 10, 30, 60, 0)
}

func TestWallTime_WaitInsideSubagent_CountsAsWaitingOnly(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 60, tools: []domain.ToolCall{wtTool("w1", ask, 20, 30)}}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 10, 40, 50, 0)
}

func TestWallTime_OverlappingSubagents_CountEachInstantOnce(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 50}, {id: "b#2", start: 30, end: 70}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 60, 40, 0)
}

func TestWallTime_OverlappingWaits_AreMergedAndCountedOnce(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 10, 30), wtTool("w2", ask, 20, 40)}
	spec.agents = []wtAgent{{id: "a#1", start: 50, end: 60, tools: []domain.ToolCall{wtTool("w3", ask, 70, 75)}}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 35, 10, 55, 0)
	wtAssertIntervals(t, "wait intervals", got.WaitIntervals, [][2]int{{10, 40}, {70, 75}})
}

func TestWallTime_OpenInvocation_RunsToTheEndOfTheSpan(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: 40, end: wtNone}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 60, 40, 0)
}

func TestWallTime_InvocationWithoutStart_AddsNoSubagentTime(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: wtNone, end: 60}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
}

func TestWallTime_GapBetweenSessions_IsBetweenSessionsTime(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s2", 50, 100), wtSession("s1", 0, 30)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 80, 20)
	wtAssertIntervals(t, "session gaps", got.SessionGaps, [][2]int{{30, 50}})
}

func TestWallTime_SessionsThatTouchOrOverlap_HaveNoGap(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 40), wtSession("s2", 40, 60), wtSession("s3", 55, 100)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
	if len(got.SessionGaps) != 0 {
		t.Errorf("session gaps = %v, want none", got.SessionGaps)
	}
}

func TestWallTime_SessionWithoutEnd_HasNoGapAfterIt(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s1", 0, wtNone), wtSession("s2", 50, 100)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 100, 0)
}

func TestWallTime_SubagentInsideSessionGap_WinsOverBetweenSessions(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 30), wtSession("s2", 50, 100)}
	spec.agents = []wtAgent{{id: "a#1", start: 35, end: 45}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 10, 80, 10)
}

func TestWallTime_WaitInsideSessionGap_WinsOverBetweenSessions(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 30), wtSession("s2", 50, 100)}
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 25, 40)}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 15, 0, 75, 10)
}

func TestWallTime_WaitStraddlingTheEndOfASessionGap_CountsEachInstantOnce(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 30), wtSession("s2", 50, 100)}
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 45, 60)}

	got := wtSplit(t, spec)

	// gap 30..50: 30..45 is between sessions, 45..50 is waiting, 50..60 is waiting.
	wtAssertSplit(t, got, 15, 0, 70, 15)
}

func TestWallTime_NotificationAfterTheRunEnd_ExtendsTheSpan(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.notifs = []domain.Notification{wtNotification(domain.Orchestrator(), 130, "idle_prompt", "")}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 130, 0)
}

func TestWallTime_CompactionAfterTheRunEnd_ExtendsTheSpan(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.compactions = []domain.Compaction{{Actor: domain.Orchestrator(), At: wtTime(140), Shape: domain.CompactionSingle}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 0, 140, 0)
}

func TestWallTime_NoTimestamps_HasNoSpanAndAllCategoriesAreZero(t *testing.T) {
	spec := newTestWallSpec(wtNone, wtNone)

	got := wtSplit(t, spec)

	if got.HasSpan {
		t.Errorf("has span with span %v..%v, want none: nothing was logged", got.Span.Start, got.Span.End)
	}
	if got.Waiting != 0 || got.Subagents != 0 || got.OrchestratorOnly != 0 || got.BetweenSessions != 0 {
		t.Errorf("categories = %v/%v/%v/%v, want all zero", got.Waiting, got.Subagents, got.OrchestratorOnly, got.BetweenSessions)
	}
}

func TestWallTime_RunWithoutEndEvent_UsesTheLatestLoggedTimestampAsSpanEnd(t *testing.T) {
	spec := newTestWallSpec(0, wtNone)
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 70}}

	got := wtSplit(t, spec)

	wtAssertSplit(t, got, 0, 60, 10, 0)
	if !got.Span.End.Equal(ctxAt(70)) {
		t.Errorf("span end = %v, want 70s", got.Span.End)
	}
}

func TestWallTime_ManyInvocations_StillSumToTheSpan(t *testing.T) {
	spec := newTestWallSpec(wtNone, wtNone)
	for i := 0; i < 150; i++ {
		spec.agents = append(spec.agents, wtAgent{id: domain.AgentInstanceID(fmt.Sprintf("a#%d", i)), start: i * 10, end: i*10 + 15})
	}

	got := wtSplit(t, spec)

	// Consecutive invocations overlap by 5s, so the union is the whole span 0..1505.
	wtAssertSplit(t, got, 0, 1505, 0, 0)
}
