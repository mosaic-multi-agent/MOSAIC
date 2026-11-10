package digest_test

// Context rewrites in the run digest.
//
// Rules under test:
//   - The session id of a usage entry survives a write and a read, for the
//     orchestrator and for agents; an entry without one stores no session_id.
//   - A digest-loaded run gives the same rewrite figures as the live run (the
//     sessions are part of the stored facts, so the first call of each session
//     is still excluded).
//   - A digest written before the session id was stored still loads, with every
//     call in one session without an id.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// rewriteUsage is a call of the given session with the given token counts.
func rewriteUsage(id string, sec int, session, model string, in, read, write int64) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model: domain.ModelID(model), Harness: domain.HarnessClaudeCode, RecordID: id, At: at(sec), SessionID: session,
		Usage: domain.TokenUsage{
			Input: domain.Tokens(in), CacheRead: domain.Tokens(read), CacheCreation: domain.Tokens(write), Output: domain.Tokens(10),
		},
	}
}

// rewriteAggregate has two interleaved orchestrator sessions and an agent.
//
//	orchestrator s1: o1 (first), o3 cold after 200 s, o5 model change
//	orchestrator s2: o2 (first, cold: excluded), o4 partial rewrite
//	agent coder#1:   a1 (first), a2 cold after 5 min
//
// Read without sessions, o2 would be a rewrite and o3 would follow o2.
func rewriteAggregate() domain.Aggregate {
	orch := []domain.InvocationUsage{
		rewriteUsage("o1", 0, "s1", "model-a", 10, 0, 100_000),
		rewriteUsage("o2", 100, "s2", "model-a", 10, 0, 100_000),
		rewriteUsage("o3", 200, "s1", "model-a", 10, 0, 100_000),
		rewriteUsage("o4", 300, "s2", "model-a", 10, 30_000, 60_000),
		rewriteUsage("o5", 400, "s1", "model-b", 10, 0, 80_000),
	}
	agentUsage := []domain.InvocationUsage{
		rewriteUsage("a1", 0, "s3", "model-a", 10, 0, 50_000),
		rewriteUsage("a2", 300, "s3", "model-a", 10, 0, 50_000),
	}
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	return domain.Aggregate{Runs: []domain.RunAggregate{{
		Run:          domain.NamedRun(runA),
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: orch},
		Agents:       []domain.ActorAggregate{{Actor: agent, Invocations: agentUsage}},
		Activity: domain.RunActivity{Invocations: []domain.Invocation{{
			Actor: agent, State: domain.InvocationClosed, StartedAt: at(0), EndedAt: at(1000), Usage: agentUsage,
		}}},
	}}}
}

func rewritePricing() domain.PricingTable {
	rate := func(s string) domain.Rate {
		r, err := domain.ParseRate(s)
		if err != nil {
			panic(err)
		}
		return r
	}
	set := domain.RateSet{
		Input: rate("3.00"), CacheRead: rate("0.30"), CacheWrite5m: rate("3.75"),
		CacheWrite1h: rate("6.00"), Output: rate("15.00"),
	}
	return domain.NewPricingTable([]domain.ModelPricing{
		{Model: "model-a", Standard: set}, {Model: "model-b", Standard: set},
	})
}

func priceRewriteRun(t *testing.T, agg domain.Aggregate) domain.RunReport {
	t.Helper()
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/x"}
	report, _ := analysis.Price(agg, rewritePricing(), domain.DefaultAnalysisSettings(), src)
	return report.Runs[0]
}

func TestRoundTrip_SessionIdOfEveryUsageEntry_SurvivesForOrchestratorAndAgents(t *testing.T) {
	loaded, findings := roundTrip(t, domain.Digest{Aggregate: rewriteAggregate()})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}

	run := loaded.Aggregate.Runs[0]
	want := map[string][]string{"orchestrator": {"s1", "s2", "s1", "s2", "s1"}, "agent": {"s3", "s3"}}
	for name, usage := range map[string][]domain.InvocationUsage{
		"orchestrator": run.Orchestrator.Invocations,
		"agent":        run.Agents[0].Invocations,
	} {
		requireAtLeast(t, name+" usage", len(usage), len(want[name]))
		for i, s := range want[name] {
			if usage[i].SessionID != s {
				t.Errorf("%s usage %d session = %q, want %q", name, i, usage[i].SessionID, s)
			}
		}
	}
}

func TestWrite_UsageEntryStoresItsSessionIdAndOmitsAnEmptyOne(t *testing.T) {
	agg := rewriteAggregate()
	agg.Runs[0].Orchestrator.Invocations[1].SessionID = ""

	doc := readJSONObject(t, writeDigest(t, domain.Digest{Aggregate: agg}))

	usage := doc["runs"].([]any)[0].(map[string]any)["orchestrator"].(map[string]any)["usage"].([]any)
	if got := usage[0].(map[string]any)["session_id"]; got != "s1" {
		t.Errorf("first entry session_id = %v, want s1", got)
	}
	if _, has := usage[1].(map[string]any)["session_id"]; has {
		t.Errorf("entry without a session stores session_id = %v, want the key omitted", usage[1].(map[string]any)["session_id"])
	}
}

func TestRoundTrip_RewriteFiguresOfTheLoadedRun_EqualThoseOfTheLiveRun(t *testing.T) {
	agg := rewriteAggregate()
	loaded, _ := roundTrip(t, domain.Digest{Aggregate: agg})

	live := priceRewriteRun(t, agg)
	restored := priceRewriteRun(t, loaded.Aggregate)

	// The comparison is vacuous unless the live run has the rewrites described above.
	if n := len(live.Context.Orchestrator.Rewrites); n != 3 {
		t.Fatalf("live orchestrator rewrites = %d, want 3 (cold, partial, model changed)", n)
	}
	if live.Cache.Rewrites.Count() != 4 {
		t.Fatalf("live run rewrites = %d, want 4", live.Cache.Rewrites.Count())
	}
	if !reflect.DeepEqual(restored.Context, live.Context) {
		t.Errorf("digest-loaded context differs from the live run:\n got %+v\nwant %+v", restored.Context, live.Context)
	}
	if !reflect.DeepEqual(restored.Cache.Rewrites, live.Cache.Rewrites) {
		t.Errorf("run rewrite figures differ:\n got %+v\nwant %+v", restored.Cache.Rewrites, live.Cache.Rewrites)
	}
	if !reflect.DeepEqual(restored.OrchestratorCache.Rewrites, live.OrchestratorCache.Rewrites) {
		t.Errorf("orchestrator rewrite figures differ:\n got %+v\nwant %+v", restored.OrchestratorCache.Rewrites, live.OrchestratorCache.Rewrites)
	}
	if len(live.ByAgentType) != 1 || len(restored.ByAgentType) != 1 {
		t.Fatalf("agent types: live %d, loaded %d, want 1 each", len(live.ByAgentType), len(restored.ByAgentType))
	}
	if got, want := restored.ByAgentType[0].Cache.Rewrites, live.ByAgentType[0].Cache.Rewrites; !reflect.DeepEqual(got, want) {
		t.Errorf("agent type rewrite figures differ:\n got %+v\nwant %+v", got, want)
	}
}

// olderRewriteDoc is a 1.3.0 digest (no session ids) with two cold calls.
func olderRewriteDoc() map[string]any {
	call := func(id, when string) map[string]any {
		return map[string]any{
			"record_id": id, "at": when, "model": "model-a", "harness": "claude-code",
			"tokens": map[string]any{
				"input_tokens": 10, "cache_read_tokens": 0, "cache_creation_tokens": 100_000, "output_tokens": 5,
			},
		}
	}
	doc := validDoc("1.3.0")
	doc["runs"] = []any{map[string]any{
		"run_id": runA,
		"orchestrator": map[string]any{"usage": []any{
			call("r1", "2026-10-23T19:13:32Z"), call("r2", "2026-10-23T19:23:32Z"),
		}},
	}}
	return doc
}

func TestRead_DigestWithoutSessionIds_LoadsAndAnalysesEveryCallAsOneSession(t *testing.T) {
	got, findings := readDigest(t, writeRawDoc(t, olderRewriteDoc()))

	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	usage := got.Aggregate.Runs[0].Orchestrator.Invocations
	requireAtLeast(t, "orchestrator usage", len(usage), 2)
	for i, u := range usage {
		if u.SessionID != "" {
			t.Errorf("usage %d session = %q, want none", i, u.SessionID)
		}
	}
	rewrites := priceRewriteRun(t, got.Aggregate).Context.Orchestrator.Rewrites
	if len(rewrites) != 1 || rewrites[0].Kind != domain.RewriteColdIdle || rewrites[0].Pause.Minutes() != 10 {
		t.Errorf("rewrites = %+v, want one cold rewrite after 10 minutes", rewrites)
	}
}
