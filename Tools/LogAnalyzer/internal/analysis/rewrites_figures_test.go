package analysis_test

// Rewrite figures per rewrite, per actor, per dispatch, per agent type and per
// run.
//
// Rules under test:
//   - A rewrite's tokens are the call's cache-write tokens and its cost is the
//     money of those writes at the call's own pricing (reported tier split
//     included); extra cost is that cost minus the same tokens at the cache-read rate.
//   - Figures count each kind, sum tokens, cost and extra cost, and are
//     incomplete when a rewrite's model has no pricing.
//   - The orchestrator's figures sit in OrchestratorCache, each dispatch's in its
//     Cache, an agent type's in its group Cache, the run's in Cache.
//
// Prices (nanodollars per token): cache write 3750 (5m) / 6000 (1h), cache read 300.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// crFigureRun has one rewrite of each kind in the orchestrator and one cold
// rewrite in the dispatch coder#1.
//
//	orchestrator: model changed 60,000 tokens, cold 100,000, partial 60,000
//	coder#1:      cold 50,000
func crFigureRun(t *testing.T) domain.RunReport {
	t.Helper()
	return crPrice(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			crCall("o1", 0, "", 10, 0, 100_000),
			crCall("o2", 600, "", 10, 0, 100_000),
			crWithModel(crCall("o3", 700, "", 10, 0, 60_000), crModelB),
			crWithModel(crCall("o4", 800, "", 10, 30_000, 60_000), crModelB),
		},
		agents: []ctxAgent{{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{
			crCall("a1", 0, "", 10, 0, 50_000),
			crCall("a2", 300, "", 10, 0, 50_000),
		}}},
	})
}

func crAssertFigures(t *testing.T, name string, f domain.RewriteFigures, counts [3]int, tokens, cost, extra int64) {
	t.Helper()
	if got := [3]int{f.ModelChanged, f.ColdIdle, f.Partial}; got != counts {
		t.Errorf("%s: counts (model changed, cold, partial) = %v, want %v", name, got, counts)
	}
	ctxAssertTokens(t, name+" tokens", f.Tokens, tokens)
	assertMoneyKnown(t, name+" cost", f.Cost, cost)
	assertMoneyKnown(t, name+" extra cost", f.ExtraCost, extra)
	if !f.Complete {
		t.Errorf("%s: Complete = false, want true (every model is priced)", name)
	}
}

func TestRewriteFigures_EachRewriteCarriesItsTokensCostAndExtraCost(t *testing.T) {
	a := crFigureRun(t).Context.Orchestrator

	crRequireRewrites(t, a, 3)
	want := []struct{ tokens, cost, extra int64 }{
		{100_000, 375_000_000, 345_000_000},
		{60_000, 225_000_000, 207_000_000},
		{60_000, 225_000_000, 207_000_000},
	}
	for i, w := range want {
		r := a.Rewrites[i]
		if r.Tokens != w.tokens {
			t.Errorf("rewrite %d tokens = %d, want %d", i, r.Tokens, w.tokens)
		}
		assertMoneyKnown(t, "cost", r.Cost, w.cost)
		assertMoneyKnown(t, "extra cost", r.ExtraCost, w.extra)
	}
}

func TestRewriteFigures_OrchestratorFiguresSumItsRewrites(t *testing.T) {
	run := crFigureRun(t)

	crAssertFigures(t, "orchestrator", run.OrchestratorCache.Rewrites, [3]int{1, 1, 1},
		220_000, 825_000_000, 759_000_000)
}

func TestRewriteFigures_RunFiguresAddTheOrchestratorAndEveryDispatch(t *testing.T) {
	run := crFigureRun(t)

	crAssertFigures(t, "run", run.Cache.Rewrites, [3]int{1, 2, 1}, 270_000, 1_012_500_000, 931_500_000)
}

func TestRewriteFigures_DispatchAndItsAgentTypeCarryTheirOwnFigures(t *testing.T) {
	run := crFigureRun(t)

	if len(run.ByAgentType) != 1 || len(run.ByAgentType[0].Dispatches) != 1 {
		t.Fatalf("agent types = %d, want one type with one dispatch", len(run.ByAgentType))
	}
	group := run.ByAgentType[0]
	crAssertFigures(t, "dispatch", group.Dispatches[0].Cache.Rewrites, [3]int{0, 1, 0}, 50_000, 187_500_000, 172_500_000)
	crAssertFigures(t, "agent type", group.Cache.Rewrites, [3]int{0, 1, 0}, 50_000, 187_500_000, 172_500_000)
}

func TestRewriteFigures_ActorWithoutARewriteHasNoRewriteTokens(t *testing.T) {
	run := crPrice(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{
		crCall("o1", 0, "", 10, 0, 100_000),
		crCall("o2", 10, "", 10, 95_000, 1_000),
	}})

	f := run.OrchestratorCache.Rewrites

	if f.Count() != 0 {
		t.Errorf("rewrites = %d, want 0", f.Count())
	}
	ctxAssertAbsent(t, "tokens", f.Tokens)
}

func TestRewriteFigures_CostFollowsTheReportedCacheWriteTier(t *testing.T) {
	oneHour := crCall("o2", 600, "", 10, 0, 100_000)
	oneHour.Usage.CacheCreation5m = domain.Tokens(0)
	oneHour.Usage.CacheCreation1h = domain.Tokens(100_000)

	a := crOrch(t, crCall("o1", 0, "", 10, 0, 100_000), oneHour)

	crRequireRewrites(t, a, 1)
	assertMoneyKnown(t, "cost", a.Rewrites[0].Cost, 600_000_000)
	assertMoneyKnown(t, "extra cost", a.Rewrites[0].ExtraCost, 570_000_000)
}

func TestRewriteFigures_UnpricedModelGivesUnpricedMoneyAndIncompleteFigures(t *testing.T) {
	unpriced := func(id string, sec int) domain.InvocationUsage {
		return crWithModel(crCall(id, sec, "", 10, 0, 100_000), "model-unpriced")
	}
	run := crPrice(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{unpriced("o1", 0), unpriced("o2", 600)}})

	r := run.Context.Orchestrator.Rewrites
	f := run.OrchestratorCache.Rewrites

	if len(r) != 1 {
		t.Fatalf("rewrites = %d, want 1", len(r))
	}
	assertMoneyState(t, "rewrite cost", r[0].Cost, domain.MoneyUnpriced)
	assertMoneyState(t, "rewrite extra cost", r[0].ExtraCost, domain.MoneyUnpriced)
	assertMoneyState(t, "figures cost", f.Cost, domain.MoneyUnpriced)
	assertMoneyState(t, "figures extra cost", f.ExtraCost, domain.MoneyUnpriced)
	ctxAssertTokens(t, "figures tokens", f.Tokens, 100_000)
	if f.Complete {
		t.Errorf("Complete = true, want false when a rewrite's model has no pricing")
	}
	if f.ColdIdle != 1 {
		t.Errorf("cold count = %d, want 1: an unpriced rewrite is still counted", f.ColdIdle)
	}
}

func TestRewriteFigures_PricedRewritesKeepTheirMoneyBesideAnUnpricedOne(t *testing.T) {
	run := crPrice(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			crCall("o1", 0, "", 10, 0, 100_000),
			crCall("o2", 600, "", 10, 0, 100_000),
		},
		agents: []ctxAgent{{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{
			crWithModel(crCall("a1", 0, "", 10, 0, 50_000), "model-unpriced"),
			crWithModel(crCall("a2", 300, "", 10, 0, 50_000), "model-unpriced"),
		}}},
	})

	f := run.Cache.Rewrites

	assertMoneyKnown(t, "run cost", f.Cost, 375_000_000)
	ctxAssertTokens(t, "run tokens", f.Tokens, 150_000)
	if f.Complete {
		t.Errorf("run figures Complete = true, want false")
	}
	if !run.OrchestratorCache.Rewrites.Complete {
		t.Errorf("orchestrator figures Complete = false, want true")
	}
}

func TestRewriteFigures_PausesAreTheTrueGapsBetweenCallStarts(t *testing.T) {
	a := crFigureRun(t).Context.Orchestrator

	crRequireRewrites(t, a, 3)
	want := []time.Duration{10 * time.Minute, 100 * time.Second, 100 * time.Second}
	for i, w := range want {
		crAssertPause(t, a.Rewrites[i], w)
	}
}
