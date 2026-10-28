package analysis_test

// Tests for the orchestrator's own cache figures (RunReport.OrchestratorCache):
// they cover the orchestrator's usage records only, with unread writes taken
// from its last call in the run.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestOrchestratorCache_CoversOnlyTheOrchestratorsOwnRecords(t *testing.T) {
	orch := []domain.InvocationUsage{ctxCall("o1", 1, 100, 300, 200, 40)}

	run := ccPriceRun(t, orch,
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 1000, 5000, 4000, 10)))

	got := run.OrchestratorCache
	rwRatioIs(t, "hit ratio", got.HitRatio, 300, 600)
	ctxAssertTokens(t, "write tokens", got.WriteTokens, 200)
	ccAssertMoneyNanos(t, "write cost", got.WriteCost, 200*3750)
	ccAssertMoneyNanos(t, "net benefit", got.NetBenefit, 300*2700-200*750)
	if !got.Complete {
		t.Error("orchestrator cache figures are incomplete for a fully priced orchestrator")
	}
}

func TestOrchestratorCache_RunScopeFiguresStayUnchanged(t *testing.T) {
	orch := []domain.InvocationUsage{ctxCall("o1", 1, 100, 300, 200, 0)}

	run := ccPriceRun(t, orch,
		ccDispatch("coder#1", "coder", ctxCall("c1", 1, 100, 700, 200, 0)))

	rwRatioIs(t, "run hit ratio", run.Cache.HitRatio, 1000, 1600)
	ctxAssertTokens(t, "run write tokens", run.Cache.WriteTokens, 400)
	ccAssertMoneyNanos(t, "run net benefit", run.Cache.NetBenefit, 1000*2700-400*750)
	rwRatioIs(t, "orchestrator hit ratio", run.OrchestratorCache.HitRatio, 300, 600)
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
	rwRatioIs(t, "hit ratio", got.HitRatio, 0, 520)
	ctxAssertAbsent(t, "unread write tokens", got.UnreadWriteTokens)
	assertMoneyState(t, "unread write cost", got.UnreadWriteCost, domain.MoneyNoData)
}

func TestOrchestratorCache_NoPerCallRecordsGivesAbsentFigures(t *testing.T) {
	orch := []domain.InvocationUsage{ctxLegacyEntry(1, 0, 1000, 400, 0)}

	got := ccPriceRun(t, orch).OrchestratorCache

	// The legacy entry still feeds the scope totals; only the per-call figures are absent.
	ctxAssertTokens(t, "write tokens", got.WriteTokens, 400)
	rwRatioIs(t, "hit ratio", got.HitRatio, 1000, 1400)
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
