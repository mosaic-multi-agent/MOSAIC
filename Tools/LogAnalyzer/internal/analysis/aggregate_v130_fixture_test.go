package analysis_test

// Fixture-driven tests: 1.3.0 logs read through the real reader and aggregated.
// The Claude Code-style fixtures mix a Runner-written 1.1.0 run_start/run_end
// with 1.3.0 adapter events and contain superseded observations.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	v130OpenCodeRun = "20261022T100000Z-v130"
	v130ClaudeRun   = "20261022T110000Z-v131"
)

func TestAggregate_OpenCodeFixture_ProviderAndReasoningPerRequest(t *testing.T) {
	run := domain.NamedRun(v130OpenCodeRun)
	agg, findings := analysis.Aggregate(analysis.Input{Streams: []analysis.Stream{
		fixtureStream(t, "v130_opencode.jsonl", orchStream(run)),
	}})

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	units := requireOneRun(t, agg).Orchestrator.Invocations
	if len(units) != 3 {
		t.Fatalf("got %d usage units, want 3", len(units))
	}
	requireUsageUnit(t, units[0], "openai", "")
	assertTokenPresent(t, "oc1 Reasoning", units[0].Usage.Reasoning, 40)
	requireUsageUnit(t, units[1], "openai", "")
	assertTokenPresent(t, "oc2 Reasoning (present zero)", units[1].Usage.Reasoning, 0)
	requireUsageUnit(t, units[2], "", "")
	assertTokenAbsent(t, "oc3 Reasoning", units[2].Usage.Reasoning)
}

func claudeFixtureRun(t *testing.T) (domain.RunAggregate, []domain.Finding) {
	t.Helper()
	run := domain.NamedRun(v130ClaudeRun)
	agg, findings := analysis.Aggregate(analysis.Input{Streams: []analysis.Stream{
		fixtureStream(t, "v130_cc_orchestrator.jsonl", orchStream(run)),
		fixtureStream(t, "v130_cc_agent.jsonl", agentStream(run, "worker#1")),
	}})
	return requireOneRun(t, agg), findings
}

func TestAggregate_ClaudeFixture_OrchestratorLastObservationWins(t *testing.T) {
	run, findings := claudeFixtureRun(t)

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	units := run.Orchestrator.Invocations
	if len(units) != 2 {
		t.Fatalf("got %d usage units, want 2 (o1 superseded in place)", len(units))
	}
	requireUsageUnit(t, units[0], "amazon-bedrock", "high")
	assertTokenPresent(t, "o1 Reasoning", units[0].Usage.Reasoning, 25)
	requireUsageUnit(t, units[1], "anthropic", "high")
	assertTokenAbsent(t, "o2 Reasoning", units[1].Usage.Reasoning)
	assertTokenPresent(t, "orchestrator Reasoning total", run.Orchestrator.Totals().Reasoning, 25)
}

func TestAggregate_ClaudeFixture_AgentLastObservationWins(t *testing.T) {
	run, _ := claudeFixtureRun(t)

	agent := requireOneAgent(t, run)
	if len(agent.Invocations) != 2 {
		t.Fatalf("got %d usage units, want 2 (a1 superseded in place)", len(agent.Invocations))
	}
	requireUsageUnit(t, agent.Invocations[0], "anthropic", "medium")
	assertTokenPresent(t, "a1 Reasoning", agent.Invocations[0].Usage.Reasoning, 7)
	requireUsageUnit(t, agent.Invocations[1], "anthropic", "low")
	assertTokenAbsent(t, "a2 Reasoning", agent.Invocations[1].Usage.Reasoning)
	assertTokenPresent(t, "agent Reasoning total", agent.Totals().Reasoning, 7)
}

func TestAggregate_ClaudeFixture_RunModelInvocationUsageMatchesAgent(t *testing.T) {
	run, _ := claudeFixtureRun(t)

	inv := rmInvocation(t, run.Activity, "worker#1")
	if len(inv.Usage) != 2 {
		t.Fatalf("Invocation.Usage has %d units, want 2", len(inv.Usage))
	}
	requireUsageUnit(t, inv.Usage[0], "anthropic", "medium")
	assertTokenPresent(t, "a1 Reasoning", inv.Usage[0].Usage.Reasoning, 7)
	requireUsageUnit(t, inv.Usage[1], "anthropic", "low")
}
