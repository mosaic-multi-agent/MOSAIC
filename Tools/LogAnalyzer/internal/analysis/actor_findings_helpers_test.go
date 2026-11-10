package analysis_test

// Shared builders for the actor findings and actor tool tests. Prices come from
// the flat table: 3000 nanodollars per input token, so a call that holds only
// input tokens costs 3000 nanodollars per token and no other category adds to it.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const afPerToken = 3000

// afRef is the actor reference of an agent as the aggregate builder makes it.
func afRef(id domain.AgentInstanceID, typ string) domain.ActorRef {
	return domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: id, Type: typ}
}

// afInput is a main call that holds only input tokens.
func afInput(id string, sec int, tokens int64) domain.InvocationUsage {
	return ctxCall(id, sec, tokens, 0, 0, 0)
}

// afAgent is an agent with the calls and tool calls given.
func afAgent(id domain.AgentInstanceID, typ string, usage []domain.InvocationUsage, tools ...domain.ToolCall) ctxAgent {
	return ctxAgent{id: id, typ: typ, usage: usage, tools: tools}
}

// afSession returns the session figures of an actor, failing the test when the run has none.
func afSession(t *testing.T, run domain.RunReport, actor domain.ActorRef) domain.ActorSession {
	t.Helper()
	s, ok := run.ActorSession(actor)
	if !ok {
		t.Fatalf("no actor session for %s among %d sessions", actor.Label(), len(run.ActorSessions))
	}
	return s
}

// afKind returns the findings of one kind in their order.
func afKind(s domain.ActorSession, kind domain.ActorFindingKind) []domain.ActorFinding {
	var out []domain.ActorFinding
	for _, f := range s.Findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

// afOne returns the only finding of the kind, failing the test otherwise.
func afOne(t *testing.T, s domain.ActorSession, kind domain.ActorFindingKind) domain.ActorFinding {
	t.Helper()
	got := afKind(s, kind)
	if len(got) != 1 {
		t.Fatalf("findings of kind %d = %d (%+v), want exactly 1", kind, len(got), s.Findings)
	}
	return got[0]
}

// afNone fails the test when the session holds a finding of the kind.
func afNone(t *testing.T, s domain.ActorSession, kind domain.ActorFindingKind) {
	t.Helper()
	if got := afKind(s, kind); len(got) != 0 {
		t.Errorf("findings of kind %d = %+v, want none", kind, got)
	}
}

// afMoney asserts a known amount of nanodollars.
func afMoney(t *testing.T, name string, got domain.MoneyValue, nanos int64) {
	t.Helper()
	assertMoneyKnown(t, name, got, nanos)
}

// afAssertShare asserts that the share is part/whole without depending on how the ratio is reduced.
func afAssertShare(t *testing.T, got domain.Ratio, part, whole int64) {
	t.Helper()
	if got.Denominator <= 0 || got.Numerator*whole != part*got.Denominator {
		t.Errorf("share = %d/%d, want %d/%d", got.Numerator, got.Denominator, part, whole)
	}
}

// afPrice prices a one-run spec.
func afPrice(t *testing.T, spec ctxRunSpec) domain.RunReport {
	t.Helper()
	return ctxPriceDefault(t, spec)
}
