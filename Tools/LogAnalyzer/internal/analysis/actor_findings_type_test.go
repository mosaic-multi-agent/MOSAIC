package analysis_test

// Tests for the findings that compare an invocation with the average of its
// agent type, and for the list of sessions itself.
//
// Rules under test:
//   - Peak context and cost are compared with the type average, with the number
//     of agents behind it, for invocations whose type average rests on at least
//     two agents and whose own figure is present.
//   - An invocation that is the only one of its type, or whose type average rests
//     on a single agent, has no such finding; so has the orchestrator.
//   - The sessions are the orchestrator first, then one per invocation sorted by
//     instance id.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// afOneCall is an agent with a single call of the given context size.
func afOneCall(id domain.AgentInstanceID, typ string, tokens int64) ctxAgent {
	return afAgent(id, typ, []domain.InvocationUsage{afInput(string(id)+"-1", 10, tokens)})
}

func afAssertVsType(t *testing.T, s domain.ActorSession, peak, avgPeak, cost, avgCost int64, members int) {
	t.Helper()
	p := afOne(t, s, domain.ActorFindingPeakContextVsType)
	ctxAssertTokens(t, "peak context", p.Tokens, peak)
	ctxAssertTokens(t, "type average peak context", p.TypeAverage, avgPeak)
	if p.TypeMembers != members {
		t.Errorf("peak context basis = %d agents, want %d", p.TypeMembers, members)
	}
	c := afOne(t, s, domain.ActorFindingCostVsType)
	afMoney(t, "cost", c.Cost, cost)
	afMoney(t, "type average cost", c.TypeCost, avgCost)
	if c.TypeMembers != members {
		t.Errorf("cost basis = %d agents, want %d", c.TypeMembers, members)
	}
}

func TestActorFindings_VsType_EveryMemberIsComparedWithTheTypeAverage(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{
		afOneCall("coder#1", "coder", 1000),
		afOneCall("coder#2", "coder", 2000),
		afOneCall("coder#3", "coder", 6000),
	}})

	// Peaks 1000, 2000, 6000 average 3000; costs 3M, 6M, 18M average 9M nanodollars.
	for _, m := range []struct {
		id   domain.AgentInstanceID
		peak int64
	}{{"coder#1", 1000}, {"coder#2", 2000}, {"coder#3", 6000}} {
		afAssertVsType(t, afSession(t, run, afRef(m.id, "coder")), m.peak, 3000, m.peak*afPerToken, 3000*afPerToken, 3)
	}
}

func TestActorFindings_VsType_AverageRoundsHalfUp(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{
		afOneCall("writer#1", "writer", 1000),
		afOneCall("writer#2", "writer", 2001),
	}})

	p := afOne(t, afSession(t, run, afRef("writer#1", "writer")), domain.ActorFindingPeakContextVsType)

	ctxAssertTokens(t, "type average peak context", p.TypeAverage, 1501) // 1500.5
}

func TestActorFindings_VsType_TypesAreKeptApart(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{
		afOneCall("coder#1", "coder", 1000),
		afOneCall("coder#2", "coder", 3000),
		afOneCall("tester#1", "tester", 50_000),
		afOneCall("tester#2", "tester", 70_000),
	}})

	afAssertVsType(t, afSession(t, run, afRef("coder#1", "coder")), 1000, 2000, 1000*afPerToken, 2000*afPerToken, 2)
	afAssertVsType(t, afSession(t, run, afRef("tester#1", "tester")), 50_000, 60_000, 50_000*afPerToken, 60_000*afPerToken, 2)
}

func TestActorFindings_VsType_OnlyAgentOfItsTypeHasNoComparison(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{
		afOneCall("coder#1", "coder", 1000),
		afOneCall("coder#2", "coder", 3000),
		afOneCall("tester#1", "tester", 50_000),
	}})

	s := afSession(t, run, afRef("tester#1", "tester"))

	afNone(t, s, domain.ActorFindingPeakContextVsType)
	afNone(t, s, domain.ActorFindingCostVsType)
}

func TestActorFindings_VsType_TypeAverageRestingOnOneAgentHasNoComparison(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{
		afOneCall("reviewer#1", "reviewer", 4000),
		afAgent("reviewer#2", "reviewer", nil),
	}})

	s := afSession(t, run, afRef("reviewer#1", "reviewer"))

	afNone(t, s, domain.ActorFindingPeakContextVsType)
	afNone(t, s, domain.ActorFindingCostVsType)
}

func TestActorFindings_VsType_MemberWithoutFiguresHasNoComparisonWhileTheOthersKeepTheirs(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{
		afOneCall("builder#1", "builder", 1000),
		afOneCall("builder#2", "builder", 3000),
		afAgent("builder#3", "builder", nil),
	}})

	var group domain.AgentTypeGroup
	for _, g := range run.ByAgentType {
		if g.AgentType == "builder" {
			group = g
		}
	}
	first := afSession(t, run, afRef("builder#1", "builder"))
	p := afOne(t, first, domain.ActorFindingPeakContextVsType)
	c := afOne(t, first, domain.ActorFindingCostVsType)
	if p.TypeAverage != group.PeakContext.Average || p.TypeMembers != group.PeakContext.Basis || p.TypeMembers != 2 {
		t.Errorf("peak comparison = %+v, want the type's peak average %+v over 2 agents", p, group.PeakContext)
	}
	if c.TypeCost != group.PerInvocation.Average || c.TypeMembers != group.PerInvocation.Basis {
		t.Errorf("cost comparison = %+v, want the type's cost average %+v", c, group.PerInvocation)
	}
	empty := afSession(t, run, afRef("builder#3", "builder"))
	afNone(t, empty, domain.ActorFindingPeakContextVsType)
	afNone(t, empty, domain.ActorFindingCostVsType)
}

func TestActorFindings_VsType_OrchestratorHasNoComparison(t *testing.T) {
	run := afPrice(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{afInput("o1", 10, 5000)},
		agents:    []ctxAgent{afOneCall("coder#1", "coder", 1000), afOneCall("coder#2", "coder", 3000)},
	})

	s := afSession(t, run, domain.Orchestrator())

	afNone(t, s, domain.ActorFindingPeakContextVsType)
	afNone(t, s, domain.ActorFindingCostVsType)
}

func TestActorSessions_OrchestratorFirstThenInvocationsSortedByInstanceId(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{
		afOneCall("zed#1", "zed", 1000),
		afOneCall("alpha#2", "alpha", 1000),
		afOneCall("alpha#1", "alpha", 1000),
	}})

	var got []string
	for _, s := range run.ActorSessions {
		got = append(got, s.Actor.Label())
	}

	if len(got) != 4 || run.ActorSessions[0].Actor.Kind != domain.ActorOrchestrator {
		t.Fatalf("sessions = %q, want the orchestrator and three invocations", got)
	}
	if want := []string{"alpha#1", "alpha#2", "zed#1"}; got[1] != want[0] || got[2] != want[1] || got[3] != want[2] {
		t.Errorf("invocation sessions = %q, want %q", got[1:], want)
	}
}

func TestActorSessions_ActorSessionFindsEachActorAndNoOtherOne(t *testing.T) {
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{afOneCall("coder#1", "coder", 1000)}})

	if _, ok := run.ActorSession(domain.Orchestrator()); !ok {
		t.Error("no session for the orchestrator")
	}
	if s, ok := run.ActorSession(afRef("coder#1", "coder")); !ok || s.Actor.Instance != "coder#1" {
		t.Errorf("session for coder#1 = %+v, %v", s.Actor, ok)
	}
	if _, ok := run.ActorSession(afRef("ghost#9", "coder")); ok {
		t.Error("a session was found for an invocation the run does not hold")
	}
}
