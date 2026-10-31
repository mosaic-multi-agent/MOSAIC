package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// maxCostliestCalls is how many costliest calls an actor lists.
const maxCostliestCalls = 3

// buildActorFindings lists the measured facts about one actor, in the order of
// domain.ActorFindingKind. Every finding is a figure the logs prove.
func buildActorFindings(rr domain.RunReport, ctx domain.ActorContext, tools []domain.ToolFigures) []domain.ActorFinding {
	total := actorTotalCost(rr, ctx.Actor)
	var out []domain.ActorFinding
	out = append(out, costliestCallFindings(ctx, total)...)
	out = append(out, rewriteFinding(ctx)...)
	out = append(out, compactionFinding(ctx)...)
	out = append(out, failedToolFindings(tools)...)
	out = append(out, vsTypeFindings(rr, ctx, total)...)
	return out
}

// actorTotalCost is the actor's priced total, or no data when the report has none.
func actorTotalCost(rr domain.RunReport, a domain.ActorRef) domain.MoneyValue {
	if a.Kind == domain.ActorOrchestrator {
		return rr.Orchestrator.Totals.Money.Total
	}
	for _, ar := range rr.Agents {
		if ar.Actor.Instance == a.Instance {
			return ar.Totals.Money.Total
		}
	}
	return domain.NoMoneyData()
}

func costliestCallFindings(ctx domain.ActorContext, total domain.MoneyValue) []domain.ActorFinding {
	var idx []int
	for i, c := range ctx.Calls {
		if c.Cost.State == domain.MoneyKnown && c.Cost.Amount > 0 {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return ctx.Calls[idx[a]].Cost.Amount > ctx.Calls[idx[b]].Cost.Amount
	})
	if len(idx) > maxCostliestCalls {
		idx = idx[:maxCostliestCalls]
	}
	out := make([]domain.ActorFinding, 0, len(idx))
	for _, i := range idx {
		c := ctx.Calls[i]
		f := domain.ActorFinding{Kind: domain.ActorFindingCostliestCall, Call: i, Cost: c.Cost}
		if total.State == domain.MoneyKnown && total.Amount > 0 {
			f.Share = domain.Ratio{Numerator: int64(c.Cost.Amount), Denominator: int64(total.Amount)}
		}
		out = append(out, f)
	}
	return out
}

func rewriteFinding(ctx domain.ActorContext) []domain.ActorFinding {
	if len(ctx.Rewrites) == 0 {
		return nil
	}
	extra := make([]domain.MoneyValue, len(ctx.Rewrites))
	for i, r := range ctx.Rewrites {
		extra[i] = r.ExtraCost
	}
	sum, _ := domain.SumMoney(extra...)
	return []domain.ActorFinding{{Kind: domain.ActorFindingRewrites, Count: len(ctx.Rewrites), Cost: sum}}
}

func compactionFinding(ctx domain.ActorContext) []domain.ActorFinding {
	count := ctx.LoggedCount() + ctx.HarnessCount() + ctx.DetectedOnlyCount()
	if count == 0 && ctx.CompactionCalls.Count == 0 {
		return nil
	}
	return []domain.ActorFinding{{
		Kind:      domain.ActorFindingCompactions,
		Count:     count,
		Unapplied: ctx.CompactionCalls.UnappliedCount,
		Cost:      ctx.CompactionCalls.Cost,
	}}
}

func failedToolFindings(tools []domain.ToolFigures) []domain.ActorFinding {
	var out []domain.ActorFinding
	for _, t := range tools {
		if t.Errors > 0 {
			out = append(out, domain.ActorFinding{Kind: domain.ActorFindingFailedTool, Tool: t.Tool, Count: t.Errors})
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Tool < out[b].Tool
	})
	return out
}

// minTypeBasis is the fewest agents a type average must rest on to be compared with.
const minTypeBasis = 2

// vsTypeFindings compare an invocation's peak context and cost with the average
// of its agent type. The orchestrator has no type and gets none.
func vsTypeFindings(rr domain.RunReport, ctx domain.ActorContext, total domain.MoneyValue) []domain.ActorFinding {
	if ctx.Actor.Kind != domain.ActorAgentInstance {
		return nil
	}
	var group *domain.AgentTypeGroup
	for i := range rr.ByAgentType {
		if rr.ByAgentType[i].AgentType == ctx.Actor.Type {
			group = &rr.ByAgentType[i]
			break
		}
	}
	if group == nil {
		return nil
	}
	var out []domain.ActorFinding
	if peak := group.PeakContext; peak.Basis >= minTypeBasis && ctx.Peak.Tokens.IsPresent() {
		out = append(out, domain.ActorFinding{
			Kind:        domain.ActorFindingPeakContextVsType,
			Tokens:      ctx.Peak.Tokens,
			TypeAverage: peak.Average,
			TypeMembers: peak.Basis,
		})
	}
	if cost := group.PerInvocation; cost.Basis >= minTypeBasis && total.State == domain.MoneyKnown {
		out = append(out, domain.ActorFinding{
			Kind:        domain.ActorFindingCostVsType,
			Cost:        total,
			TypeCost:    cost.Average,
			TypeMembers: cost.Basis,
		})
	}
	return out
}
