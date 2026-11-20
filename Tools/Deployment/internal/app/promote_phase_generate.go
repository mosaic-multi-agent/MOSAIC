package app

// promote_phase_generate.go is the generation phase of promoting one agent: classify the
// source frontmatter, recover the generic tools and the generic-only fields (asking the
// questions that need an answer) and build the generic agent bytes. It writes nothing; the
// numeric id arrives from the caller.

import (
	"context"
	"fmt"

	"mosaic-common/agentformat"
	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// promoteGenerated is the outcome of the generation phase.
type promoteGenerated struct {
	// content is the generic agent file.
	content []byte
	// tools is the resolved generic tools list.
	tools []string
	// verbatimTools lists harness tool names carried through unmapped.
	verbatimTools []string
	// recovered carries the generic-only fields the user supplied.
	recovered promoteRecoveredFields
	// stripped lists the source fields that did not travel to the generic file.
	stripped []StrippedField
}

// promoteDrops is the frontmatter classification of a source.
type promoteDrops struct {
	// dropKeys are the frontmatter keys removed from the generic file.
	dropKeys map[string]bool
	// stripped reports the dropped fields the operator should know about.
	stripped []StrippedField
	// divertedTools are generic tool names recovered from diverted tool-destination fields.
	divertedTools []string
}

// generatePromotedAgent builds the generic agent for src under the given numeric id. It asks
// the tool and generic-field questions through Interaction.
func (s *service) generatePromotedAgent(ctx context.Context, src *promoteSource, numericID string) (promoteGenerated, error) {
	// A plan failure is fatal: proceeding without the full drop-set would leak the harness's
	// own added fields into the generic file.
	drops, err := classifyPromoteSource(src)
	if err != nil {
		return promoteGenerated{}, err
	}

	tools, verbatim := s.resolvePromoteToolList(ctx, src, drops.divertedTools)
	recovered := s.resolvePromoteGenericFields(ctx, src.facts, src.path)
	return assemblePromotedAgent(src, numericID, drops, tools, verbatim, recovered)
}

// assemblePromotedAgent builds the generic agent bytes of src once every question has been
// answered: it reports the carriage-container keys that cannot travel the format change and
// generates the file. It asks nothing and writes nothing.
func assemblePromotedAgent(
	src *promoteSource, numericID string, drops promoteDrops,
	tools, verbatim []string, recovered promoteRecoveredFields,
) (promoteGenerated, error) {
	// Keys held in the carriage container cannot travel the format change; each is reported.
	// StripCarriage is pure, so the source on disk is not affected.
	stripResult, stripErr := agentformat.StripCarriage(src.canonical)
	if stripErr != nil {
		return promoteGenerated{}, fmt.Errorf("stripping carriage container from source: %w", stripErr)
	}
	stripped := drops.stripped
	for _, k := range stripResult.CarriedKeys {
		stripped = append(stripped, StrippedField{Key: k, Reason: StripReasonCarriedKey})
	}

	content, err := buildGenericAgent(PromoteInput{
		Source:    stripResult.Canonical,
		NumericID: numericID,
		Key:       src.key,
		Role:      src.verdict.Meta.Role,
		Version:   src.verdict.Meta.Version,
		DropKeys:  drops.dropKeys,
		Tools:     tools,
		// An empty answer of a source that records no tools must be written as tools: []; an
		// absent tools key is read as "inherit all" by some consumers.
		WriteEmptyToolsList: src.module.Descriptor().ToolInfoUnrecoverable,
		RecommendedTier:     recovered.RecommendedTier,
		TierRationale:       recovered.TierRationale,
		RequiredSkills:      recovered.RequiredSkills,
	})
	if err != nil {
		return promoteGenerated{}, fmt.Errorf("%s: %w", src.path, err)
	}
	return promoteGenerated{
		content:       content,
		tools:         tools,
		verbatimTools: verbatim,
		recovered:     recovered,
		stripped:      stripped,
	}, nil
}

// resolvePromoteToolList returns the generic tools list for src: reverse-mapped harness tools
// or, for a harness that does not record tools, the operator's answer, followed by the tools
// recovered from diverted fields (first-seen deduplicated).
func (s *service) resolvePromoteToolList(ctx context.Context, src *promoteSource, diverted []string) (tools, verbatim []string) {
	if src.module.Descriptor().ToolInfoUnrecoverable {
		tools = s.resolveNonRecoverablePromoteTools(ctx, src.path)
	} else {
		tools, verbatim = s.resolvePromoteTools(ctx, src.module, src.facts.HarnessTools, src.path)
	}

	return mergeDivertedTools(tools, diverted), verbatim
}

// mergeDivertedTools appends the tools recovered from diverted fields to tools, first-seen
// deduplicated.
func mergeDivertedTools(tools, diverted []string) []string {
	seen := make(map[string]bool, len(tools))
	for _, t := range tools {
		seen[t] = true
	}
	for _, t := range diverted {
		if !seen[t] {
			seen[t] = true
			tools = append(tools, t)
		}
	}
	return tools
}

// classifyPromoteSource derives the drop set from the harness contract, then classifies each
// source key: unknown and harness-only keys are dropped and reported, and diverted
// tool-destination fields are recovered into generic tool names or reported as stripped.
func classifyPromoteSource(src *promoteSource) (promoteDrops, error) {
	module := src.module
	harnessDropSet, dropErr := buildPromoteHarnessDropSet(module, domain.ArtifactAgent, src.key)
	if dropErr != nil {
		return promoteDrops{}, fmt.Errorf("deriving harness frontmatter drop-set: %w", dropErr)
	}
	drops := promoteDrops{dropKeys: harnessDropSet.DropKeys}
	clf := harnessDropSet.Classifier

	// Diverted tool-destination keys are harness artifacts whose content is recovered into the
	// generic tools list; the key itself never appears in the generic output.
	for _, df := range descriptor.DivertedToolFields(module.Descriptor()) {
		if !promoteProtectedGenericKeys[df.Key] {
			drops.dropKeys[df.Key] = true
		}
	}

	srcDoc, parseErr := ParseGenericSource(src.canonical)
	if parseErr != nil {
		return drops, nil
	}
	srcFm := srcDoc.Frontmatter()

	// Unknown keys, and harness-specific keys not already in the drop set, do not travel the
	// format-change boundary: drop and report them.
	for _, k := range srcFm.Keys() {
		class := clf.Classify(k)
		switch {
		case class == descriptor.ClassUnknown:
			if !promoteProtectedGenericKeys[k] {
				drops.dropKeys[k] = true
			}
			drops.stripUnknown(srcFm, k)
		case class == descriptor.ClassHarness && !drops.dropKeys[k] && !promoteProtectedGenericKeys[k]:
			drops.dropKeys[k] = true
			drops.stripUnknown(srcFm, k)
		}
	}

	recoverDivertedTools(module, srcFm, &drops)
	return drops, nil
}

// recoverDivertedTools extracts the entries of each diverted tool-destination field of the
// source, reverse-maps them into generic tool names (never prompting), and reports the
// unmapped ones as stripped.
func recoverDivertedTools(module domain.HarnessModule, srcFm *docformat.Frontmatter, drops *promoteDrops) {
	seen := make(map[string]bool)
	for _, df := range descriptor.DivertedToolFields(module.Descriptor()) {
		var fieldValue domain.FieldValue
		if v, ok := srcFm.Get(df.Key); ok {
			fieldValue = v
		}

		var unmapped []string
		for _, entry := range descriptor.ExtractDivertedToolEntries(df, fieldValue) {
			name, ok := descriptor.ReverseMapTool(module.Descriptor(), entry)
			switch {
			case !ok:
				unmapped = append(unmapped, entry)
			case !seen[name]:
				seen[name] = true
				drops.divertedTools = append(drops.divertedTools, name)
			}
		}
		if len(unmapped) > 0 {
			drops.stripped = append(drops.stripped, StrippedField{
				Key:    df.Key,
				Values: unmapped,
				Reason: StripReasonUnmappedDivertedValue,
			})
		}
	}
}

// stripUnknown reports key of fm as a field that did not travel the format-change boundary.
func (d *promoteDrops) stripUnknown(fm *docformat.Frontmatter, key string) {
	val, _ := fm.Get(key)
	d.stripped = append(d.stripped, StrippedField{
		Key:    key,
		Values: descriptor.RenderFieldValues(val),
		Reason: StripReasonUnknownField,
	})
}
