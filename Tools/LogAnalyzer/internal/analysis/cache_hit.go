package analysis

import "mosaic-log-analyzer/internal/domain"

// cacheHitRatio is the share of what could have been read from the cache that
// was: the sum of the actors' cache reads over the sum of their previous calls'
// contexts. A call counts when its previous main call lies in the same session
// with no compaction and no model change between the two; its reads are capped
// at that previous context, and a cold read after a pause counts as a miss. It
// is undefined (the zero Ratio) when no call qualifies.
func cacheHitRatio(actors ...domain.ActorContext) domain.Ratio {
	var read, prevContext int64
	for _, a := range actors {
		r, p := actorCacheHit(a)
		read += r
		prevContext += p
	}
	if prevContext <= 0 {
		return domain.Ratio{}
	}
	return domain.Ratio{Numerator: read, Denominator: prevContext}
}

// actorCacheHit sums, over one actor's qualifying main calls, the capped cache
// read and the previous call's context.
func actorCacheHit(a domain.ActorContext) (read, prevContext int64) {
	sessions := callSessions(a.Calls)
	last := map[string]int{}
	for i, c := range a.Calls {
		if c.Kind != domain.CallMain || c.At.IsZero() {
			continue
		}
		prevIdx, hasPrev := last[sessions[i]]
		last[sessions[i]] = i
		if !hasPrev || !c.Usage.CacheRead.IsPresent() {
			continue
		}
		prev := a.Calls[prevIdx]
		prevCtx, ok := prev.Context.Value()
		if !ok || prev.Partial || prevCtx <= 0 {
			continue
		}
		if c.Model != "" && prev.Model != "" && c.Model != prev.Model {
			continue
		}
		if compactionBetween(a, prevIdx, i) {
			continue
		}
		read += min(c.Usage.CacheRead.ValueOr(0), prevCtx)
		prevContext += prevCtx
	}
	return read, prevContext
}

// runActorContexts lists the orchestrator's and every invocation's context.
func runActorContexts(rc domain.RunContext) []domain.ActorContext {
	return append([]domain.ActorContext{rc.Orchestrator}, rc.Invocations...)
}

// dispatchContexts lists the contexts of the given dispatches' invocations.
func dispatchContexts(rc domain.RunContext, dispatches []domain.Dispatch) []domain.ActorContext {
	var out []domain.ActorContext
	for _, d := range dispatches {
		if ctx, ok := rc.Find(d.Invocation.Actor.Instance); ok {
			out = append(out, ctx)
		}
	}
	return out
}
