package app

// promote_batch_agent.go processes one agent of a batch: the existing-key question, the
// category, the tools of a harness that records none, the optional generic fields, generation
// and the write into the batch layer. The destination and write steps are the single-agent
// promote phases.

import (
	"fmt"
	"strconv"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// processAgent promotes entry e (result index i). It reports false when the batch was aborted,
// in which case nothing was written for the agent.
func (r *promoteBatchRun) processAgent(i int, e *batchEntry) bool {
	src := e.src
	existing := r.findBatchExisting(src.key)
	if existing != nil {
		replace, ok := r.askReplaceExisting(src.key, existing)
		if !ok {
			return false
		}
		if !replace {
			r.settle(i, PromoteBatchSkipped, "kept the existing agent in "+existing.layer, nil)
			return true
		}
	}

	category, err, ok := r.askBatchCategory(src.key)
	if !ok {
		return false
	}
	if err != nil {
		r.settle(i, PromoteBatchFailed, err.Error(), err)
		return true
	}

	tools, verbatim, ok := r.batchAgentTools(src)
	if !ok {
		return false
	}
	recovered, ok := r.batchRecoveredFields(src)
	if !ok {
		return false
	}

	err = r.writeBatchAgent(i, src, category, existing, tools, verbatim, recovered)
	if err != nil {
		r.settle(i, PromoteBatchFailed, err.Error(), err)
	}
	return true
}

// askBatchCategory asks the category of an agent; it offers no skip. The error is the failure of
// this agent (no category chosen, or one that is not offered); ok is false on an abort.
func (r *promoteBatchRun) askBatchCategory(key string) (category string, err error, ok bool) {
	opts, optErr := buildPromoteCategoryOptions(r.layer.root)
	if optErr != nil {
		return "", fmt.Errorf("reading promote category options: %w", optErr), true
	}
	ans, ok := r.selectOne(domain.ChoiceQuestion{
		Question: domain.Question{
			ID:      domain.QPromoteCategory,
			Subject: key,
			Title:   "Select a target category for the promoted agent",
		},
		Options: opts,
	})
	if !ok {
		return "", nil, false
	}
	if ans.Status != domain.Answered {
		return "", ErrPromoteCategoryRequired, true
	}
	for _, o := range opts {
		if o.ID == ans.OptionID && o.ID != "" {
			return o.ID, nil, true
		}
	}
	return "", fmt.Errorf("category %q is not one of the categories of %s", ans.OptionID, r.layerName()), true
}

// layerName names the batch layer in messages.
func (r *promoteBatchRun) layerName() string {
	if r.layer.label == "" {
		return plainCatalogLabel
	}
	return r.layer.label
}

// batchAgentTools returns the generic tools list of an agent. A harness that records no tools
// asks the operator per agent; otherwise the once-per-batch answers apply.
func (r *promoteBatchRun) batchAgentTools(src *promoteSource) (tools, verbatim []string, ok bool) {
	drops, err := classifyPromoteSource(src)
	if err != nil {
		// The failure surfaces again when the agent is generated.
		return nil, nil, true
	}
	if r.desc.ToolInfoUnrecoverable {
		tools, cancelled := r.s.askNonRecoverablePromoteTools(r.ctx, src.key)
		if cancelled || r.ctx.Err() != nil {
			return nil, nil, r.abort()
		}
		return mergeDivertedTools(tools, drops.divertedTools), nil, true
	}
	tools = make([]string, 0, len(src.facts.HarnessTools))
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			tools = append(tools, name)
		}
	}
	for _, res := range descriptor.ReverseMapTools(r.desc, src.facts.HarnessTools) {
		if res.Outcome == domain.ToolMapped {
			add(res.Generic)
			continue
		}
		if generic := r.tools[res.Harness].generic; generic != "" {
			add(generic)
			continue
		}
		add(res.Harness)
		verbatim = append(verbatim, res.Harness)
	}
	return mergeDivertedTools(tools, drops.divertedTools), verbatim, true
}

// batchRecoveredFields collects the generic-only fields of an agent: the tier answered for its
// model and the rationale and skills asked for it. A field its source already carries is left
// as it is.
func (r *promoteBatchRun) batchRecoveredFields(src *promoteSource) (promoteRecoveredFields, bool) {
	var rec promoteRecoveredFields
	if !src.facts.Keys["recommended_tier"] {
		if tier := r.tiers[tierSubject(src)]; tier != "" {
			rec.RecommendedTier = tier
			rec.Supplied = append(rec.Supplied, "recommended_tier")
		}
	}
	if !src.facts.Keys["tier_rationale"] && !r.rationaleSkipped {
		text, ok := r.askOptionalText(domain.QPromoteTierRationale, src.key,
			"Tier rationale for the promoted agent", &r.rationaleSkipped)
		if !ok {
			return rec, false
		}
		if strings.TrimSpace(text) != "" {
			rec.TierRationale = text
			rec.Supplied = append(rec.Supplied, "tier_rationale")
		}
	}
	if !src.facts.Keys["required_skills"] && !r.skillsSkipped {
		text, ok := r.askOptionalText(domain.QPromoteRequiredSkills, src.key,
			"Required skills (comma-separated)", &r.skillsSkipped)
		if !ok {
			return rec, false
		}
		if skills := splitSkills(text); len(skills) > 0 {
			rec.RequiredSkills = skills
			rec.Supplied = append(rec.Supplied, "required_skills")
		}
	}
	return rec, true
}

// askOptionalText asks a text question that can be skipped for this agent or, through latch,
// for the rest of the batch. It returns the answer text, empty unless the question was answered.
func (r *promoteBatchRun) askOptionalText(id domain.QuestionID, key, title string, latch *bool) (string, bool) {
	ans, ok := r.askText(domain.TextQuestion{
		Question: domain.Question{ID: id, Subject: key, Title: title, AllowSkip: true, AllowSkipAll: true},
	})
	if !ok {
		return "", false
	}
	switch ans.Status {
	case domain.Answered:
		return ans.Text, true
	case domain.SkippedAll:
		*latch = true
	}
	return "", true
}

// splitSkills splits a comma-separated answer, trimming entries and dropping empty ones.
func splitSkills(text string) []string {
	var skills []string
	for _, p := range strings.Split(text, ",") {
		if p = strings.TrimSpace(p); p != "" {
			skills = append(skills, p)
		}
	}
	return skills
}

// writeBatchAgent generates the generic agent and writes it into the batch layer. A replacement
// keeps the id of the agent it replaces; a new agent takes the next unused id of the batch.
func (r *promoteBatchRun) writeBatchAgent(
	i int, src *promoteSource, category string, existing *batchExisting,
	tools, verbatim []string, recovered promoteRecoveredFields,
) error {
	dest := promoteDestination{
		targetRoot:      r.layer.root,
		targetCatalogID: r.layer.catalogID,
		category:        category,
		path:            promoteDestinationPath(r.layer.root, category, src.key),
	}
	numericID := ""
	replacedLayer := ""
	if existing != nil {
		numericID, replacedLayer = existing.numericID, existing.layer
		if existing.path != "" {
			dest.existing = &promoteExisting{path: existing.path, numericID: existing.numericID}
		}
	}
	newID := numericID == ""
	if newID {
		numericID = strconv.Itoa(r.nextID)
	}

	drops, err := classifyPromoteSource(src)
	if err != nil {
		return err
	}
	gen, err := assemblePromotedAgent(src, numericID, drops, tools, verbatim, recovered, r.result.ProjectRegions == ProjectRegionsEmpty)
	if err != nil {
		return err
	}
	if err := writePromotedAgent(dest, gen.content); err != nil {
		return err
	}
	if newID {
		r.nextID++
	}

	res := &r.result.Agents[i]
	res.Status, res.ReplacedLayer = PromoteBatchPromoted, replacedLayer
	res.Result = PromoteResult{
		SourcePath:            src.path,
		DestinationPath:       dest.path,
		Key:                   src.key,
		NumericID:             numericID,
		Category:              category,
		TargetCatalogID:       r.layer.catalogID,
		HarnessID:             src.harnessID,
		Tools:                 gen.tools,
		VerbatimTools:         gen.verbatimTools,
		RecoveredFields:       gen.recovered.Supplied,
		StrippedFields:        gen.stripped,
		RemovedCustomRegions:  gen.removedCustom,
		DivertedTools:         src.facts.DivertedTools,
		ExperimentalHarnesses: domain.ExperimentalHarnessesOf(src.module.Ref()),
	}
	return nil
}
