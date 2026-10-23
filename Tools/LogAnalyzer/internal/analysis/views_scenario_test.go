package analysis_test

// Shared scenario for the run-view tests (overview, agent types, status codes,
// tools). One named run with an orchestrator and six subagent instances, built
// in memory with explicit timestamps so every figure can be computed by hand.
//
// Rates per 1M tokens for model "m-a": input 1.00, cache read 0.10, cache write
// 5m 1.25, cache write 1h 2.00, output 5.00. Nanodollars per token are therefore
// input 1000, cache read 100, 5m 1250, 1h 2000, output 5000. Model "m-unpriced"
// has no pricing entry.
//
//	actor          type      usage                                          end
//	orchestrator   -         m-a: in 1000 cr 3000 cc 500 (no split) out 200  -
//	coder#1        coder     m-a: in 2000 cr 1000 cc 400 (300/100) out 100;  SUCCESS
//	                         m-a: in 1000 out 50
//	coder#2        coder     m-a: in 500 out 100                              SUCCESS
//	coder#3        coder     m-unpriced: in 700 out 70                        BLOCKED
//	tester#4       tester    m-a: in 100 cc 200 (no split)                    none (open)
//	reviewer#5     reviewer  no token usage                                   ended, no status_code
//	x#6            (none)    m-a: in 10                                       WEIRD (not in the enum)
//
// Per-actor money in nanodollars: orchestrator 2,925,000; coder#1 4,425,000;
// coder#2 1,000,000; coder#3 unpriced; tester#4 350,000; reviewer#5 no data;
// x#6 10,000. Run total 8,710,000 (partial: coder#3 is unpriced).
//
// Tool calls (seconds are offsets on the scenario clock):
//
//	orchestrator  Read o1 4s ok; Grep o2 start only; o3 end only (no name)
//	coder#1       Read p1 1s error; Read p2 2s ok (overlapping p1); p-orphan end only
//	coder#2       Bash b1 3s ok
//	tester#4      Read t1 start only

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	viewsModel    = domain.ModelID("m-a")
	viewsUnpriced = domain.ModelID("m-unpriced")
)

func viewsPricing(t *testing.T) domain.PricingTable {
	t.Helper()
	return domain.NewPricingTable([]domain.ModelPricing{{
		Model: viewsModel,
		Standard: domain.RateSet{
			Input:        mustParseRateAnalyze(t, "1.00"),
			CacheRead:    mustParseRateAnalyze(t, "0.10"),
			CacheWrite5m: mustParseRateAnalyze(t, "1.25"),
			CacheWrite1h: mustParseRateAnalyze(t, "2.00"),
			Output:       mustParseRateAnalyze(t, "5.00"),
		},
	}})
}

func viewsUsage(in, cr, cc, out int64) domain.TokenUsage {
	u := domain.TokenUsage{Input: domain.Tokens(in), Output: domain.Tokens(out)}
	if cr > 0 {
		u.CacheRead = domain.Tokens(cr)
	}
	if cc > 0 {
		u.CacheCreation = domain.Tokens(cc)
	}
	return u
}

func viewsScenarioInput() analysis.Input {
	run := testRunRef
	split := viewsUsage(2000, 1000, 400, 100)
	split.CacheCreation5m, split.CacheCreation1h = domain.Tokens(300), domain.Tokens(100)
	agent := func(id domain.AgentInstanceID, events ...domain.Event) analysis.Stream {
		return analysis.Stream{Ref: agentStream(run, id), Events: events}
	}
	return analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(run), Events: []domain.Event{
			usageRecordEvent("", "o-r1", viewsModel, viewsUsage(1000, 3000, 500, 200)),
			rmToolStart("o1", "Read", rmAt(1)), rmToolEnd("o1", domain.ToolCallSuccess, rmAt(5)),
			rmToolStart("o2", "Grep", rmAt(6)),
			rmToolEnd("o3", domain.ToolCallSuccess, rmAt(7)),
		}},
		agent("coder#1",
			rmInvStart("coder#1", "coder", rmAt(10)),
			rmToolStart("p1", "Read", rmAt(11)), rmToolStart("p2", "Read", rmAt(11)),
			rmToolEnd("p1", domain.ToolCallError, rmAt(12)), rmToolEnd("p2", domain.ToolCallSuccess, rmAt(13)),
			rmToolEnd("p-orphan", domain.ToolCallSuccess, rmAt(14)),
			usageRecordEvent("coder#1", "r1a", viewsModel, split),
			usageRecordEvent("coder#1", "r2", viewsModel, viewsUsage(1000, 0, 0, 50)),
			rmInvEnd("coder#1", "SUCCESS", viewsModel, rmAt(70))),
		agent("coder#2",
			rmInvStart("coder#2", "coder", rmAt(20)),
			rmToolStart("b1", "Bash", rmAt(21)), rmToolEnd("b1", domain.ToolCallSuccess, rmAt(24)),
			usageRecordEvent("coder#2", "r1b", viewsModel, viewsUsage(500, 0, 0, 100)),
			rmInvEnd("coder#2", "SUCCESS", viewsModel, rmAt(40))),
		agent("coder#3",
			rmInvStart("coder#3", "coder", rmAt(30)),
			usageRecordEvent("coder#3", "r1c", viewsUnpriced, viewsUsage(700, 0, 0, 70)),
			rmInvEnd("coder#3", "BLOCKED", viewsUnpriced, rmAt(50))),
		agent("tester#4",
			rmInvStart("tester#4", "tester", rmAt(35)),
			rmToolStart("t1", "Read", rmAt(36)),
			usageRecordEvent("tester#4", "r1d", viewsModel, viewsUsage(100, 0, 200, 0))),
		agent("reviewer#5",
			rmInvStart("reviewer#5", "reviewer", rmAt(45)),
			rmInvEnd("reviewer#5", "", "", rmAt(55))),
		agent("x#6",
			usageRecordEvent("x#6", "r1e", viewsModel, viewsUsage(10, 0, 0, 0)),
			rmInvEnd("x#6", "WEIRD", viewsModel, rmAt(60))),
	}}
}

// viewsScenarioRun analyses the scenario and returns its single named run.
func viewsScenarioRun(t *testing.T, table domain.PricingTable) domain.RunReport {
	t.Helper()
	report := analysis.Analyze(viewsScenarioInput(), table, testSource(), nil)
	return requireRunReport(t, report, testRunID)
}

// requireGroup returns the agent-type group with the given type, failing when absent.
func requireGroup(t *testing.T, run domain.RunReport, agentType string) domain.AgentTypeGroup {
	t.Helper()
	for _, g := range run.ByAgentType {
		if g.AgentType == agentType {
			return g
		}
	}
	t.Fatalf("no agent-type group %q among %d groups", agentType, len(run.ByAgentType))
	return domain.AgentTypeGroup{}
}

func requireDispatch(t *testing.T, g domain.AgentTypeGroup, id domain.AgentInstanceID) domain.Dispatch {
	t.Helper()
	for _, d := range g.Dispatches {
		if d.Invocation.Actor.Instance == id {
			return d
		}
	}
	t.Fatalf("group %q has no dispatch %s", g.AgentType, id)
	return domain.Dispatch{}
}

func assertUnpricedModels(t *testing.T, name string, got []domain.PricingKey, want ...domain.ModelID) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: unpriced models = %v, want %v", name, got, want)
		return
	}
	for i := range want {
		if got[i] != (domain.PricingKey{Model: want[i]}) {
			t.Errorf("%s: unpriced models = %v, want %v", name, got, want)
			return
		}
	}
}

func assertRatio(t *testing.T, name string, got domain.Ratio, num, den int64) {
	t.Helper()
	if got.Numerator != num || got.Denominator != den {
		t.Errorf("%s = %d/%d, want %d/%d", name, got.Numerator, got.Denominator, num, den)
	}
}
