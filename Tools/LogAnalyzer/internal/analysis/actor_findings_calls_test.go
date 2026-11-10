package analysis_test

// Tests for the costliest-call findings of an actor.
//
// Rules under test:
//   - Calls with a known cost above zero, highest first, ties to the earlier
//     call, at most three.
//   - Each finding names the call by its index among the actor's calls and
//     carries the call cost and its share of the actor total.
//   - A call without a known cost, or with a zero cost, is never listed.
//   - The orchestrator has the same findings as an invocation.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// afCostliestCalls are the findings of the costliest kind of an actor.
func afCostliestCalls(s domain.ActorSession) []domain.ActorFinding {
	return afKind(s, domain.ActorFindingCostliestCall)
}

func TestActorFindings_CostliestCalls_ListsTheThreeCostliestHighestFirstWithTiesToTheEarlierCall(t *testing.T) {
	ref := afRef(ctxAgentID, "coder")
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{afAgent(ctxAgentID, "coder", []domain.InvocationUsage{
		afInput("a1", 10, 2000), // 6,000,000
		afInput("a2", 20, 5000), // 15,000,000
		afInput("a3", 30, 5000), // 15,000,000 (tie with the earlier call)
		afInput("a4", 40, 4000), // 12,000,000
		afInput("a5", 50, 1000), // 3,000,000
	})}})

	got := afCostliestCalls(afSession(t, run, ref))

	if len(got) != 3 {
		t.Fatalf("costliest call findings = %d (%+v), want 3", len(got), got)
	}
	wantCalls := []int{1, 2, 3}
	wantNanos := []int64{5000 * afPerToken, 5000 * afPerToken, 4000 * afPerToken}
	for i, f := range got {
		if f.Call != wantCalls[i] {
			t.Errorf("finding %d is call %d, want call %d", i, f.Call, wantCalls[i])
		}
		afMoney(t, "call cost", f.Cost, wantNanos[i])
		afAssertShare(t, f.Share, wantNanos[i], 17000*afPerToken)
	}
}

func TestActorFindings_CostliestCalls_FewerThanThreePricedCallsGiveFewerFindings(t *testing.T) {
	ref := afRef(ctxAgentID, "coder")
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{afAgent(ctxAgentID, "coder", []domain.InvocationUsage{
		afInput("a1", 10, 2000),
		afInput("a2", 20, 3000),
	})}})

	got := afCostliestCalls(afSession(t, run, ref))

	if len(got) != 2 {
		t.Fatalf("costliest call findings = %d, want 2", len(got))
	}
	if got[0].Call != 1 || got[1].Call != 0 {
		t.Errorf("calls = %d, %d, want 1 then 0 (costliest first)", got[0].Call, got[1].Call)
	}
}

func TestActorFindings_CostliestCalls_ShareIsTheCallCostOverTheActorTotal(t *testing.T) {
	ref := afRef(ctxAgentID, "coder")
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{afAgent(ctxAgentID, "coder", []domain.InvocationUsage{
		afInput("a1", 10, 1000),
		afInput("a2", 20, 3000),
	})}})

	got := afCostliestCalls(afSession(t, run, ref))

	afAssertShare(t, got[0].Share, 3000*afPerToken, 4000*afPerToken) // 75 percent
	afAssertShare(t, got[1].Share, 1000*afPerToken, 4000*afPerToken) // 25 percent
}

func TestActorFindings_CostliestCalls_ZeroCostCallIsNotListed(t *testing.T) {
	ref := afRef(ctxAgentID, "coder")
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{afAgent(ctxAgentID, "coder", []domain.InvocationUsage{
		afInput("a1", 10, 3000),
		afInput("a2", 20, 0),
	})}})

	got := afCostliestCalls(afSession(t, run, ref))

	if len(got) != 1 || got[0].Call != 0 {
		t.Errorf("findings = %+v, want only call 0: a zero-cost call is not costly", got)
	}
}

func TestActorFindings_CostliestCalls_CallOfAnUnpricedModelIsNotListed(t *testing.T) {
	ref := afRef(ctxAgentID, "coder")
	unpriced := afInput("a2", 20, 90_000)
	unpriced.Model = "model-unpriced"
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{afAgent(ctxAgentID, "coder", []domain.InvocationUsage{
		afInput("a1", 10, 3000),
		unpriced,
	})}})

	got := afCostliestCalls(afSession(t, run, ref))

	if len(got) != 1 || got[0].Call != 0 {
		t.Errorf("findings = %+v, want only call 0: the unpriced call has no known cost", got)
	}
}

func TestActorFindings_CostliestCalls_ActorWithoutCallsHasNone(t *testing.T) {
	ref := afRef(ctxAgentID, "coder")
	run := afPrice(t, ctxRunSpec{agents: []ctxAgent{afAgent(ctxAgentID, "coder", nil)}})

	s := afSession(t, run, ref)

	if len(s.Findings) != 0 {
		t.Errorf("findings = %+v, want none for an actor without calls", s.Findings)
	}
}

func TestActorFindings_CostliestCalls_OrchestratorHasThemToo(t *testing.T) {
	run := afPrice(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{
		afInput("o1", 10, 1000),
		afInput("o2", 20, 4000),
		afInput("o3", 30, 2000),
	}})

	got := afCostliestCalls(afSession(t, run, domain.Orchestrator()))

	if len(got) != 3 || got[0].Call != 1 || got[1].Call != 2 || got[2].Call != 0 {
		t.Errorf("orchestrator findings = %+v, want calls 1, 2, 0", got)
	}
}
