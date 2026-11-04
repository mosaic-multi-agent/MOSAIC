package analysis_test

// Shared builders for the run-fact tests: lifecycle, notification and turn
// events with explicit envelope fields, and lookups on the aggregate.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const rfCC = domain.HarnessClaudeCode

// rfEnv sets the envelope fields on ev and returns it.
func rfEnv(ev domain.Event, at time.Time, harness domain.Harness, schema, session string) domain.Event {
	ev.Timestamp = at
	ev.Harness = harness
	ev.SchemaVersion = schema
	ev.SessionID = session
	return ev
}

func rfRunStart(at time.Time, f domain.RunStartFields) domain.Event {
	return rfEnv(domain.Event{Type: domain.EventRunStart, RunStart: &f}, at, rfCC, "1.3.0", "")
}

func rfRunEnd(at time.Time, outcome string) domain.Event {
	return rfEnv(domain.Event{Type: domain.EventRunEnd, RunEnd: &domain.RunEndFields{Outcome: outcome}}, at, rfCC, "1.3.0", "")
}

func rfSessionStart(session string, at time.Time, f domain.SessionStartFields) domain.Event {
	return rfEnv(domain.Event{Type: domain.EventSessionStart, SessionStart: &f}, at, rfCC, "1.3.0", session)
}

func rfSessionEnd(session string, at time.Time, reason string) domain.Event {
	return rfEnv(domain.Event{Type: domain.EventSessionEnd, SessionEnd: &domain.SessionEndFields{Reason: reason}}, at, rfCC, "1.3.0", session)
}

func rfTurn(role domain.TurnRole, at time.Time, usage domain.TokenUsage) domain.Event {
	ev := domain.Event{Type: domain.EventTurn, Turn: &domain.TurnFields{
		Role: role, Model: "model-a", Usage: usage, HasUsage: !usage.IsEmpty(),
	}}
	return rfEnv(ev, at, rfCC, "1.3.0", "")
}

func rfNotification(at time.Time, f domain.NotificationFields) domain.Event {
	return rfEnv(domain.Event{Type: domain.EventNotification, Notification: &f}, at, rfCC, "1.3.0", "")
}

// rfAggregate feeds each stream's events through an Aggregator, in order.
func rfAggregate(streams ...analysis.Stream) (domain.Aggregate, []domain.Finding) {
	return analysis.Aggregate(analysis.Input{Streams: streams})
}

func rfOrch(events ...domain.Event) analysis.Stream {
	return analysis.Stream{Ref: orchStream(testRunRef), Events: events}
}

func rfAgent(id domain.AgentInstanceID, events ...domain.Event) analysis.Stream {
	return analysis.Stream{Ref: agentStream(testRunRef, id), Events: events}
}

// rfAgentNoHint is an agent-instance stream whose folder gave no instance hint.
func rfAgentNoHint(events ...domain.Event) analysis.Stream {
	return analysis.Stream{Ref: agentStream(testRunRef, ""), Events: events}
}

func rfUnattributable(events ...domain.Event) analysis.Stream {
	return analysis.Stream{Ref: orchStream(domain.UnattributableRun()), Events: events}
}

// rfOtherRunOrch builds a one-run orchestrator stream for an arbitrary run.
func rfOtherRunOrch(run domain.RunRef, events ...domain.Event) analysis.Stream {
	return analysis.Stream{Ref: orchStream(run), Events: events}
}

// rfRun aggregates the streams and returns the single named run.
func rfRun(t *testing.T, streams ...analysis.Stream) domain.RunAggregate {
	t.Helper()
	agg, _ := rfAggregate(streams...)
	return requireOneRun(t, agg)
}

func rfStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
