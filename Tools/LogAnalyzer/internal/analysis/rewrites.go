package analysis

import (
	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

// detectRewrites finds the calls of one actor that wrote their context again
// instead of reading it from the cache. The calls are those of callRecords, so
// each rewrite's Call indexes ActorContext.Calls.
//
// Only main calls count. The previous call is the preceding main call of the
// same session (calls without a session id form one session); the first call of
// a session is never a rewrite. A call is analysed when it and the previous call
// have a time, the call has a cache-read figure and the previous call has a
// context. It is a rewrite when it wrote at least half of that context and read
// less than half of it.
func detectRewrites(usage []domain.InvocationUsage, table domain.PricingTable, settings domain.AnalysisSettings) []domain.ContextRewrite {
	calls := callRecords(usage)
	last := map[string]int{}
	var out []domain.ContextRewrite
	for i, u := range calls {
		if callKindOrMain(u.Kind) != domain.CallMain {
			continue
		}
		prev, hasPrev := last[u.SessionID]
		last[u.SessionID] = i
		if !hasPrev {
			continue
		}
		if r, ok := classifyRewrite(u, calls[prev], table, settings); ok {
			r.Call = i
			out = append(out, r)
		}
	}
	return out
}

// classifyRewrite decides whether u rewrote the context that prev held.
func classifyRewrite(u, prev domain.InvocationUsage, table domain.PricingTable, settings domain.AnalysisSettings) (domain.ContextRewrite, bool) {
	if u.At.IsZero() || prev.At.IsZero() || !u.Usage.CacheRead.IsPresent() {
		return domain.ContextRewrite{}, false
	}
	prevCtx, ok := prev.Usage.TotalContext().Value()
	if !ok || prevCtx <= 0 {
		return domain.ContextRewrite{}, false
	}
	read := u.Usage.CacheRead.ValueOr(0)
	write := u.Usage.CacheCreation.ValueOr(0)
	limit := prevCtx * domain.RewriteThresholdPercent
	if write*100 < limit || read*100 >= limit {
		return domain.ContextRewrite{}, false
	}
	r := domain.ContextRewrite{Pause: u.At.Sub(prev.At), Tokens: write}
	switch {
	case u.Model != "" && prev.Model != "" && u.Model != prev.Model:
		r.Kind = domain.RewriteModelChanged
		r.PreviousModel = prev.Model
	case read == 0:
		r.Kind = domain.RewriteColdIdle
	default:
		r.Kind = domain.RewritePartial
	}
	r.Cost, r.ExtraCost = rewriteMoney(u, write, table, settings)
	return r, true
}

// rewriteMoney prices the call's cache writes and the extra they cost over
// reading the same tokens from the cache; both are unpriced without a rate.
func rewriteMoney(u domain.InvocationUsage, write int64, table domain.PricingTable, settings domain.AnalysisSettings) (domain.MoneyValue, domain.MoneyValue) {
	rates, found := cost.RatesFor(u, table)
	if !found {
		return domain.UnpricedMoney(), domain.UnpricedMoney()
	}
	written := priceUsage(u, table, settings).Money.CacheCreation
	if written.State != domain.MoneyKnown {
		return written, written
	}
	atRead := rates.CacheRead.Apply(domain.Tokens(write))
	return written, domain.KnownMoney(written.Amount - atRead.Amount)
}

// sumRewrites folds the rewrites of several actors into one figures block.
func sumRewrites(actors ...[]domain.ContextRewrite) domain.RewriteFigures {
	f := domain.RewriteFigures{Complete: true}
	var tokens []domain.TokenCount
	var costs, extras []domain.MoneyValue
	for _, rewrites := range actors {
		for _, r := range rewrites {
			switch r.Kind {
			case domain.RewriteModelChanged:
				f.ModelChanged++
			case domain.RewriteColdIdle:
				f.ColdIdle++
			default:
				f.Partial++
			}
			tokens = append(tokens, domain.Tokens(r.Tokens))
			costs = append(costs, r.Cost)
			extras = append(extras, r.ExtraCost)
			if r.Cost.State != domain.MoneyKnown {
				f.Complete = false
			}
		}
	}
	f.Tokens = domain.SumTokens(tokens...)
	f.Cost, _ = domain.SumMoney(costs...)
	f.ExtraCost, _ = domain.SumMoney(extras...)
	return f
}
