package analysis_test

// Tests for the cache-hit ratio and the cache-write figures per dispatch, agent
// type and run. The run scope includes the orchestrator.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestCacheHitRatio_IsCacheReadOverTotalContextPerScope(t *testing.T) {
	run := ccPriceRun(t,
		[]domain.InvocationUsage{ctxCall("o1", 1, 100, 100, 0, 0)},
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 100, 700, 200, 50)),
		ccDispatch("coder#2", "coder", ctxCall("c2", 1, 300, 100, 0, 0)),
	)

	// Output tokens are not part of the context: 100+700+200 = 1000.
	rwRatioIs(t, "dispatch", rwDispatchOf(t, run, "coder#1").Cache.HitRatio, 700, 1000)
	rwRatioIs(t, "agent type", rwTypeGroup(t, run, "coder").Cache.HitRatio, 800, 1400)
	rwRatioIs(t, "run", run.Cache.HitRatio, 900, 1600)
}

func TestCacheHitRatio_DispatchWithoutTokensIsUndefinedNotZero(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder"))

	got := rwDispatchOf(t, run, "coder#1").Cache.HitRatio

	if got.Denominator > 0 {
		t.Errorf("hit ratio = %d/%d, want undefined without any context tokens", got.Numerator, got.Denominator)
	}
}

func TestCacheWrite_TokensAndCostPerScope(t *testing.T) {
	run := ccPriceRun(t,
		[]domain.InvocationUsage{ctxCall("o1", 1, 0, 0, 50, 0)},
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 100, 0, 400, 0)),
		ccDispatch("coder#2", "coder", ctxCall("c2", 1, 100, 0, 100, 0)),
	)

	d := rwDispatchOf(t, run, "coder#1").Cache
	ctxAssertTokens(t, "dispatch write tokens", d.WriteTokens, 400)
	ccAssertMoneyNanos(t, "dispatch write cost", d.WriteCost, 400*3750)

	g := rwTypeGroup(t, run, "coder").Cache
	ctxAssertTokens(t, "agent type write tokens", g.WriteTokens, 500)
	ccAssertMoneyNanos(t, "agent type write cost", g.WriteCost, 500*3750)

	ctxAssertTokens(t, "run write tokens", run.Cache.WriteTokens, 550)
	ccAssertMoneyNanos(t, "run write cost", run.Cache.WriteCost, 550*3750)
}

func TestCacheWrite_OneHourTierIsPricedAtTheOneHourRate(t *testing.T) {
	call := ctxCall("c1", 1, 0, 0, 100, 0)
	call.Usage.CacheCreation5m = domain.Tokens(0)
	call.Usage.CacheCreation1h = domain.Tokens(100)

	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", call))

	ccAssertMoneyNanos(t, "write cost", rwDispatchOf(t, run, "coder#1").Cache.WriteCost, 100*6000)
}

func TestCacheWrite_AbsentWhenNoCallLoggedCacheWriteNeverZero(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", ctxInputOnlyCall("c1", 1, 500, 10)))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertAbsent(t, "write tokens", got.WriteTokens)
	assertMoneyState(t, "write cost", got.WriteCost, domain.MoneyNoData)
}

func TestCacheWrite_UnpricedModelKeepsTokensAndMarksCostUnpriced(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", ccUnpriced(ctxCall("c1", 1, 0, 0, 400, 0))))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertTokens(t, "write tokens", got.WriteTokens, 400)
	assertMoneyState(t, "write cost", got.WriteCost, domain.MoneyUnpriced)
}
