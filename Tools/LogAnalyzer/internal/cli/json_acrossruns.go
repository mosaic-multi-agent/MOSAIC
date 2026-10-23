package cli

import "mosaic-log-analyzer/internal/domain"

// This file encodes the across-runs section of the report JSON: the named runs
// side by side and each agent type summed over the runs. Lists are always
// arrays, never null. The unattributable bucket is never part of it.

type jsonRunSummary struct {
	RunID          string           `json:"run_id"`
	Provisional    bool             `json:"provisional"`
	Totals         jsonTotals       `json:"totals"`
	UnpricedModels []string         `json:"unpriced_models"`
	Subagents      jsonGroupFigures `json:"subagents"`
}

type jsonAgentTypeAcrossRuns struct {
	AgentType         string           `json:"agent_type"`
	Runs              int              `json:"runs"`
	ProvisionalRuns   int              `json:"provisional_runs"`
	Figures           jsonGroupFigures `json:"figures"`
	PerRunInvocations jsonRatio        `json:"per_run_invocations"`
	PerRunTokens      jsonRatio        `json:"per_run_tokens"`
	PerRunMoney       jsonMoneyStats   `json:"per_run_money"`
}

type jsonAcrossRuns struct {
	Runs        []jsonRunSummary          `json:"runs"`
	ByAgentType []jsonAgentTypeAcrossRuns `json:"by_agent_type"`
}

func encodeAcrossRuns(a domain.AcrossRuns) jsonAcrossRuns {
	out := jsonAcrossRuns{
		Runs:        make([]jsonRunSummary, len(a.Runs)),
		ByAgentType: make([]jsonAgentTypeAcrossRuns, len(a.ByAgentType)),
	}
	for i, s := range a.Runs {
		out.Runs[i] = jsonRunSummary{
			RunID:          s.Run.Label(),
			Provisional:    s.Provisional,
			Totals:         encodeTotals(s.Totals),
			UnpricedModels: encodeUnpricedModels(s.UnpricedModels),
			Subagents:      encodeGroupFigures(s.Subagents),
		}
	}
	for i, g := range a.ByAgentType {
		out.ByAgentType[i] = jsonAgentTypeAcrossRuns{
			AgentType:         g.AgentType,
			Runs:              g.Runs,
			ProvisionalRuns:   g.ProvisionalRuns,
			Figures:           encodeGroupFigures(g.Figures),
			PerRunInvocations: encodeRatio(g.PerRunInvocations),
			PerRunTokens:      encodeRatio(g.PerRunTokens),
			PerRunMoney: jsonMoneyStats{
				Basis:    g.PerRunMoney.Basis,
				Min:      encodeMoneyValue(g.PerRunMoney.Min),
				Max:      encodeMoneyValue(g.PerRunMoney.Max),
				Average:  encodeMoneyValue(g.PerRunMoney.Average),
				Complete: g.PerRunMoney.Complete,
			},
		}
	}
	return out
}
