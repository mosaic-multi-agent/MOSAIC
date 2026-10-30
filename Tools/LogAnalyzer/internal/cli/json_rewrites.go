package cli

import "mosaic-log-analyzer/internal/domain"

// This file encodes the context rewrite figures of the report JSON: the totals
// inside each cache object and the per-actor list of rewrites.

// jsonRewriteFigures are the rewrite totals of one scope.
type jsonRewriteFigures struct {
	ModelChanged int            `json:"model_changed"`
	ColdIdle     int            `json:"cold_idle"`
	Partial      int            `json:"partial"`
	Tokens       *int64         `json:"tokens"`
	Cost         jsonMoneyValue `json:"cost"`
	ExtraCost    jsonMoneyValue `json:"extra_cost"`
	Complete     bool           `json:"complete"`
}

func encodeRewriteFigures(f domain.RewriteFigures) jsonRewriteFigures {
	return jsonRewriteFigures{
		ModelChanged: f.ModelChanged, ColdIdle: f.ColdIdle, Partial: f.Partial,
		Tokens: encodeTokenCount(f.Tokens), Cost: encodeMoneyValue(f.Cost),
		ExtraCost: encodeMoneyValue(f.ExtraCost), Complete: f.Complete,
	}
}

// jsonRewrite is one rewrite call of an actor.
type jsonRewrite struct {
	At            *string        `json:"at"`
	Kind          string         `json:"kind"`
	Label         string         `json:"label"`
	PauseSeconds  float64        `json:"pause_seconds"`
	PreviousModel string         `json:"previous_model,omitempty"` // model_changed only
	Tokens        int64          `json:"tokens"`
	Cost          jsonMoneyValue `json:"cost"`
	ExtraCost     jsonMoneyValue `json:"extra_cost"`
}

// encodeRewrites lists the actor's rewrites; an actor without any gets [].
func encodeRewrites(a domain.ActorContext) []jsonRewrite {
	out := make([]jsonRewrite, 0, len(a.Rewrites))
	for _, r := range a.Rewrites {
		out = append(out, jsonRewrite{
			At: callTimeOrNull(a.Calls, r.Call), Kind: r.Kind.Key(), Label: r.Label(),
			PauseSeconds: r.Pause.Seconds(), PreviousModel: string(r.PreviousModel),
			Tokens: r.Tokens, Cost: encodeMoneyValue(r.Cost), ExtraCost: encodeMoneyValue(r.ExtraCost),
		})
	}
	return out
}
