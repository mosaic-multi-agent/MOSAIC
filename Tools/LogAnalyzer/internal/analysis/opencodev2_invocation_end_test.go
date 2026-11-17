package analysis_test

// Tests for invocation_end events without token_usage.
//
// Rules under test:
//   - opencode-v2 reports usage through usage_record events and its
//     invocation_end never carries token_usage, so the missing token usage
//     finding is not raised for it.
//   - Other harnesses keep the finding.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func v2AgentWithoutEndUsage(h domain.Harness) analysis.Input {
	agentID := domain.AgentInstanceID("Agent#1")
	usage := usageRecordEvent(agentID, "r1", "claude-sonnet", domain.TokenUsage{Input: domain.Tokens(40)})
	usage.Harness = h
	end := invEndEventNoUsage(agentID, "claude-sonnet")
	end.Harness = h
	return oneAgentStream(testRunRef, agentID, usage, end)
}

func TestAggregate_OpenCodeV2InvocationEndWithoutTokenUsage_RaisesNoMissingTokenUsageFinding(t *testing.T) {
	agg, findings := analysis.Aggregate(v2AgentWithoutEndUsage(domain.HarnessOpenCodeV2))

	if hasFindingKind(findings, domain.FindingMissingTokenUsage) {
		t.Errorf("opencode-v2 usage comes from usage_record; got missing_token_usage: %v", findings)
	}
	if hasFindingKind(findings, domain.FindingUnrecognisedHarness) {
		t.Errorf("opencode-v2 must be recognised; got: %v", findings)
	}
	run := requireOneRun(t, agg)
	if len(run.Agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(run.Agents))
	}
	assertTokenPresent(t, "Input", run.Agents[0].Totals().Input, 40)
}

func TestAggregate_OtherHarnessInvocationEndWithoutTokenUsage_StillRaisesMissingTokenUsageFinding(t *testing.T) {
	_, findings := analysis.Aggregate(v2AgentWithoutEndUsage(domain.HarnessClaudeCode))

	if !hasFindingKind(findings, domain.FindingMissingTokenUsage) {
		t.Errorf("claude-code invocation_end without token_usage must still raise missing_token_usage; got: %v", findings)
	}
}
