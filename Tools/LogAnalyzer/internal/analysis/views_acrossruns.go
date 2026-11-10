package analysis

import "mosaic-log-analyzer/internal/domain"

// buildAcrossRuns builds the across-runs view over the named runs only. The
// unattributable bucket is not part of runs and so never appears here.
func buildAcrossRuns(runs []domain.RunReport) domain.AcrossRuns {
	var out domain.AcrossRuns
	for _, r := range runs {
		out.Runs = append(out.Runs, domain.RunSummary{
			Run:            r.Run,
			Provisional:    r.Provisional,
			Totals:         r.Totals,
			UnpricedModels: r.UnpricedModels,
			Subagents:      r.Overview.Subagents,
		})
	}
	out.ByAgentType = buildAcrossRunsByType(runs)
	return out
}

// buildAcrossRunsByType sums each agent type over the runs that hold it.
func buildAcrossRunsByType(runs []domain.RunReport) []domain.AgentTypeAcrossRuns {
	byType := make(map[string][]domain.Dispatch)
	perRun := make(map[string][]domain.Dispatch) // one pseudo-dispatch per counted run
	provisional := make(map[string]int)
	runCount := make(map[string]int)
	var order []domain.Dispatch // type carrier for groupByType
	seen := make(map[string]bool)
	for _, r := range runs {
		for _, g := range r.ByAgentType {
			if len(g.Dispatches) == 0 {
				continue
			}
			byType[g.AgentType] = append(byType[g.AgentType], g.Dispatches...)
			perRun[g.AgentType] = append(perRun[g.AgentType], domain.Dispatch{Totals: groupFigures(g.Dispatches).Totals})
			runCount[g.AgentType]++
			if r.Provisional {
				provisional[g.AgentType]++
			}
			if !seen[g.AgentType] {
				seen[g.AgentType] = true
				order = append(order, g.Dispatches[0])
			}
		}
	}
	groups := groupByType(order)
	out := make([]domain.AgentTypeAcrossRuns, len(groups))
	for i, g := range groups {
		t := g.agentType
		figures := groupFigures(byType[t])
		n := int64(runCount[t])
		out[i] = domain.AgentTypeAcrossRuns{
			AgentType:         t,
			Runs:              runCount[t],
			ProvisionalRuns:   provisional[t],
			Figures:           figures,
			PerRunInvocations: domain.Ratio{Numerator: int64(figures.Invocations), Denominator: n},
			PerRunTokens:      domain.Ratio{Numerator: presentTokenSum(figures.Totals.Tokens), Denominator: n},
			PerRunMoney:       moneyStats(perRun[t]),
		}
	}
	return out
}

// presentTokenSum adds the four token categories that are present.
func presentTokenSum(u domain.TokenUsage) int64 {
	return u.Input.ValueOr(0) + u.CacheRead.ValueOr(0) + u.CacheCreation.ValueOr(0) + u.Output.ValueOr(0)
}
