package app

// promote_service.go contains the Service.Promote method and the helpers that ask for tools
// and generic-only fields. The per-phase steps live in promote_phase_*.go, the target layer
// rules in promote_target_layer.go; the pure generation core is in promote.go (buildGenericAgent).

import (
	"context"
	"sort"
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// promoteHarnessDropSet is the unified result of the harness drop-key derivation and
// field classification. Produced by a single module.Frontmatter call so both products
// share the same plan.
type promoteHarnessDropSet struct {
	// DropKeys is the harness-specific frontmatter key drop set for PromoteInput.DropKeys.
	DropKeys map[string]bool
	// Classifier is the field classifier for the same harness, ready for unknown-field
	// detection and diverted-field recovery in the same Promote call.
	Classifier descriptor.FieldClassifier
}

// buildPromoteHarnessDropSet derives the harness-specific frontmatter drop set and the
// field classifier from a resolved harness module. Both products derive from a single
// module.Frontmatter call, so the classifier used for unknown-field detection and
// diverted-field recovery later in the Promote call is guaranteed to be built from the
// same plan that shaped the drop set.
//
// The drop set includes:
//   - Descriptor().Frontmatter.ModelKey and ToolsKey — always included, even when those
//     names happen to be MOSAIC generic vocabulary words (e.g. ModelKey="model").
//   - Every key from FrontmatterPlan.Set. FrontmatterSpec.Drop and FrontmatterPlan.Remove
//     name generic fields the harness removes at deploy time, which promote is trying to
//     restore, not remove again (AD-3). Only plan.Set keys enter the drop set.
//
// Keys in promoteProtectedGenericKeys are excluded (AD-4). On Frontmatter failure the
// classifier is built from descriptor keys alone; the error is returned so the caller
// can decide whether to proceed or abort.
//
// Pure with respect to the filesystem: it calls only module.Descriptor() and
// module.Frontmatter(...), both of which the HarnessModule contract requires to be
// deterministic and side-effect free.
func buildPromoteHarnessDropSet(
	module domain.HarnessModule,
	kind domain.ArtifactKind,
	agentKey string,
) (promoteHarnessDropSet, error) {
	desc := module.Descriptor()

	// Request the frontmatter plan for this specific agent kind and key. The plan is used
	// both to populate the field classifier and to derive the plan-Set key contribution to
	// the drop set.
	plan, err := module.Frontmatter(domain.FrontmatterRequest{
		Kind:     kind,
		AgentKey: agentKey,
	})

	// Build the field classifier. On plan failure, use a zero plan so the classifier still
	// contributes descriptor-declared keys. The error is returned alongside the partial set.
	var clfPlan domain.FrontmatterPlan
	if err == nil {
		clfPlan = plan
	}
	clf := descriptor.NewFieldClassifier(desc, clfPlan)

	result := make(map[string]bool)

	// Descriptor-level keys: the key the harness writes the model to and the key it writes
	// its tool entries to. Included even when they match a MOSAIC vocabulary name (e.g.
	// ModelKey="model"), because the harness explicitly uses them and promote must remove
	// them. AD-4 protected keys remain excluded.
	if k := desc.Frontmatter.ModelKey; k != "" && !promoteProtectedGenericKeys[k] {
		result[k] = true
	}
	if k := desc.Frontmatter.ToolsKey; k != "" && !promoteProtectedGenericKeys[k] {
		result[k] = true
	}

	// Plan.Set keys: the harness adds these to the deployed file; promote must remove them.
	// Plan.Remove and Spec.Drop name generic fields the harness removes at deploy time —
	// exactly what promote is trying to restore, not remove again (AD-3). Empty key names
	// and collisions with protected generic keys are silently ignored.
	for _, field := range clfPlan.Set {
		if field.Key == "" || promoteProtectedGenericKeys[field.Key] {
			continue
		}
		result[field.Key] = true
	}

	return promoteHarnessDropSet{DropKeys: result, Classifier: clf}, err
}

// promoteHarnessDropKeys derives the harness-specific frontmatter keys to drop from a
// promoted generic file. It is a thin wrapper over buildPromoteHarnessDropSet for callers
// that only need the drop-key map and not the field classifier.
//
// The plan is requested for the promoted agent's own kind and key so a harness that shapes
// its plan per artifact contributes the keys it would actually have added to this file.
// A module whose Frontmatter call fails contributes only its descriptor keys, and the error
// is returned so the caller can decide; it never panics or silently drops the whole set.
func promoteHarnessDropKeys(
	module domain.HarnessModule,
	kind domain.ArtifactKind,
	agentKey string,
) (map[string]bool, error) {
	set, err := buildPromoteHarnessDropSet(module, kind, agentKey)
	return set.DropKeys, err
}

// promoteRecoveredFields carries the generic-only frontmatter values recovered from the user
// during a promote run. A zero-valued member means "not supplied" and is omitted from the
// promoted file entirely — never written as an empty value.
type promoteRecoveredFields struct {
	RecommendedTier string
	TierRationale   string
	RequiredSkills  []string
	// Supplied names the fields the user actually answered, in ask order, for
	// PromoteResult.RecoveredFields.
	Supplied []string
}

// resolvePromoteTools turns the harness-side tool entries of a deployed agent into the
// authoritative generic tools list.
//
// Entries that reverse-map are resolved silently and never prompt. Each distinct unmapped
// entry is asked once through QPromoteCustomTool, in extraction order, mirroring
// resolveCustomTools. Every non-answered outcome — SkippedOne, SkippedAll, Cancelled, or a
// transport error — carries the harness name into the generic list verbatim and unmodified;
// SkippedAll additionally latches so no further question is asked this run. This is what
// makes the non-interactive path complete with zero user interaction.
//
// The returned tools slice is deduplicated first-seen in extraction order and is always
// non-nil, so the caller may pass it straight into PromoteInput.Tools as an authoritative
// replacement. verbatim lists the entries that were carried through unchanged, in the same
// order, for PromoteResult.VerbatimTools.
func (s *service) resolvePromoteTools(
	ctx context.Context,
	module domain.HarnessModule,
	entries []string,
	subject string,
) (tools []string, verbatim []string) {
	tools = make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))

	resolutions := descriptor.ReverseMapTools(module.Descriptor(), entries)

	skippedAll := false

	addTool := func(name string) {
		if !seen[name] {
			seen[name] = true
			tools = append(tools, name)
		}
	}

	for _, res := range resolutions {
		if res.Outcome == domain.ToolMapped {
			// Resolved via descriptor mapping — add silently, no question asked.
			addTool(res.Generic)
			continue
		}

		// Unmapped entry — ask the user for a generic name, or carry verbatim.
		harnessName := res.Harness

		if skippedAll {
			// SkippedAll was already latched; carry verbatim without asking.
			addTool(harnessName)
			verbatim = append(verbatim, harnessName)
			continue
		}

		q := domain.TextQuestion{
			Question: domain.Question{
				ID:           domain.QPromoteCustomTool,
				Subject:      harnessName,
				Title:        "Generic tool name for harness tool " + harnessName,
				AllowSkip:    true,
				AllowSkipAll: true,
			},
		}
		ans, err := s.deps.Interaction.AskText(ctx, q)
		if err != nil {
			// Transport error — carry verbatim (AD-8).
			addTool(harnessName)
			verbatim = append(verbatim, harnessName)
			continue
		}

		switch ans.Status {
		case domain.Answered:
			if ans.Text != "" {
				addTool(ans.Text)
			} else {
				// Empty answer treated as non-answered — carry verbatim.
				addTool(harnessName)
				verbatim = append(verbatim, harnessName)
			}
		case domain.SkippedAll:
			skippedAll = true
			addTool(harnessName)
			verbatim = append(verbatim, harnessName)
		default:
			// SkippedOne, Cancelled, or any other non-answered status — carry verbatim (AD-8).
			addTool(harnessName)
			verbatim = append(verbatim, harnessName)
		}
	}

	return tools, verbatim
}

// resolvePromoteGenericFields asks for the generic-only frontmatter fields the harness drops
// at deploy time and that cannot be derived from the deployed file: recommended_tier,
// tier_rationale, and required_skills.
//
// A field the source already carries is not asked about — a harness that does not drop it
// carried it through, and re-asking would be noise. Each remaining field is asked at most
// once per run. A field the user does not supply (skipped, cancelled, unresolvable, or
// answered with empty text) is returned as its zero value, which the generation core leaves
// out of the generic agent.
func (s *service) resolvePromoteGenericFields(
	ctx context.Context,
	facts promoteSourceFacts,
	subject string,
) promoteRecoveredFields {
	var result promoteRecoveredFields

	// recommended_tier — only asked when the source does not already carry it.
	if !facts.Keys["recommended_tier"] {
		opts := s.buildPromoteTierOptions()
		ans, err := s.deps.Interaction.SelectOne(ctx, domain.ChoiceQuestion{
			Question: domain.Question{
				ID:          domain.QPromoteRecommendedTier,
				Subject:     subject,
				Title:       "Recommended tier for the promoted agent",
				AllowSkip:   true,
				AllowCustom: true,
			},
			Options: opts,
		})
		if err == nil && ans.Status == domain.Answered {
			tier := ans.OptionID
			if tier == "" {
				tier = ans.Custom
			}
			if strings.TrimSpace(tier) != "" {
				result.RecommendedTier = tier
				result.Supplied = append(result.Supplied, "recommended_tier")
			}
		}
	}

	// tier_rationale — only asked when the source does not already carry it.
	if !facts.Keys["tier_rationale"] {
		ans, err := s.deps.Interaction.AskText(ctx, domain.TextQuestion{
			Question: domain.Question{
				ID:        domain.QPromoteTierRationale,
				Subject:   subject,
				Title:     "Tier rationale for the promoted agent",
				AllowSkip: true,
			},
		})
		if err == nil && ans.Status == domain.Answered && strings.TrimSpace(ans.Text) != "" {
			result.TierRationale = ans.Text
			result.Supplied = append(result.Supplied, "tier_rationale")
		}
	}

	// required_skills — only asked when the source does not already carry it.
	if !facts.Keys["required_skills"] {
		ans, err := s.deps.Interaction.AskText(ctx, domain.TextQuestion{
			Question: domain.Question{
				ID:        domain.QPromoteRequiredSkills,
				Subject:   subject,
				Title:     "Required skills (comma-separated)",
				AllowSkip: true,
			},
		})
		if err == nil && ans.Status == domain.Answered && ans.Text != "" {
			// Split on commas, trim whitespace, discard empty entries (AD-11).
			parts := strings.Split(ans.Text, ",")
			skills := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					skills = append(skills, p)
				}
			}
			if len(skills) > 0 {
				result.RequiredSkills = skills
				result.Supplied = append(result.Supplied, "required_skills")
			}
		}
	}

	return result
}

// buildPromoteTierOptions returns the SelectOne options for QPromoteRecommendedTier, one per
// tier the catalog has discovered, sorted by tier string for determinism.
//
// The source is Deps.Catalog.Tiers() []domain.TierInfo, whose Tier field is the only one used:
// each entry becomes domain.Option{ID: string(ti.Tier), Label: string(ti.Tier)}. TierInfo's
// Rationale and AgentKeys are deliberately not surfaced — Rationale is the existing rationale
// of other agents and must not be presented as a suggestion for this one. A nil or empty
// Tiers() yields an empty option slice, which is legitimate: the question sets AllowCustom so
// a tier no catalog agent declares yet can still be entered.
func (s *service) buildPromoteTierOptions() []domain.Option {
	tiers := s.deps.Catalog.Tiers()
	opts := make([]domain.Option, 0, len(tiers))
	for _, ti := range tiers {
		opts = append(opts, domain.Option{
			ID:    string(ti.Tier),
			Label: string(ti.Tier),
		})
	}
	sort.Slice(opts, func(i, j int) bool {
		return opts[i].ID < opts[j].ID
	})
	return opts
}

// Promote generates a generic agent source file from a single already boundary-tagged
// harness-only agent file, so that a one-off harness-specific agent becomes reusable
// across harnesses through the normal Deploy/Update flow.
//
// Collision policy: an agent with the same key already in the target layer, under any
// category, is refused with ErrPromoteDestinationExists and nothing is written, unless
// req.Overwrite is true. A replacement keeps the replaced agent's numeric id, is written under
// the chosen category, and removes the old file when that is a different one. There is no
// silent overwrite and no automatic renaming.
//
// Registration is automatic: writing a well-formed file into the chosen generic agent
// directory IS the registration — the catalog discovers agents by scanning those directories.
//
// The work is split into phases (source, destination, generation, write) in the
// promote_phase_*.go files; this method sequences them for one file.
func (s *service) Promote(ctx context.Context, req PromoteRequest) (PromoteResult, error) {
	src, err := s.loadPromoteSource(req.FilePath, req.HarnessID)
	if err != nil {
		return PromoteResult{}, err
	}
	defer src.module.Close() //nolint:errcheck // close error must not fail an otherwise complete run

	dest, err := s.resolvePromoteDestination(ctx, req, src.key, req.Category)
	if err != nil {
		return PromoteResult{}, err
	}

	numericID := promoteNumericIDFor(dest, s.deps.Catalog)
	gen, err := s.generatePromotedAgent(ctx, src, numericID, req.EmptyProjectRegions)
	if err != nil {
		return PromoteResult{}, err
	}

	// A dry run validates and computes everything but writes no file; DestinationPath still
	// reflects the path that would have been written.
	if !req.DryRun {
		if err := writePromotedAgent(dest, gen.content); err != nil {
			return PromoteResult{}, err
		}
	}

	return PromoteResult{
		SourcePath:           req.FilePath,
		DestinationPath:      dest.path,
		Key:                  src.key,
		NumericID:            numericID,
		Category:             dest.category,
		DryRun:               req.DryRun,
		TargetCatalogID:      dest.targetCatalogID,
		HarnessID:            req.HarnessID,
		Tools:                gen.tools,
		VerbatimTools:        gen.verbatimTools,
		RecoveredFields:      gen.recovered.Supplied,
		StrippedFields:       gen.stripped,
		RemovedCustomRegions: gen.removedCustom,
		DivertedTools:        src.facts.DivertedTools,

		ExperimentalHarnesses: domain.ExperimentalHarnessesOf(src.module.Ref()),
	}, nil
}
