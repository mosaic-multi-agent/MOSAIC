package analysis_test

// Tests that provider, effort and reasoning survive aggregation.
//
// Rules under test:
//   - Every per-request usage unit carries the Provider of its source event;
//     Effort is carried from usage_record only, legacy turn / invocation_end
//     samples carry "".
//   - Orchestrator and agent actors behave alike, and the run model's
//     Invocation.Usage carries the same values as the actor aggregate.
//   - The last observation of a record_id replaces the whole request:
//     provider, effort and reasoning all come from that observation.
//   - Reasoning sums keep absent-versus-zero semantics.

import (
	"fmt"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const v130Model = domain.ModelID("model-x")

func withReasoning(output int64, reasoning domain.TokenCount) domain.TokenUsage {
	return domain.TokenUsage{Output: domain.Tokens(output), Reasoning: reasoning}
}

// v130Record builds a usage_record carrying provider and effort.
func v130Record(id domain.AgentInstanceID, recordID string, provider domain.ProviderID, effort string, usage domain.TokenUsage) domain.Event {
	ev := usageRecordEvent(id, recordID, v130Model, usage)
	ev.UsageRecord.Provider = provider
	ev.UsageRecord.Effort = effort
	return ev
}

func orchestratorUsages(t *testing.T, events ...domain.Event) []domain.InvocationUsage {
	t.Helper()
	agg, _ := analysis.Aggregate(orchestratorOnlyInput(events...))
	return requireOneRun(t, agg).Orchestrator.Invocations
}

func agentUsages(t *testing.T, events ...domain.Event) []domain.InvocationUsage {
	t.Helper()
	agg, _ := analysis.Aggregate(oneAgentStream(testRunRef, "a#1", events...))
	return requireOneAgent(t, requireOneRun(t, agg)).Invocations
}

func requireUsageUnit(t *testing.T, u domain.InvocationUsage, provider domain.ProviderID, effort string) {
	t.Helper()
	if u.Provider != provider || u.Effort != effort {
		t.Errorf("unit = {Provider:%q Effort:%q}, want {Provider:%q Effort:%q}", u.Provider, u.Effort, provider, effort)
	}
}

func TestAggregate_Orchestrator_UsageRecord_CarriesProviderEffortAndReasoning(t *testing.T) {
	got := orchestratorUsages(t, v130Record("", "r1", "anthropic", "high", withReasoning(100, domain.Tokens(30))))

	if len(got) != 1 {
		t.Fatalf("got %d usage units, want 1", len(got))
	}
	requireUsageUnit(t, got[0], "anthropic", "high")
	assertTokenPresent(t, "Reasoning", got[0].Usage.Reasoning, 30)
}

func TestAggregate_Agent_UsageRecord_CarriesProviderEffortAndReasoning(t *testing.T) {
	got := agentUsages(t, v130Record("a#1", "r1", "openai", "low", withReasoning(100, domain.Tokens(0))))

	if len(got) != 1 {
		t.Fatalf("got %d usage units, want 1", len(got))
	}
	requireUsageUnit(t, got[0], "openai", "low")
	assertTokenPresent(t, "Reasoning", got[0].Usage.Reasoning, 0)
}

func TestAggregate_UsageRecord_WithoutProviderOrEffort_CarriesEmptyStrings(t *testing.T) {
	got := orchestratorUsages(t, v130Record("", "r1", "", "", withReasoning(100, domain.AbsentTokens())))

	requireUsageUnit(t, got[0], "", "")
	assertTokenAbsent(t, "Reasoning", got[0].Usage.Reasoning)
}

func TestAggregate_OrchestratorTurn_CarriesProviderButNoEffort(t *testing.T) {
	ev := turnEvent(v130Model, withReasoning(100, domain.Tokens(20)))
	ev.Turn.Provider = "anthropic"

	got := orchestratorUsages(t, ev)

	requireUsageUnit(t, got[0], "anthropic", "")
	assertTokenPresent(t, "Reasoning", got[0].Usage.Reasoning, 20)
}

func TestAggregate_AgentInvocationEnd_CarriesProviderButNoEffort(t *testing.T) {
	ev := invEndEvent("a#1", v130Model, withReasoning(100, domain.Tokens(20)))
	ev.InvocationEnd.Provider = "github-copilot"

	got := agentUsages(t, ev)

	requireUsageUnit(t, got[0], "github-copilot", "")
	assertTokenPresent(t, "Reasoning", got[0].Usage.Reasoning, 20)
}

func TestAggregate_SupersededRecord_LastObservationWinsForProviderEffortAndReasoning(t *testing.T) {
	got := orchestratorUsages(t,
		v130Record("", "r1", "anthropic", "medium", withReasoning(100, domain.Tokens(10))),
		v130Record("", "r2", "anthropic", "low", withReasoning(50, domain.AbsentTokens())),
		v130Record("", "r1", "amazon-bedrock", "high", withReasoning(100, domain.Tokens(25))),
	)

	if len(got) != 2 {
		t.Fatalf("got %d usage units, want 2 (superseded record counted once)", len(got))
	}
	requireUsageUnit(t, got[0], "amazon-bedrock", "high")
	assertTokenPresent(t, "r1 Reasoning", got[0].Usage.Reasoning, 25)
	requireUsageUnit(t, got[1], "anthropic", "low")
}

// Regression guard (passes before carry exists): protects against a later
// "merge instead of replace" bug. Last-wins evidence is the test above.
func TestAggregate_SupersededRecord_LastObservationWithoutNewFields_ClearsThem(t *testing.T) {
	got := agentUsages(t,
		v130Record("a#1", "r1", "anthropic", "high", withReasoning(100, domain.Tokens(10))),
		v130Record("a#1", "r1", "", "", withReasoning(100, domain.AbsentTokens())),
	)

	if len(got) != 1 {
		t.Fatalf("got %d usage units, want 1", len(got))
	}
	requireUsageUnit(t, got[0], "", "")
	assertTokenAbsent(t, "Reasoning", got[0].Usage.Reasoning)
}

func TestAggregate_Reasoning_SumKeepsAbsentVersusZero(t *testing.T) {
	cases := []struct {
		name      string
		counts    []domain.TokenCount
		present   bool
		wantTotal int64
	}{
		{"present and absent sum to the present part", []domain.TokenCount{domain.Tokens(5), domain.AbsentTokens(), domain.Tokens(7)}, true, 12},
		{"all absent stays absent", []domain.TokenCount{domain.AbsentTokens(), domain.AbsentTokens()}, false, 0},
		{"present zero stays present", []domain.TokenCount{domain.Tokens(0), domain.AbsentTokens()}, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var events []domain.Event
			for i, rc := range c.counts {
				events = append(events, v130Record("", fmt.Sprintf("r%d", i), "p", "", withReasoning(10, rc)))
			}

			agg, _ := analysis.Aggregate(orchestratorOnlyInput(events...))

			got := requireOneRun(t, agg).Orchestrator.Totals().Reasoning
			if !c.present {
				assertTokenAbsent(t, "Reasoning total", got)
				return
			}
			assertTokenPresent(t, "Reasoning total", got, c.wantTotal)
		})
	}
}

func TestAggregate_Reasoning_RunTotalsSumAcrossActors(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{v130Record("", "o1", "p", "", withReasoning(10, domain.Tokens(4)))}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{v130Record("a#1", "a1", "p", "", withReasoning(10, domain.Tokens(6)))}},
	}}

	agg, _ := analysis.Aggregate(in)

	assertTokenPresent(t, "run Reasoning", requireOneRun(t, agg).Totals().Reasoning, 10)
}

func TestAggregate_RunModelInvocationUsage_CarriesProviderEffortAndReasoning(t *testing.T) {
	agg, _ := analysis.Aggregate(oneAgentStream(testRunRef, "a#1",
		v130Record("a#1", "r1", "anthropic", "medium", withReasoning(100, domain.Tokens(8))),
	))

	run := requireOneRun(t, agg)
	inv := rmInvocation(t, run.Activity, "a#1")
	if len(inv.Usage) != 1 {
		t.Fatalf("Invocation.Usage has %d units, want 1", len(inv.Usage))
	}
	requireUsageUnit(t, inv.Usage[0], "anthropic", "medium")
	assertTokenPresent(t, "Reasoning", inv.Usage[0].Usage.Reasoning, 8)
}
