package digest_test

// A run loaded from a digest carries the same context figures as the run read
// from logs: they are computed from the per-call records the digest restores.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func ctxRoundTripAggregate() domain.Aggregate {
	call := func(id string, sec int, in, cr, cw int64) domain.InvocationUsage {
		return domain.InvocationUsage{
			Model: "model-a", Harness: domain.HarnessClaudeCode, RecordID: id, At: at(sec),
			Usage: domain.TokenUsage{
				Input: domain.Tokens(in), CacheRead: domain.Tokens(cr), CacheCreation: domain.Tokens(cw),
				Output: domain.Tokens(10),
			},
		}
	}
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	usage := []domain.InvocationUsage{
		call("r2", 20, 150_000, 50_000, 60_000), // context 260000
		call("r1", 10, 50_000, 40_000, 10_000),  // context 100000
		{Model: "model-a", Harness: domain.HarnessClaudeCode, At: at(5), // legacy aggregate entry
			Usage: domain.TokenUsage{Input: domain.Tokens(900_000)}},
	}
	return domain.Aggregate{Runs: []domain.RunAggregate{{
		Run: domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: []domain.InvocationUsage{
			call("o1", 12, 1, 2, 3),
		}},
		Agents: []domain.ActorAggregate{{Actor: agent, Invocations: usage}},
		Activity: domain.RunActivity{Invocations: []domain.Invocation{{
			Actor: agent, State: domain.InvocationClosed, StartedAt: at(0), EndedAt: at(100),
			Usage: usage,
			ToolCalls: []domain.ToolCall{{
				CallID: "w1", ToolName: "Write", StartedAt: at(15), EndedAt: at(16), Pairing: domain.ToolCallPaired,
			}},
		}}},
	}}}
}

func TestRoundTrip_ContextFiguresOfADigestLoadedRun_EqualThoseOfTheOriginalRun(t *testing.T) {
	agg := ctxRoundTripAggregate()
	loaded, findings := roundTrip(t, domain.Digest{Aggregate: agg})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"}
	table := domain.NewPricingTable(nil)

	original, _ := analysis.Price(agg, table, domain.DefaultAnalysisSettings(), src)
	restored, _ := analysis.Price(loaded.Aggregate, table, domain.DefaultAnalysisSettings(), src)

	got := restored.Runs[0].Context
	want := original.Runs[0].Context
	if !reflect.DeepEqual(got, want) {
		t.Errorf("digest-loaded context differs from the original:\n got %+v\nwant %+v", got, want)
	}
	// The comparison above is vacuous unless the original run has figures.
	coder, ok := want.Find("coder#1")
	if !ok {
		t.Fatal("original run has no context figures for coder#1")
	}
	if v, _ := coder.Peak.Tokens.Value(); v != 260_000 {
		t.Errorf("original peak = %d, want 260000", v)
	}
	if v, _ := coder.FirstWrite.Tokens.Value(); v != 100_000 {
		t.Errorf("original first write = %d, want 100000", v)
	}
}
