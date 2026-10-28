package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// buildOverview builds the run overview from the already priced run figures and
// the priced dispatches.
func buildOverview(run domain.RunAggregate, rr domain.RunReport, dispatches []domain.Dispatch, table domain.PricingTable) domain.RunOverview {
	subagents := groupFigures(dispatches)

	ov := domain.RunOverview{
		Totals:            rr.Totals,
		Orchestrator:      rr.Orchestrator.Totals,
		Subagents:         subagents,
		OrchestratorShare: share(rr.Orchestrator.Totals.Money.Total, rr.Totals.Money.Total),
		SubagentShare:     share(subagents.Totals.Money.Total, rr.Totals.Money.Total),
		ByModel:           buildByModel(run, table),
		CacheHitRatio:     cacheHitRatio(rr.Totals.Tokens),
		Outcomes:          buildOutcomeSplit(dispatches),
	}
	if rr.Unattributed != nil {
		ov.Unattributed = &rr.Unattributed.Totals
		ov.UnattributedShare = share(rr.Unattributed.Totals.Money.Total, rr.Totals.Money.Total)
	}
	return ov
}

// share is part/whole in nanodollars; undefined unless whole is a positive known amount.
func share(part, whole domain.MoneyValue) domain.Ratio {
	if whole.State != domain.MoneyKnown || whole.Amount.Nanos() <= 0 {
		return domain.Ratio{}
	}
	r := domain.Ratio{Denominator: whole.Amount.Nanos()}
	if part.State == domain.MoneyKnown {
		r.Numerator = part.Amount.Nanos()
	}
	return r
}

// buildByModel prices every request of the run and groups it by (provider,
// model): models ascending, requests without a model last, and within a model
// the providers ascending with "" first. A row is priced only when the strict
// lookup of its own provider finds an entry.
func buildByModel(run domain.RunAggregate, table domain.PricingTable) []domain.ModelFigures {
	byKey := make(map[domain.PricingKey][]domain.InvocationUsage)
	collect := func(a domain.ActorAggregate) {
		for _, u := range withTokens(a.Invocations) {
			key := domain.PricingKey{Provider: u.Provider, Model: u.Model}
			byKey[key] = append(byKey[key], u)
		}
	}
	collect(run.Orchestrator)
	if run.Unattributed != nil {
		collect(*run.Unattributed)
	}
	for _, a := range run.Agents {
		collect(a)
	}

	keys := make([]domain.PricingKey, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Model.IsEmpty() != b.Model.IsEmpty() {
			return b.Model.IsEmpty()
		}
		return a.Less(b)
	})

	out := make([]domain.ModelFigures, len(keys))
	for i, k := range keys {
		usages := byKey[k]
		costs, totals := priceUsages(usages, table)
		priced := len(costs) > 0
		for _, c := range costs {
			priced = priced && c.Priced
		}
		out[i] = domain.ModelFigures{
			Model:    k.Model,
			Provider: k.Provider,
			Priced:   priced,
			Requests: len(usages),
			Totals:   totals,
		}
	}
	return out
}
