package analysis_test

// Shared builders and lookups for the run-model tests. Events are built in
// memory with explicit timestamps; no filesystem, clock or randomness is used
// except where a test reads a fixture log explicitly.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

var rmBase = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

// rmAt returns rmBase plus the given number of seconds.
func rmAt(sec int) time.Time { return rmBase.Add(time.Duration(sec) * time.Second) }

func rmInvStart(id domain.AgentInstanceID, agentType string, at time.Time) domain.Event {
	return domain.Event{
		Type:      domain.EventInvocationStart,
		Timestamp: at,
		InvocationStart: &domain.InvocationStartFields{
			AgentInstanceID: id,
			AgentType:       agentType,
		},
	}
}

// rmInvEnd builds an invocation_end with no token_usage key.
func rmInvEnd(id domain.AgentInstanceID, status string, model domain.ModelID, at time.Time) domain.Event {
	return domain.Event{
		Type:      domain.EventInvocationEnd,
		Timestamp: at,
		InvocationEnd: &domain.InvocationEndFields{
			AgentInstanceID: id,
			StatusCode:      status,
			Model:           model,
		},
	}
}

func rmToolStart(callID, tool string, at time.Time) domain.Event {
	return domain.Event{
		Type:          domain.EventToolCallStart,
		Timestamp:     at,
		ToolCallStart: &domain.ToolCallStartFields{CallID: callID, ToolName: tool},
	}
}

func rmToolEnd(callID string, status domain.ToolCallStatus, at time.Time) domain.Event {
	return domain.Event{
		Type:        domain.EventToolCallEnd,
		Timestamp:   at,
		ToolCallEnd: &domain.ToolCallEndFields{CallID: callID, Status: status},
	}
}

func rmCompaction(trigger string, before, after domain.TokenCount, at time.Time) domain.Event {
	return domain.Event{
		Type:       domain.EventCompaction,
		Timestamp:  at,
		Compaction: &domain.CompactionFields{Trigger: trigger, TokensBefore: before, TokensAfter: after},
	}
}

func rmUsage(id domain.AgentInstanceID, recordID string, input int64) domain.Event {
	return usageRecordEvent(id, recordID, "model-a", domain.TokenUsage{Input: domain.Tokens(input)})
}

func rmOrchestratorInput(events ...domain.Event) analysis.Input {
	return analysis.Input{Streams: []analysis.Stream{{Ref: orchStream(testRunRef), Events: events}}}
}

// rmActivity aggregates the input and returns the single named run's activity.
func rmActivity(t *testing.T, in analysis.Input) domain.RunActivity {
	t.Helper()
	agg, _ := analysis.Aggregate(in)
	return requireOneRun(t, agg).Activity
}

// rmInvocation returns the invocation of the given instance, failing when absent.
func rmInvocation(t *testing.T, act domain.RunActivity, id domain.AgentInstanceID) domain.Invocation {
	t.Helper()
	for _, inv := range act.Invocations {
		if inv.Actor.Instance == id {
			return inv
		}
	}
	t.Fatalf("no invocation for %q among %d invocations", id, len(act.Invocations))
	return domain.Invocation{}
}

func rmCallIDs(calls []domain.ToolCall) []string {
	ids := make([]string, len(calls))
	for i, c := range calls {
		ids[i] = c.CallID
	}
	return ids
}

func rmRequireCall(t *testing.T, calls []domain.ToolCall, callID string) domain.ToolCall {
	t.Helper()
	for _, c := range calls {
		if c.CallID == callID {
			return c
		}
	}
	t.Fatalf("no tool call %q among %v", callID, rmCallIDs(calls))
	return domain.ToolCall{}
}

func rmEqualStrings(a, b []string) bool {
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
