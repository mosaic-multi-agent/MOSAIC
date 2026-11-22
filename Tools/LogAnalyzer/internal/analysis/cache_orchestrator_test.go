package analysis_test

// Tests for the orchestrator's own cache figures (RunReport.OrchestratorCache):
// they cover the orchestrator's usage records only, with unread writes taken
// from its last call in the run.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestOrchestratorCache_CoversOnlyTheOrchestratorsOwnRecords(t *testing.T) {
	orch := []domain.InvocationUsage{
		ctxCall("o1", 1, 100, 300, 200, 40),
		ctxCall("o2", 2, 100, 400, 100, 40),
	}

	run := ccPriceRun(t, orch,
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 1000, 5000, 4000, 10)))

	got := run.OrchestratorCache
	// o2 reads 400 of o1's 600 context.
	rwRatioIs(t, "hit ratio", got.HitRatio, 400, 600)
	ctxAssertTokens(t, "write tokens", got.WriteTokens, 300)
	ccAssertMoneyNanos(t, "write cost", got.WriteCost, 300*3750)
	ccAssertMoneyNanos(t, "net benefit", got.NetBenefit, 700*2700-300*750)
	if !got.Complete {
		t.Error("orchestrator cache figures are incomplete for a fully priced orchestrator")
	}
}

func TestOrchestratorCache_RunScopeHitRatioUsesPreviousContextWhileWriteFiguresKeep(t *testing.T) {
	// Each second call reads less than its previous call's context, so the
	// previous-context figure differs from reads over the whole context.
	orch := []domain.InvocationUsage{
		ctxCall("o1", 1, 100, 300, 200, 0), // context 600
		ctxCall("o2", 2, 100, 200, 200, 0), // context 500, reads 200
	}

	run := ccPriceRun(t, orch,
		ccDispatch("coder#1", "coder",
			ctxCall("c1", 1, 100, 700, 200, 0), // context 1000
			ctxCall("c2", 2, 100, 500, 200, 0))) // context 800, reads 500

	chAssertExact(t, "run hit ratio", run.Cache.HitRatio, 200+500, 600+1000)
	chAssertExact(t, "orchestrator hit ratio", run.OrchestratorCache.HitRatio, 200, 600)
	ctxAssertTokens(t, "run write tokens", run.Cache.WriteTokens, 800)
	ccAssertMoneyNanos(t, "run net benefit", run.Cache.NetBenefit, 1700*2700-800*750)
}

func TestOrchestratorCache_UnreadWriteIsTheLastCallInTheRunOnly(t *testing.T) {
	orch := []domain.InvocationUsage{
		ctxCall("o1", 1, 10, 0, 500, 0),
		ctxCall("o2", 5, 10, 0, 70, 0),
		ctxCall("o3", 3, 10, 0, 900, 0),
	}

	run := ccPriceRun(t, orch,
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 10, 0, 250, 0)))

	got := run.OrchestratorCache
	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 70)
	ccAssertMoneyNanos(t, "unread write cost", got.UnreadWriteCost, 70*3750)
}

func TestOrchestratorCache_UnreadWriteAbsentWhenLastCallHasNoCacheWrite(t *testing.T) {
	orch := []domain.InvocationUsage{
		ctxCall("o1", 1, 10, 0, 500, 0),
		ctxInputOnlyCall("o2", 2, 10, 5),
	}

	got := ccPriceRun(t, orch).OrchestratorCache

	// The scope is computed (totals present) while the unread figures stay absent,
	// rather than falling back to the earlier call's 500.
	ctxAssertTokens(t, "write tokens", got.WriteTokens, 500)
	// The last call logged no cache read, so it forms no qualifying pair.
	chAssertUndefined(t, "hit ratio", got.HitRatio)
	ctxAssertAbsent(t, "unread write tokens", got.UnreadWriteTokens)
	assertMoneyState(t, "unread write cost", got.UnreadWriteCost, domain.MoneyNoData)
}

func TestOrchestratorCache_NoPerCallRecordsGivesAbsentFigures(t *testing.T) {
	orch := []domain.InvocationUsage{ctxLegacyEntry(1, 0, 1000, 400, 0)}

	got := ccPriceRun(t, orch).OrchestratorCache

	// The legacy entry still feeds the scope totals; only the per-call figures are absent.
	ctxAssertTokens(t, "write tokens", got.WriteTokens, 400)
	// A legacy entry is no per-call record, so it takes part in no pair.
	chAssertUndefined(t, "hit ratio", got.HitRatio)
	ctxAssertAbsent(t, "unread write tokens", got.UnreadWriteTokens)
	assertMoneyState(t, "unread write cost", got.UnreadWriteCost, domain.MoneyNoData)
	assertMoneyState(t, "net benefit", got.NetBenefit, domain.MoneyNoData)
}

func TestOrchestratorCache_UnpricedModelClearsCompletenessAndIsNeverZeroDollars(t *testing.T) {
	orch := []domain.InvocationUsage{ccUnpriced(ctxCall("o1", 1, 0, 1000, 400, 0))}

	got := ccPriceRun(t, orch).OrchestratorCache

	ctxAssertTokens(t, "write tokens", got.WriteTokens, 400)
	assertMoneyState(t, "write cost", got.WriteCost, domain.MoneyUnpriced)
	assertMoneyState(t, "net benefit", got.NetBenefit, domain.MoneyUnpriced)
	assertMoneyState(t, "unread write cost", got.UnreadWriteCost, domain.MoneyUnpriced)
	if got.Complete {
		t.Error("orchestrator cache figures are complete for an unpriced orchestrator")
	}
}
