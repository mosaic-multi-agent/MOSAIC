package analysis_test

// Builders for the cache hit figure tests. The figure is the capped cache reads
// over the previous call's context, so fixtures are written by context size and
// cache read: chCall("c2", 20, 150, 100) is a call at second 20 whose context is
// 150 tokens of which 100 were read from the cache.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const chAgentType = "coder"

// chCall is a main call with the given total context and cache read; the rest
// of the context is input. Cache write and output are logged as zero.
func chCall(id string, sec int, context, read int64) domain.InvocationUsage {
	return ctxCall(id, sec, context-read, read, 0, 0)
}

// chSession returns the call with its session id set.
func chSession(session string, u domain.InvocationUsage) domain.InvocationUsage {
	return sessCall(session, u)
}

// chUntimed returns the call without a timestamp.
func chUntimed(u domain.InvocationUsage) domain.InvocationUsage {
	u.At = time.Time{}
	return u
}

// chNoCacheWrite returns the call with its cache write figure absent (a partial
// context) while its cache read stays present.
func chNoCacheWrite(u domain.InvocationUsage) domain.InvocationUsage {
	u.Usage.CacheCreation = domain.AbsentTokens()
	return u
}

// chActor is the actor reference of a coder dispatch, for logged compactions.
func chActor(id domain.AgentInstanceID) domain.ActorRef {
	return domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: id, Type: chAgentType}
}

// chAgent is a coder dispatch holding the given calls.
func chAgent(id domain.AgentInstanceID, calls ...domain.InvocationUsage) ctxAgent {
	return ctxAgent{id: id, typ: chAgentType, usage: calls}
}

// chRun prices a run of the orchestrator calls and dispatches, with the given
// logged compaction marks.
func chRun(t *testing.T, orch []domain.InvocationUsage, marks []domain.Compaction, agents ...ctxAgent) domain.RunReport {
	t.Helper()
	return crPrice(t, ctxRunSpec{orchUsage: orch, agents: agents, compactions: marks})
}

// chDispatchHit prices one coder dispatch and returns its hit ratio.
func chDispatchHit(t *testing.T, calls ...domain.InvocationUsage) domain.Ratio {
	t.Helper()
	run := chRun(t, nil, nil, chAgent("coder#1", calls...))
	return rwDispatchOf(t, run, "coder#1").Cache.HitRatio
}

// chFullUsage is a usage with all four categories present (zeros included).
func chFullUsage(in, cacheRead, cacheWrite, out int64) domain.TokenUsage {
	return domain.TokenUsage{
		Input:         domain.Tokens(in),
		CacheRead:     domain.Tokens(cacheRead),
		CacheCreation: domain.Tokens(cacheWrite),
		Output:        domain.Tokens(out),
	}
}

// chTimedUsageEvent is a usage_record event stamped at the given second of the
// run-model clock, so the call it makes is timed.
func chTimedUsageEvent(id domain.AgentInstanceID, recordID string, sec int, usage domain.TokenUsage) domain.Event {
	ev := usageRecordEvent(id, recordID, viewsModel, usage)
	ev.Timestamp = rmAt(sec)
	return ev
}

// chAssertUndefined fails unless the ratio is the zero Ratio.
func chAssertUndefined(t *testing.T, name string, got domain.Ratio) {
	t.Helper()
	if got != (domain.Ratio{}) {
		t.Errorf("%s = %d/%d, want undefined (0/0)", name, got.Numerator, got.Denominator)
	}
}

// chAssertExact fails unless the ratio has exactly this numerator and
// denominator (not merely an equal value), so the cap and the pair sums show.
func chAssertExact(t *testing.T, name string, got domain.Ratio, num, den int64) {
	t.Helper()
	if got.Numerator != num || got.Denominator != den {
		t.Errorf("%s = %d/%d, want %d/%d", name, got.Numerator, got.Denominator, num, den)
	}
}
