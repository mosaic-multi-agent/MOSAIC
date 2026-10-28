package analysis_test

// Shared builders for the context-figure tests. Aggregates are built directly
// (not from events) so every call, tool call and compaction is explicit.
//
// Prices come from flatModelPricing: input 3000 nanodollars per token.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const ctxModel = domain.ModelID("model-a")

var ctxBase = time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)

func ctxAt(sec int) time.Time { return ctxBase.Add(time.Duration(sec) * time.Second) }

// ctxCall is a per-call usage record with all four categories present.
func ctxCall(recordID string, sec int, in, cacheRead, cacheWrite, out int64) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model:    ctxModel,
		Harness:  domain.HarnessClaudeCode,
		RecordID: recordID,
		At:       ctxAt(sec),
		Usage: domain.TokenUsage{
			Input:         domain.Tokens(in),
			CacheRead:     domain.Tokens(cacheRead),
			CacheCreation: domain.Tokens(cacheWrite),
			Output:        domain.Tokens(out),
		},
	}
}

// ctxInputOnlyCall is a per-call record that logged only input and output.
func ctxInputOnlyCall(recordID string, sec int, in, out int64) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model:    ctxModel,
		Harness:  domain.HarnessClaudeCode,
		RecordID: recordID,
		At:       ctxAt(sec),
		Usage:    domain.TokenUsage{Input: domain.Tokens(in), Output: domain.Tokens(out)},
	}
}

// ctxLegacyEntry is an aggregate entry of the legacy kind (no record id).
func ctxLegacyEntry(sec int, in, cacheRead, cacheWrite, out int64) domain.InvocationUsage {
	return ctxCall("", sec, in, cacheRead, cacheWrite, out)
}

// ctxTool is a paired tool call from startSec to endSec.
func ctxTool(callID, name string, startSec, endSec int) domain.ToolCall {
	return domain.ToolCall{
		CallID:    callID,
		ToolName:  name,
		StartedAt: ctxAt(startSec),
		EndedAt:   ctxAt(endSec),
		Pairing:   domain.ToolCallPaired,
	}
}

// ctxAgent describes one subagent invocation of a test run.
type ctxAgent struct {
	id    domain.AgentInstanceID
	typ   string
	usage []domain.InvocationUsage
	tools []domain.ToolCall
}

// ctxRunSpec describes a one-run aggregate.
type ctxRunSpec struct {
	orchUsage   []domain.InvocationUsage
	orchTools   []domain.ToolCall
	agents      []ctxAgent
	compactions []domain.Compaction
	harnesses   []domain.Harness // RunFacts.Harnesses
}

const ctxRunID = "20261028T100000Z-aaaa"

func ctxAggregate(spec ctxRunSpec) domain.Aggregate {
	run := domain.RunAggregate{
		Run:          domain.NamedRun(ctxRunID),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: spec.orchUsage},
		Activity: domain.RunActivity{
			OrchestratorToolCalls: spec.orchTools,
			Compactions:           spec.compactions,
		},
		Facts: domain.RunFacts{Harnesses: spec.harnesses},
	}
	for _, a := range spec.agents {
		actor := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: a.id, Type: a.typ}
		run.Agents = append(run.Agents, domain.ActorAggregate{Actor: actor, Invocations: a.usage})
		run.Activity.Invocations = append(run.Activity.Invocations, domain.Invocation{
			Actor:      actor,
			State:      domain.InvocationClosed,
			StatusCode: "SUCCESS",
			StartedAt:  ctxAt(0),
			EndedAt:    ctxAt(1000),
			ToolCalls:  a.tools,
			Usage:      a.usage,
		})
	}
	return domain.Aggregate{Runs: []domain.RunAggregate{run}}
}

// ctxPrice prices an aggregate with the given analysis settings and returns the
// only run's report.
//
// All context tests go through this one function, so the call that threads the
// settings into the pricing entry point lives here only.
func ctxPrice(t *testing.T, agg domain.Aggregate, settings domain.AnalysisSettings) domain.RunReport {
	t.Helper()
	table := domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, ctxModel)})
	report, _ := analysis.Price(agg, table, settings, testSource())
	if len(report.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(report.Runs))
	}
	return report.Runs[0]
}

func ctxPriceDefault(t *testing.T, spec ctxRunSpec) domain.RunReport {
	t.Helper()
	return ctxPrice(t, ctxAggregate(spec), domain.DefaultAnalysisSettings())
}

func ctxRequireAgent(t *testing.T, run domain.RunReport, id domain.AgentInstanceID) domain.ActorContext {
	t.Helper()
	got, ok := run.Context.Find(id)
	if !ok {
		t.Fatalf("no context figures for %s among %d invocations", id, len(run.Context.Invocations))
	}
	return got
}

// ctxRequireCall returns the i-th call row, failing the test when it is missing.
func ctxRequireCall(t *testing.T, a domain.ActorContext, i int) domain.ContextCall {
	t.Helper()
	if i >= len(a.Calls) {
		t.Fatalf("call rows = %d, want at least %d", len(a.Calls), i+1)
	}
	return a.Calls[i]
}

func ctxAssertTokens(t *testing.T, name string, got domain.TokenCount, want int64) {
	t.Helper()
	v, ok := got.Value()
	if !ok {
		t.Errorf("%s: absent, want %d", name, want)
		return
	}
	if v != want {
		t.Errorf("%s = %d, want %d", name, v, want)
	}
}

func ctxAssertAbsent(t *testing.T, name string, got domain.TokenCount) {
	t.Helper()
	if v, ok := got.Value(); ok {
		t.Errorf("%s = %d, want absent", name, v)
	}
}
