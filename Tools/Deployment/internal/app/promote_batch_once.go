package app

// promote_batch_once.go asks the questions a batch asks once for all its agents, before any
// agent is written: the layer that receives the agents, each unmapped harness tool name, and
// the recommended tier per distinct source model. Each reports false when the batch is aborted.

import (
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// chooseLayer decides the layer that receives every write of the batch and records it on the
// result. A stack of several layers is asked; one layer, or a catalog without layers, is not.
func (r *promoteBatchRun) chooseLayer() bool {
	layers := r.s.promoteCatalogLayers()
	if len(layers) == 0 {
		r.layer = batchLayer{root: r.s.deps.Catalog.CatalogRoot()}
		return true
	}
	idx := 0
	if len(layers) > 1 {
		opts := make([]domain.Option, 0, len(layers))
		for _, l := range layers {
			opts = append(opts, domain.Option{ID: l.Identity.ID, Label: promoteLayerLabel(l), Description: l.Root})
		}
		ans, ok := r.selectOne(domain.ChoiceQuestion{
			Question: domain.Question{
				ID:      domain.QPromoteTargetCatalog,
				Subject: r.result.AgentsDir,
				Title:   "Select the catalog that receives the promoted agents",
			},
			Options: opts,
		})
		if !ok {
			return false
		}
		idx = -1
		if ans.Status == domain.Answered {
			idx = findPromoteLayer(layers, ans.OptionID)
		}
		if idx < 0 {
			return r.abort()
		}
	}
	l := layers[idx]
	r.layer = batchLayer{root: l.Root, catalogID: l.Identity.ID, label: promoteLayerLabel(l)}
	r.result.TargetCatalogID, r.result.TargetLayer = r.layer.catalogID, r.layer.label
	return true
}

// askUnmappedTools asks, once per distinct unmapped harness tool name across the loaded agents,
// for the generic tool name. A harness whose tool information is unrecoverable asks per agent
// later instead.
func (r *promoteBatchRun) askUnmappedTools() bool {
	if r.desc.ToolInfoUnrecoverable {
		return true
	}
	for _, e := range r.entries {
		if e.src == nil {
			continue
		}
		for _, res := range descriptor.ReverseMapTools(r.desc, e.src.facts.HarnessTools) {
			if res.Outcome == domain.ToolMapped {
				continue
			}
			if _, asked := r.tools[res.Harness]; asked {
				continue
			}
			if !r.askUnmappedTool(res.Harness) {
				return false
			}
		}
	}
	return true
}

// askUnmappedTool asks for one harness tool name and records the answer. Every answer that is
// not a non-empty name keeps the harness name verbatim; skip-all also stops further questions.
func (r *promoteBatchRun) askUnmappedTool(name string) bool {
	if r.toolsSkipped {
		r.tools[name] = batchToolAnswer{}
		return true
	}
	ans, ok := r.askText(promoteToolQuestion(name))
	if !ok {
		return false
	}
	answer := batchToolAnswer{}
	switch ans.Status {
	case domain.Answered:
		answer.generic = strings.TrimSpace(ans.Text)
	case domain.SkippedAll:
		r.toolsSkipped = true
	}
	r.tools[name] = answer
	return true
}

// tierSubject is what the tier question of an agent is about: its source model, or its own key
// when it has none.
func tierSubject(src *promoteSource) string {
	if src.facts.Model != "" {
		return src.facts.Model
	}
	return src.key
}

// askTiers asks the recommended tier once per distinct source model among the agents whose
// source carries none, and once per such agent without a model.
func (r *promoteBatchRun) askTiers() bool {
	for _, e := range r.entries {
		if e.src == nil || e.src.facts.Keys["recommended_tier"] {
			continue
		}
		subject := tierSubject(e.src)
		if _, asked := r.tiers[subject]; asked {
			continue
		}
		if r.tiersSkipped {
			r.tiers[subject] = ""
			continue
		}
		if !r.askTier(subject) {
			return false
		}
	}
	return true
}

// askTier asks the recommended tier for subject and records it; an empty record means none.
func (r *promoteBatchRun) askTier(subject string) bool {
	ans, ok := r.selectOne(domain.ChoiceQuestion{
		Question: domain.Question{
			ID:           domain.QPromoteRecommendedTier,
			Subject:      subject,
			Title:        "Recommended tier for " + subject,
			AllowSkip:    true,
			AllowSkipAll: true,
			AllowCustom:  true,
			CustomPrompt: "Enter a tier",
		},
		Options: r.s.buildPromoteTierOptions(),
	})
	if !ok {
		return false
	}
	tier := ""
	switch ans.Status {
	case domain.Answered:
		tier = ans.OptionID
		if tier == "" {
			tier = ans.Custom
		}
		tier = strings.TrimSpace(tier)
	case domain.SkippedAll:
		r.tiersSkipped = true
	}
	r.tiers[subject] = tier
	return true
}
