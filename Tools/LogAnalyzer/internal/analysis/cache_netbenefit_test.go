package analysis_test

// Tests for net cache benefit: per call, (cache read at input rate minus its
// cache-read cost) minus (cache-write cost minus the same tokens at input rate).
// With the flat pricing a call is worth cacheRead*2700 - cacheWrite*750
// nanodollars (cache writes at the assumed 5m rate).

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestNetCacheBenefit_PositiveWhenReadsOutweighWrites(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", ctxCall("c1", 1, 0, 1000, 400, 0)))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ccAssertMoneyNanos(t, "net benefit", got.NetBenefit, 1000*2700-400*750)
	if !got.Complete {
		t.Error("cache figures are incomplete for a fully priced dispatch")
	}
}

func TestNetCacheBenefit_NegativeWhenWritesOutweighReads(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", ctxCall("c1", 1, 0, 100, 1000, 0)))

	got := rwDispatchOf(t, run, "coder#1").Cache.NetBenefit

	ccAssertMoneyNanos(t, "net benefit", got, 100*2700-1000*750)
	if got.Amount.Nanos() >= 0 {
		t.Errorf("net benefit = %d, want negative", got.Amount.Nanos())
	}
}

func TestNetCacheBenefit_OneHourWriteIsChargedAtItsOwnRate(t *testing.T) {
	call := ctxCall("c1", 1, 0, 0, 100, 0)
	call.Usage.CacheCreation5m = domain.Tokens(0)
	call.Usage.CacheCreation1h = domain.Tokens(100)

	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", call))

	// Extra cost of the write over the input price: 100 * (6000 - 3000).
	ccAssertMoneyNanos(t, "net benefit", rwDispatchOf(t, run, "coder#1").Cache.NetBenefit, -100*3000)
}

func TestNetCacheBenefit_SumsOverTheCallsOfEachScope(t *testing.T) {
	run := ccPriceRun(t,
		[]domain.InvocationUsage{ctxCall("o1", 1, 0, 200, 0, 0)},
		ccDispatch("coder#1", "coder",
			ctxCall("c1", 1, 0, 1000, 400, 0),
			ctxCall("c2", 2, 0, 100, 1000, 0)),
		ccDispatch("coder#2", "coder", ctxCall("c3", 1, 0, 0, 0, 0)),
	)

	dispatch := 1000*2700 - 400*750 + 100*2700 - 1000*750
	ccAssertMoneyNanos(t, "dispatch", rwDispatchOf(t, run, "coder#1").Cache.NetBenefit, int64(dispatch))
	ccAssertMoneyNanos(t, "agent type", rwTypeGroup(t, run, "coder").Cache.NetBenefit, int64(dispatch))
	ccAssertMoneyNanos(t, "run incl. orchestrator", run.Cache.NetBenefit, int64(dispatch+200*2700))
}

func TestNetCacheBenefit_UnpricedModelGivesNoValueNeverZero(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", ccUnpriced(ctxCall("c1", 1, 0, 1000, 400, 0))))

	got := rwDispatchOf(t, run, "coder#1").Cache

	assertMoneyState(t, "net benefit", got.NetBenefit, domain.MoneyUnpriced)
	if got.Complete {
		t.Error("cache figures are complete for an unpriced dispatch")
	}
}

func TestNetCacheBenefit_UnpricedCallsAreLeftOutAndMarkTheFigurePartial(t *testing.T) {
	run := ccPriceRun(t, nil,
		ccDispatch("coder#1", "coder",
			ctxCall("c1", 1, 0, 1000, 400, 0),
			ccUnpriced(ctxCall("c2", 2, 0, 5000, 0, 0))),
		ccDispatch("coder#2", "coder", ccUnpriced(ctxCall("c3", 1, 0, 700, 0, 0))),
	)

	mixed := rwDispatchOf(t, run, "coder#1").Cache
	ccAssertMoneyNanos(t, "mixed dispatch", mixed.NetBenefit, 1000*2700-400*750)
	if mixed.Complete {
		t.Error("a dispatch with an unpriced call is marked complete")
	}
	ccAssertMoneyNanos(t, "run", run.Cache.NetBenefit, 1000*2700-400*750)
	if run.Cache.Complete {
		t.Error("a run with an unpriced call is marked complete")
	}
}

func TestNetCacheBenefit_NoPerCallRecordsGivesNoData(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", ctxLegacyEntry(1, 0, 1000, 400, 0)))

	got := rwDispatchOf(t, run, "coder#1").Cache.NetBenefit

	assertMoneyState(t, "net benefit", got, domain.MoneyNoData)
}
