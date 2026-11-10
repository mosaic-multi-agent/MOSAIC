package analysis_test

// Builders for the cache figure tests. Usage records come from ctxCall
// (input, cache read, cache write, output); the flat pricing gives input 3000,
// cache read 300 and cache write 3750 (assumed 5m) nanodollars per token.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// ccDispatch is a closed SUCCESS dispatch holding the given per-call records.
func ccDispatch(id domain.AgentInstanceID, typ string, calls ...domain.InvocationUsage) rwDispatch {
	return rwDispatch{
		id: id, typ: typ, state: domain.InvocationClosed, status: "SUCCESS",
		start: 0, end: 100, usage: calls,
	}
}

// ccUnpriced returns the record with its model replaced by one without pricing.
func ccUnpriced(u domain.InvocationUsage) domain.InvocationUsage {
	u.Model = rwUnpricedModel
	return u
}

// ccPriceRun prices a run of the given dispatches and orchestrator records.
func ccPriceRun(t *testing.T, orch []domain.InvocationUsage, dispatches ...rwDispatch) domain.RunReport {
	t.Helper()
	return rwPrice(t, rwAggregate(rwRun{id: ctxRunID, orch: orch, dispatches: dispatches}), ctxRunID)
}

func ccAssertMoneyNanos(t *testing.T, name string, got domain.MoneyValue, want int64) {
	t.Helper()
	assertMoneyKnown(t, name, got, want)
}
