package analysis_test

// Shared builders for the wall-time split tests. The aggregate is built
// directly so every timestamp, tool call, session and notification is explicit.
// Seconds are offsets on the context-test clock (ctxAt).

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// wtNone marks "no timestamp" for a second offset.
const wtNone = -1

type wtAgent struct {
	id      domain.AgentInstanceID
	start   int // wtNone = no usable start
	end     int // wtNone = open invocation
	harness domain.Harness
	tools   []domain.ToolCall
}

type wtSpec struct {
	harnesses   []domain.Harness // RunFacts.Harnesses
	start, end  int              // RunFacts.StartedAt / EndedAt; wtNone = absent
	sessions    []domain.RunSession
	notifs      []domain.Notification
	orchTools   []domain.ToolCall
	compactions []domain.Compaction
	orchHarness domain.Harness // harness of the orchestrator's usage record; "" = none
	agents      []wtAgent
}

// newTestWallSpec returns a spec of a claude-code run over [start, end] seconds.
func newTestWallSpec(start, end int) wtSpec {
	return wtSpec{harnesses: []domain.Harness{domain.HarnessClaudeCode}, start: start, end: end}
}

func wtTime(sec int) time.Time {
	if sec == wtNone {
		return time.Time{}
	}
	return ctxAt(sec)
}

func wtTool(callID, name string, startSec, endSec int) domain.ToolCall {
	return ctxTool(callID, name, startSec, endSec)
}

// wtOpenTool is a tool call that started and never ended.
func wtOpenTool(callID, name string, startSec int) domain.ToolCall {
	return domain.ToolCall{CallID: callID, ToolName: name, StartedAt: ctxAt(startSec), Pairing: domain.ToolCallStartOnly}
}

func wtUsage(h domain.Harness) []domain.InvocationUsage {
	if h == "" {
		return nil
	}
	return []domain.InvocationUsage{{
		Model: ctxModel, Harness: h, RecordID: "r1",
		Usage: domain.TokenUsage{Input: domain.Tokens(1)},
	}}
}

func wtAggregate(spec wtSpec) domain.Aggregate {
	run := domain.RunAggregate{
		Run:           domain.NamedRun(ctxRunID),
		Orchestrator:  domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: wtUsage(spec.orchHarness)},
		Activity:      domain.RunActivity{OrchestratorToolCalls: spec.orchTools, Compactions: spec.compactions},
		Facts:         domain.RunFacts{Harnesses: spec.harnesses, StartedAt: wtTime(spec.start), EndedAt: wtTime(spec.end)},
		Sessions:      spec.sessions,
		Notifications: spec.notifs,
	}
	for _, a := range spec.agents {
		actor := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: a.id, Type: "coder"}
		usage := wtUsage(a.harness)
		run.Agents = append(run.Agents, domain.ActorAggregate{Actor: actor, Invocations: usage})
		inv := domain.Invocation{
			Actor: actor, State: domain.InvocationOpen,
			StartedAt: wtTime(a.start), ToolCalls: a.tools, Usage: usage,
		}
		if a.end != wtNone {
			inv.State, inv.StatusCode, inv.EndedAt, inv.EndEvents = domain.InvocationClosed, "SUCCESS", ctxAt(a.end), 1
		}
		run.Activity.Invocations = append(run.Activity.Invocations, inv)
	}
	return domain.Aggregate{Runs: []domain.RunAggregate{run}}
}

func wtSplitWith(t *testing.T, spec wtSpec, settings domain.AnalysisSettings) domain.WallTimeSplit {
	t.Helper()
	return ctxPrice(t, wtAggregate(spec), settings).WallTime
}

func wtSplit(t *testing.T, spec wtSpec) domain.WallTimeSplit {
	t.Helper()
	return wtSplitWith(t, spec, domain.DefaultAnalysisSettings())
}

func wtSession(id string, startSec, endSec int) domain.RunSession {
	return domain.RunSession{ID: id, StartedAt: wtTime(startSec), EndedAt: wtTime(endSec)}
}

func wtNotification(actor domain.ActorRef, sec int, typ, id string) domain.Notification {
	return domain.Notification{Actor: actor, At: wtTime(sec), Type: typ, ID: id}
}

// wtAssertSplit checks the four category durations in seconds and that they sum
// to the measured span.
func wtAssertSplit(t *testing.T, got domain.WallTimeSplit, waiting, subagents, orchOnly, between int) {
	t.Helper()
	if !got.HasSpan {
		t.Fatal("split has no span, want one")
	}
	sec := func(n int) time.Duration { return time.Duration(n) * time.Second }
	if got.Waiting != sec(waiting) || got.Subagents != sec(subagents) ||
		got.OrchestratorOnly != sec(orchOnly) || got.BetweenSessions != sec(between) {
		t.Errorf("waiting/subagents/orchestrator-only/between = %v/%v/%v/%v, want %ds/%ds/%ds/%ds",
			got.Waiting, got.Subagents, got.OrchestratorOnly, got.BetweenSessions, waiting, subagents, orchOnly, between)
	}
	wtAssertSumIsSpan(t, got)
}

func wtAssertSumIsSpan(t *testing.T, got domain.WallTimeSplit) {
	t.Helper()
	sum := got.Waiting + got.Subagents + got.OrchestratorOnly + got.BetweenSessions
	if span := got.Span.End.Sub(got.Span.Start); sum != span {
		t.Errorf("categories sum to %v, want the span %v", sum, span)
	}
}

// wtAssertIntervals compares intervals with [start, end] second pairs.
func wtAssertIntervals(t *testing.T, name string, got []domain.TimeInterval, want [][2]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
	for i, w := range want {
		if !got[i].Start.Equal(ctxAt(w[0])) || !got[i].End.Equal(ctxAt(w[1])) {
			t.Errorf("%s[%d] = %v..%v, want %ds..%ds", name, i, got[i].Start, got[i].End, w[0], w[1])
		}
	}
}
