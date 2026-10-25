package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

// withTokens drops entries that carry no token count at all (for example the
// empty usage of an invocation_end that reported none); such an entry is a
// missing-usage marker, not a request, and must not be priced as one.
func withTokens(usages []domain.InvocationUsage) []domain.InvocationUsage {
	var out []domain.InvocationUsage
	for _, u := range usages {
		if !u.Usage.IsEmpty() {
			out = append(out, u)
		}
	}
	return out
}

// priceUsages prices each request and sums the results into one Totals.
func priceUsages(usages []domain.InvocationUsage, table domain.PricingTable) ([]cost.InvocationCost, domain.Totals) {
	costs := make([]cost.InvocationCost, len(usages))
	tokens := make([]domain.TokenUsage, len(usages))
	var assumed int64
	for i, u := range usages {
		costs[i] = cost.PriceInvocation(u, table)
		tokens[i] = u.Usage
		assumed += costs[i].AssumedTierCacheWrite
	}
	return costs, domain.Totals{
		Tokens:                domain.SumUsage(tokens...),
		Money:                 cost.SumCosts(costs...),
		AssumedTierCacheWrite: assumed,
	}
}

// buildDispatches prices every subagent invocation of the run.
func buildDispatches(invocations []domain.Invocation, table domain.PricingTable) []domain.Dispatch {
	out := make([]domain.Dispatch, len(invocations))
	for i, inv := range invocations {
		usages := withTokens(inv.Usage)
		costs, totals := priceUsages(usages, table)
		models := make(map[domain.ModelID]bool)
		unpriced := make(map[domain.PricingKey]bool)
		for _, c := range costs {
			if c.Model.IsEmpty() {
				continue
			}
			models[c.Model] = true
			if !c.Priced {
				unpriced[domain.PricingKey{Provider: c.Provider, Model: c.Model}] = true
			}
		}
		out[i] = domain.Dispatch{
			Invocation:     inv,
			Models:         sortedModelIDSet(models),
			UnpricedModels: domain.SortedPricingKeys(unpriced),
			Totals:         totals,
			MissingUsage:   len(usages) == 0,
			Effort:         domain.CountEffort(usages...),
		}
	}
	return out
}

// groupFigures summarises a set of dispatches.
func groupFigures(members []domain.Dispatch) domain.GroupFigures {
	moneys := make([]domain.CategoryMoney, len(members))
	tokens := make([]domain.TokenUsage, len(members))
	unpriced := make(map[domain.PricingKey]bool)
	f := domain.GroupFigures{Invocations: len(members)}
	for i, d := range members {
		moneys[i] = d.Totals.Money
		tokens[i] = d.Totals.Tokens
		f.Totals.AssumedTierCacheWrite += d.Totals.AssumedTierCacheWrite
		for _, m := range d.UnpricedModels {
			unpriced[m] = true
		}
		if d.MissingUsage {
			f.MissingUsage++
		}
		if d.Invocation.State == domain.InvocationOpen {
			f.Open++
		}
	}
	f.Totals.Tokens = domain.SumUsage(tokens...)
	f.Totals.Money = sumCategoryMoneys(moneys...)
	f.UnpricedModels = domain.SortedPricingKeys(unpriced)
	return f
}

// moneyStats computes min, max and average of the members' known money totals.
func moneyStats(members []domain.Dispatch) domain.MoneyStats {
	stats := domain.MoneyStats{
		Min: domain.NoMoneyData(), Max: domain.NoMoneyData(), Average: domain.NoMoneyData(),
	}
	allComplete := true
	var sum, lo, hi int64
	for _, d := range members {
		if !d.Totals.Money.Complete {
			allComplete = false
		}
		total := d.Totals.Money.Total
		if total.State != domain.MoneyKnown {
			continue
		}
		n := total.Amount.Nanos()
		if stats.Basis == 0 || n < lo {
			lo = n
		}
		if stats.Basis == 0 || n > hi {
			hi = n
		}
		sum += n
		stats.Basis++
	}
	stats.Complete = stats.Basis == len(members) && allComplete
	if stats.Basis > 0 {
		b := int64(stats.Basis)
		stats.Min = domain.KnownMoney(domain.Money(lo))
		stats.Max = domain.KnownMoney(domain.Money(hi))
		stats.Average = domain.KnownMoney(domain.Money((sum + b/2) / b))
	}
	return stats
}

// typeGroup is the dispatches of one agent type.
type typeGroup struct {
	agentType  string
	dispatches []domain.Dispatch
}

// groupByType groups dispatches by agent type: ascending, the unknown type ""
// last. The dispatch order within a group is preserved.
func groupByType(dispatches []domain.Dispatch) []typeGroup {
	index := make(map[string]int)
	var groups []typeGroup
	for _, d := range dispatches {
		t := d.Invocation.Actor.Type
		i, ok := index[t]
		if !ok {
			i = len(groups)
			index[t] = i
			groups = append(groups, typeGroup{agentType: t})
		}
		groups[i].dispatches = append(groups[i].dispatches, d)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i].agentType, groups[j].agentType
		if (a == "") != (b == "") {
			return b == ""
		}
		return a < b
	})
	return groups
}

// buildAgentTypeGroups builds the by-agent-type view.
func buildAgentTypeGroups(groups []typeGroup) []domain.AgentTypeGroup {
	out := make([]domain.AgentTypeGroup, len(groups))
	for i, g := range groups {
		out[i] = domain.AgentTypeGroup{
			AgentType:     g.agentType,
			Figures:       groupFigures(g.dispatches),
			PerInvocation: moneyStats(g.dispatches),
			Dispatches:    g.dispatches,
		}
	}
	return out
}
