package analysis_test

// Shared builders for the tool context growth tests. Contexts are given as one
// number and split over input, cache read and cache write, so the growth
// formula is exercised across all three categories.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const tgAgentType = "coder"

// tgOne is a call that requested one tool once.
func tgOne(tool string) domain.RequestedTools {
	return domain.NewRequestedTools(map[string]int{tool: 1})
}

// tgCall is a per-call record whose context (input + cache read + cache write)
// is ctx, with out output tokens, requesting rt.
func tgCall(id string, sec int, ctx, out int64, rt domain.RequestedTools) domain.InvocationUsage {
	u := ctxCall(id, sec, ctx-30, 20, 10, out)
	u.RequestedTools = rt
	return u
}

// tgKinded marks a call as the given kind.
func tgKinded(u domain.InvocationUsage, kind domain.CallKind) domain.InvocationUsage {
	u.Kind = kind
	return u
}

// tgUserTurn is a user turn of an actor.
func tgUserTurn(actor domain.ActorRef, sec int) domain.Turn {
	return domain.Turn{Actor: actor, Role: domain.TurnUser, At: ctxAt(sec)}
}

// tgAgentActor is the actor of the single test agent (turns carry no type).
func tgAgentActor() domain.ActorRef { return domain.AgentInstance(ctxAgentID) }

// tgLoggedCompaction is a logged compaction of an actor.
func tgLoggedCompaction(actor domain.ActorRef, sec int) domain.Compaction {
	return domain.Compaction{Actor: actor, At: ctxAt(sec), Shape: domain.CompactionSingle}
}

// tgRunSpec describes a one-run aggregate with extra run-level inputs.
type tgRunSpec struct {
	ctxRunSpec
	turns []domain.Turn
}

func tgPrice(t *testing.T, spec tgRunSpec) domain.RunReport {
	t.Helper()
	agg := ctxAggregate(spec.ctxRunSpec)
	agg.Runs[0].Turns = spec.turns
	return ctxPrice(t, agg, domain.DefaultAnalysisSettings())
}

// tgAgentRun prices a run with one agent holding the given calls.
func tgAgentRun(t *testing.T, usage ...domain.InvocationUsage) domain.RunReport {
	t.Helper()
	return tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: usage}},
	}})
}

// tgCallGrowth returns the growth of the call with the given record id.
func tgCallGrowth(t *testing.T, a domain.ActorContext, recordID string) domain.CallGrowth {
	t.Helper()
	for _, c := range a.Calls {
		if c.RecordID == recordID {
			return c.Growth
		}
	}
	t.Fatalf("no call %q among %d calls", recordID, len(a.Calls))
	return domain.CallGrowth{}
}

func tgAssertGrowth(t *testing.T, name string, got domain.CallGrowth, state domain.GrowthState, tool string, tokens int64) {
	t.Helper()
	want := domain.CallGrowth{State: state, Tool: tool, Tokens: tokens}
	if got != want {
		t.Errorf("%s growth = %+v, want %+v", name, got, want)
	}
}

func tgAssertNotSingular(t *testing.T, name string, got domain.CallGrowth) {
	t.Helper()
	if got.State != domain.GrowthNotSingular {
		t.Errorf("%s growth = %+v, want not singular", name, got)
	}
}
