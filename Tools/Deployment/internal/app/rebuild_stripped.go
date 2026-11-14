package app

// rebuild_stripped.go finds the source fields a rebuild from a non-Markdown source harness
// removes: fields only the source harness knows (or nobody knows) and user keys held in the
// carriage container. The container itself never reaches the rebuilt agent.

import (
	"sort"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// sourceStrippedFields lists the stripped fields of a decoded source frontmatter and the keys
// to remove from the generic source because of them. It reports nothing for a source format
// without a closed vocabulary, whose frontmatter travels as it is.
func sourceStrippedFields(in RebuildInput, role domain.AgentRole, fm *docformat.Frontmatter) ([]StrippedField, []string) {
	srcDesc := in.SourceModule.Descriptor()
	if srcDesc == nil {
		return nil, nil
	}
	if traits, err := agentformat.TraitsOf(srcDesc.AgentFormatID); err != nil || !traits.ClosedVocabulary {
		return nil, nil
	}
	var plan domain.FrontmatterPlan
	if p, err := in.SourceModule.Frontmatter(domain.FrontmatterRequest{
		Kind: domain.ArtifactAgent, AgentKey: in.AgentKey, Source: fm.Fields(), Model: in.Model, Role: role,
	}); err == nil {
		plan = p
	}
	classifier := descriptor.NewFieldClassifier(srcDesc, plan)

	var stripped []StrippedField
	var drop []string
	for _, key := range fm.Keys() {
		if agentformat.IsCarriageKey(key) {
			drop = append(drop, key)
			continue
		}
		if key == srcDesc.Frontmatter.ToolsKey || key == srcDesc.Frontmatter.ModelKey {
			continue
		}
		if class := classifier.Classify(key); class == descriptor.ClassHarness || class == descriptor.ClassUnknown {
			v, _ := fm.Get(key)
			stripped = append(stripped, StrippedField{
				Key: key, Values: descriptor.RenderFieldValues(v), Reason: StripReasonUnknownField,
			})
			drop = append(drop, key)
		}
	}
	return append(stripped, carriedKeyFields(fm)...), drop
}

// carriedKeyFields lists the user keys held in the carriage container, in ascending key order:
// value-carried keys with their rendered values, marker-carried keys without.
func carriedKeyFields(fm *docformat.Frontmatter) []StrippedField {
	var out []StrippedField
	if v, ok := fm.Get(agentformat.CarriageValuesKey); ok && v.Kind == domain.KindMapping {
		for _, pair := range v.Pairs {
			out = append(out, StrippedField{
				Key: pair.Key, Values: descriptor.RenderFieldValues(pair.Value), Reason: StripReasonCarriedKey,
			})
		}
	}
	if v, ok := fm.Get(agentformat.CarriageMarkersKey); ok && v.Kind == domain.KindList {
		for _, item := range v.Items {
			out = append(out, StrippedField{Key: item.Scalar, Reason: StripReasonCarriedKey})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
