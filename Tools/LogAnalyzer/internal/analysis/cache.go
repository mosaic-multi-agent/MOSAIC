package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

// callRecords lists the per-call usage records (those with a record id and any
// token count) in time order; a record without a time follows the timed ones.
func callRecords(usage []domain.InvocationUsage) []domain.InvocationUsage {
	var calls []domain.InvocationUsage
	for _, u := range usage {
		if u.RecordID != "" && !u.Usage.IsEmpty() {
			calls = append(calls, u)
		}
	}
	sort.SliceStable(calls, func(i, j int) bool {
		a, b := calls[i].At, calls[j].At
		if a.IsZero() != b.IsZero() {
			return b.IsZero()
		}
		return a.Before(b)
	})
	return calls
}

// cacheHitRatio is cache read over total context of the tokens; undefined
// (the zero Ratio) when the context is absent or empty.
func cacheHitRatio(tokens domain.TokenUsage) domain.Ratio {
	if denom, ok := tokens.TotalContext().Value(); ok && denom > 0 {
		return domain.Ratio{Numerator: tokens.CacheRead.ValueOr(0), Denominator: denom}
	}
	return domain.Ratio{}
}

// callNetBenefit is what the call's cache use saved against paying the input
// price for the same tokens: the cache reads' saving minus the cache writes'
// extra cost. NoData when the call has no cache tokens, Unpriced when its model
// has no pricing entry.
func callNetBenefit(u domain.InvocationUsage, table domain.PricingTable, settings domain.AnalysisSettings) domain.MoneyValue {
	if !u.Usage.CacheRead.IsPresent() && !u.Usage.CacheCreation.IsPresent() {
		return domain.NoMoneyData()
	}
	rates, found := cost.RatesFor(u, table)
	if !found {
		return domain.UnpricedMoney()
	}
	priced := priceUsage(u, table, settings)
	var net domain.Money
	if u.Usage.CacheRead.IsPresent() {
		net += rates.Input.Apply(u.Usage.CacheRead).Amount - priced.Money.CacheRead.Amount
	}
	if u.Usage.CacheCreation.IsPresent() {
		net -= priced.Money.CacheCreation.Amount - rates.Input.Apply(u.Usage.CacheCreation).Amount
	}
	return domain.KnownMoney(net)
}

// buildCacheFigures computes the cache figures of one scope. totals are the
// scope's priced totals; actors holds each contributing actor's usage records
// (a dispatch's invocation, or the orchestrator).
func buildCacheFigures(totals domain.Totals, actors [][]domain.InvocationUsage, table domain.PricingTable, settings domain.AnalysisSettings) domain.CacheFigures {
	var nets, unreadCosts []domain.MoneyValue
	var unread []domain.TokenCount
	var rewrites [][]domain.ContextRewrite
	for _, usage := range actors {
		calls := callRecords(usage)
		rewrites = append(rewrites, detectRewrites(usage, table, settings))
		for _, u := range calls {
			nets = append(nets, callNetBenefit(u, table, settings))
		}
		if len(calls) > 0 {
			last := calls[len(calls)-1]
			unread = append(unread, last.Usage.CacheCreation)
			unreadCosts = append(unreadCosts, priceUsage(last, table, settings).Money.CacheCreation)
		}
	}
	net, _ := domain.SumMoney(nets...)
	unreadCost, _ := domain.SumMoney(unreadCosts...)
	return domain.CacheFigures{
		HitRatio:          cacheHitRatio(totals.Tokens),
		WriteTokens:       totals.Tokens.CacheCreation,
		WriteCost:         totals.Money.CacheCreation,
		NetBenefit:        net,
		UnreadWriteTokens: domain.SumTokens(unread...),
		UnreadWriteCost:   unreadCost,
		Complete:          totals.Money.Complete,
		Rewrites:          sumRewrites(rewrites...),
	}
}

// attachDispatchCache sets the cache figures of every dispatch.
func attachDispatchCache(dispatches []domain.Dispatch, table domain.PricingTable, settings domain.AnalysisSettings) {
	for i, d := range dispatches {
		dispatches[i].Cache = buildCacheFigures(d.Totals, [][]domain.InvocationUsage{d.Invocation.Usage}, table, settings)
	}
}

// buildGroupCache sums the cache figures of one agent type's dispatches.
func buildGroupCache(g typeGroup, table domain.PricingTable, settings domain.AnalysisSettings) domain.CacheFigures {
	actors := make([][]domain.InvocationUsage, len(g.dispatches))
	for i, d := range g.dispatches {
		actors[i] = d.Invocation.Usage
	}
	return buildCacheFigures(groupFigures(g.dispatches).Totals, actors, table, settings)
}

// buildOrchestratorCache computes the orchestrator's own cache figures from its
// usage records alone; only its last call in the run counts as unread.
func buildOrchestratorCache(run domain.RunAggregate, totals domain.Totals, table domain.PricingTable, settings domain.AnalysisSettings) domain.CacheFigures {
	return buildCacheFigures(totals, [][]domain.InvocationUsage{run.Orchestrator.Invocations}, table, settings)
}

// buildRunCache computes the run's cache figures: every invocation plus the
// orchestrator (only its last call counts as unread).
func buildRunCache(run domain.RunAggregate, totals domain.Totals, table domain.PricingTable, settings domain.AnalysisSettings) domain.CacheFigures {
	actors := [][]domain.InvocationUsage{run.Orchestrator.Invocations}
	for _, inv := range run.Activity.Invocations {
		actors = append(actors, inv.Usage)
	}
	return buildCacheFigures(totals, actors, table, settings)
}
