package analysis_test

// Tests that requested tools survive aggregation and reach the per-call
// context model.
//
// Rules under test:
//   - A usage_record unit carries the event's requested tools, orchestrator
//     and invocation streams alike; provided-empty and not-provided stay apart.
//   - The last observation of a record_id wins, the field included: a later
//     absent value replaces an earlier present one.
//   - Turn and invocation_end units are never provided.
//   - ContextCall exposes the value for orchestrator and invocation calls.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func toolsRecord(id domain.AgentInstanceID, recordID string, tools domain.RequestedTools) domain.Event {
	ev := usageRecordEvent(id, recordID, v130Model, domain.TokenUsage{Input: domain.Tokens(10)})
	ev.UsageRecord.RequestedTools = tools
	return ev
}

func TestAggregate_UsageRecord_CarriesRequestedToolsKeepingEmptyAndAbsentApart(t *testing.T) {
	present := domain.NewRequestedTools(map[string]int{"read_file": 2, "glob": 1})
	empty := domain.NewRequestedTools(nil)

	got := orchestratorUsages(t,
		toolsRecord("", "r1", present),
		toolsRecord("", "r2", empty),
		toolsRecord("", "r3", domain.RequestedTools{}),
	)

	if len(got) != 3 {
		t.Fatalf("got %d usage units, want 3", len(got))
	}
	if !got[0].RequestedTools.IsProvided() || got[0].RequestedTools.Count("read_file") != 2 || got[0].RequestedTools.Count("glob") != 1 {
		t.Errorf("r1 requested tools = %v, want read_file 2 and glob 1", got[0].RequestedTools.Counts())
	}
	if !got[1].RequestedTools.IsProvided() || got[1].RequestedTools.Total() != 0 {
		t.Errorf("r2 requested tools provided %v total %d, want provided and empty",
			got[1].RequestedTools.IsProvided(), got[1].RequestedTools.Total())
	}
	if got[2].RequestedTools.IsProvided() {
		t.Error("r3 requested tools provided, want not provided")
	}
}

func TestAggregate_Agent_UsageRecord_CarriesRequestedTools(t *testing.T) {
	got := agentUsages(t, toolsRecord("a#1", "r1", domain.NewRequestedTools(map[string]int{"edit": 1})))

	if len(got) != 1 {
		t.Fatalf("got %d usage units, want 1", len(got))
	}
	if got[0].RequestedTools.Count("edit") != 1 || !got[0].RequestedTools.IsProvided() {
		t.Errorf("requested tools = %v, want edit 1", got[0].RequestedTools.Counts())
	}
}

func TestAggregate_SupersededRecord_LastObservationWinsForRequestedTools(t *testing.T) {
	got := orchestratorUsages(t,
		toolsRecord("", "r1", domain.NewRequestedTools(map[string]int{"glob": 1})),
		toolsRecord("", "r1", domain.NewRequestedTools(map[string]int{"edit": 2, "glob": 1})),
	)

	if len(got) != 1 {
		t.Fatalf("got %d usage units, want 1", len(got))
	}
	if got[0].RequestedTools.Count("edit") != 2 || got[0].RequestedTools.Count("glob") != 1 {
		t.Errorf("requested tools = %v, want the later observation (edit 2, glob 1)", got[0].RequestedTools.Counts())
	}
}

func TestAggregate_SupersededRecord_LaterAbsentValueReplacesEarlierPresentOne(t *testing.T) {
	got := orchestratorUsages(t,
		toolsRecord("", "r1", domain.NewRequestedTools(map[string]int{"glob": 1})),
		toolsRecord("", "r1", domain.RequestedTools{}),
	)

	if got[0].RequestedTools.IsProvided() {
		t.Errorf("requested tools = %v, want not provided: the last observation wins", got[0].RequestedTools.Counts())
	}
}

func TestAggregate_SupersededRecord_LaterEmptyValueReplacesEarlierAbsentOne(t *testing.T) {
	got := orchestratorUsages(t,
		toolsRecord("", "r1", domain.RequestedTools{}),
		toolsRecord("", "r1", domain.NewRequestedTools(nil)),
	)

	if !got[0].RequestedTools.IsProvided() || got[0].RequestedTools.Total() != 0 {
		t.Errorf("requested tools provided %v, want provided and empty", got[0].RequestedTools.IsProvided())
	}
}

func TestAggregate_TurnAndInvocationEnd_AreNotProvided(t *testing.T) {
	turn := orchestratorUsages(t, turnEvent(v130Model, withReasoning(100, domain.AbsentTokens())))
	end := agentUsages(t, invEndEvent("a#1", v130Model, withReasoning(100, domain.AbsentTokens())))

	if turn[0].RequestedTools.IsProvided() || end[0].RequestedTools.IsProvided() {
		t.Error("a turn or invocation_end unit reads as provided, want not provided")
	}
}

func TestPrice_ContextCalls_ExposeRequestedToolsForOrchestratorAndInvocation(t *testing.T) {
	orch := ctxCall("o1", 10, 100, 0, 0, 5)
	orch.RequestedTools = domain.NewRequestedTools(map[string]int{"glob": 1})
	orchEmpty := ctxCall("o2", 20, 120, 0, 0, 5)
	orchEmpty.RequestedTools = domain.NewRequestedTools(nil)
	agentCall := ctxCall("a1", 15, 200, 0, 0, 5)
	agentCall.RequestedTools = domain.NewRequestedTools(map[string]int{"read_file": 2, "edit": 1})
	agentOld := ctxCall("a2", 25, 220, 0, 0, 5)

	run := ctxPriceDefault(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{orch, orchEmpty},
		agents: []ctxAgent{{id: "coder#1", typ: "coder",
			usage: []domain.InvocationUsage{agentCall, agentOld}}},
	})

	o1 := ctxRequireCall(t, run.Context.Orchestrator, 0)
	if o1.RequestedTools.Count("glob") != 1 || !o1.RequestedTools.IsProvided() {
		t.Errorf("orchestrator call 0 requested tools = %v, want glob 1", o1.RequestedTools.Counts())
	}
	o2 := ctxRequireCall(t, run.Context.Orchestrator, 1)
	if !o2.RequestedTools.IsProvided() || o2.RequestedTools.Total() != 0 {
		t.Errorf("orchestrator call 1 provided %v total %d, want provided and empty",
			o2.RequestedTools.IsProvided(), o2.RequestedTools.Total())
	}
	coder := ctxRequireAgent(t, run, "coder#1")
	a1 := ctxRequireCall(t, coder, 0)
	if a1.RequestedTools.Count("read_file") != 2 || a1.RequestedTools.Count("edit") != 1 {
		t.Errorf("invocation call 0 requested tools = %v, want read_file 2 and edit 1", a1.RequestedTools.Counts())
	}
	a2 := ctxRequireCall(t, coder, 1)
	if a2.RequestedTools.IsProvided() {
		t.Error("invocation call 1 reads as provided, want not provided")
	}
}
