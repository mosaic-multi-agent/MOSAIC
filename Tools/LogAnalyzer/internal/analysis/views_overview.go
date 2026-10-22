package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// buildOverview builds the run overview from the already priced run figures and
// the priced dispatches.
func buildOverview(run domain.RunAggregate, rr domain.RunReport, dispatches []domain.Dispatch, table domain.PricingTable) domain.RunOverview {
	subagents := groupFigures(dispatches)

	var cacheHit domain.Ratio
	if denom, ok := rr.Totals.Tokens.TotalContext().Value(); ok && denom > 0 {
		cacheHit = domain.Ratio{Numerator: rr.Totals.Tokens.CacheRead.ValueOr(0), Denominator: denom}
	}

	return domain.RunOverview{
		Totals:            rr.Totals,
		Orchestrator:      rr.Orchestrator.Totals,
		Subagents:         subagents,
		OrchestratorShare: share(rr.Orchestrator.Totals.Money.Total, rr.Totals.Money.Total),
		SubagentShare:     share(subagents.Totals.Money.Total, rr.Totals.Money.Total),
		ByModel:           buildByModel(run, table),
		CacheHitRatio:     cacheHit,
	}
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

// buildByModel prices every request of the run and groups it by model: models
// ascending, requests without a model last.
func buildByModel(run domain.RunAggregate, table domain.PricingTable) []domain.ModelFigures {
	byModel := make(map[domain.ModelID][]domain.InvocationUsage)
	collect := func(a domain.ActorAggregate) {
		for _, u := range withTokens(a.Invocations) {
			byModel[u.Model] = append(byModel[u.Model], u)
		}
	}
	collect(run.Orchestrator)
	for _, a := range run.Agents {
		collect(a)
	}

	models := make([]domain.ModelID, 0, len(byModel))
	for m := range byModel {
		models = append(models, m)
	}
	sort.Slice(models, func(i, j int) bool {
		a, b := models[i], models[j]
		if a.IsEmpty() != b.IsEmpty() {
			return b.IsEmpty()
		}
		return a < b
	})

	out := make([]domain.ModelFigures, len(models))
	for i, m := range models {
		usages := byModel[m]
		_, totals := priceUsages(usages, table)
		_, priced := table.Lookup(m)
		out[i] = domain.ModelFigures{
			Model:    m,
			Priced:   priced,
			Requests: len(usages),
			Totals:   totals,
		}
	}
	return out
}
