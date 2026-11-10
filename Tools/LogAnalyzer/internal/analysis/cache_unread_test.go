package analysis_test

// Tests for unread cache writes: the cache-write tokens of an invocation's last
// per-call record, with their cost. For the orchestrator only its last call in
// the run counts.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestUnreadCacheWrite_IsTheLastCallsWriteOfTheDispatch(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder",
		ctxCall("c1", 1, 10, 0, 100, 0),
		ctxCall("c2", 2, 10, 0, 300, 0),
		ctxCall("c3", 3, 10, 0, 250, 0)))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 250)
	ccAssertMoneyNanos(t, "unread write cost", got.UnreadWriteCost, 250*3750)
}

func TestUnreadCacheWrite_AbsentWhenTheLastCallWroteNoCache(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder",
		ctxCall("c1", 1, 10, 0, 300, 0),
		ctxInputOnlyCall("c2", 2, 10, 5)))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertAbsent(t, "unread write tokens", got.UnreadWriteTokens)
	assertMoneyState(t, "unread write cost", got.UnreadWriteCost, domain.MoneyNoData)
}

func TestUnreadCacheWrite_AgentTypeSumsTheLastCallOfEachMember(t *testing.T) {
	run := ccPriceRun(t, nil,
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 10, 0, 900, 0), ctxCall("c2", 2, 10, 0, 250, 0)),
		ccDispatch("coder#2", "coder", ctxCall("c3", 1, 10, 0, 100, 0)),
		ccDispatch("coder#3", "coder", ctxCall("c4", 1, 10, 0, 800, 0), ctxInputOnlyCall("c5", 2, 10, 5)),
	)

	got := rwTypeGroup(t, run, "coder").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 350)
	ccAssertMoneyNanos(t, "unread write cost", got.UnreadWriteCost, 350*3750)
}

func TestUnreadCacheWrite_RunCountsOnlyTheOrchestratorsLastCall(t *testing.T) {
	orch := []domain.InvocationUsage{
		ctxCall("o1", 1, 10, 0, 500, 0),
		ctxCall("o2", 2, 10, 0, 70, 0),
	}

	run := ccPriceRun(t, orch,
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 10, 0, 900, 0), ctxCall("c2", 2, 10, 0, 250, 0)),
		ccDispatch("tester#1", "tester", ctxCall("c3", 1, 10, 0, 100, 0)),
	)

	ctxAssertTokens(t, "run unread write tokens", run.Cache.UnreadWriteTokens, 250+100+70)
	ccAssertMoneyNanos(t, "run unread write cost", run.Cache.UnreadWriteCost, (250+100+70)*3750)
}

func TestUnreadCacheWrite_UnpricedModelKeepsTokensAndMarksCostUnpriced(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", ccUnpriced(ctxCall("c1", 1, 10, 0, 250, 0))))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 250)
	assertMoneyState(t, "unread write cost", got.UnreadWriteCost, domain.MoneyUnpriced)
}
