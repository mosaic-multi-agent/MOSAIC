package app

import (
	"bytes"
	"errors"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// errRebuildNoFrontmatter is returned when a deployed source file has no frontmatter block.
var errRebuildNoFrontmatter = errors.New("deployed agent file has no frontmatter block")

// deployedAgentFacts are the stamps and tool names read from a deployed agent's frontmatter.
// A stamp the file does not carry stays empty and is never invented.
type deployedAgentFacts struct {
	ID      string
	Version string
	Role    string
	Tools   []string
	Keys    []string // every frontmatter key of the source, in file order
}

// readDeployedAgentFacts reads the stamps, tool names and key list of a deployed agent.
func readDeployedAgentFacts(fm *docformat.Frontmatter, srcDesc *domain.HarnessDescriptor) deployedAgentFacts {
	facts := deployedAgentFacts{Keys: fm.Keys()}
	facts.ID = readStampScalar(fm, "id")
	facts.Version = readStampScalar(fm, "version")
	facts.Role = readStampScalar(fm, "role")
	if toolsKey := srcDesc.Frontmatter.ToolsKey; toolsKey != "" {
		if v, ok := fm.Get(toolsKey); ok {
			facts.Tools = descriptor.ExtractToolEntries(srcDesc, v)
		}
	}
	return facts
}

// readStampScalar returns the scalar of a stamp under its prefixed name, otherwise its legacy
// name; empty when the file carries neither.
func readStampScalar(fm *docformat.Frontmatter, generic string) string {
	field, _ := agentfields.ByGeneric(generic)
	for _, key := range agentfields.ReadOrder(field) {
		if v, ok := fm.Get(key); ok && v.Kind == domain.KindScalar && v.Scalar != "" {
			return v.Scalar
		}
	}
	return ""
}

// genericToolsValue is the tools value of the generic source: the placeholder when the target
// gets its placeholder expansion, otherwise the converted generic tool names.
func genericToolsValue(conv ConvertedTools, placeholder string) (domain.FieldValue, bool) {
	if conv.UsePlaceholder && placeholder != "" {
		return domain.ScalarValue(placeholder, domain.QuotePlain), true
	}
	items := make([]domain.FieldValue, len(conv.Generic))
	for i, g := range conv.Generic {
		items[i] = domain.ScalarValue(g, domain.QuotePlain)
	}
	return domain.ListValue(items, domain.ListFlow), true
}

// sourceOnlyKeys are the frontmatter keys that belong to the source harness and must not be
// carried into the target file: its tools and model keys, every MOSAIC bookkeeping stamp, and
// the keys its own frontmatter plan sets (for example a mode or invocability flag).
func sourceOnlyKeys(in RebuildInput, facts deployedAgentFacts, role domain.AgentRole, srcFields []domain.FrontmatterField) ([]string, error) {
	srcDesc := in.SourceModule.Descriptor()
	keys := []string{srcDesc.Frontmatter.ToolsKey, srcDesc.Frontmatter.ModelKey}
	for _, f := range agentfields.All() {
		keys = append(keys, f.Deployed, f.Legacy)
	}
	plan, err := in.SourceModule.Frontmatter(domain.FrontmatterRequest{
		Kind:     domain.ArtifactAgent,
		AgentKey: in.AgentKey,
		Source:   srcFields,
		Model:    in.Model,
		Role:     role,
		Versions: domain.VersionStamps{Version: facts.Version, HarnessVersion: srcDesc.TransformVersion},
	})
	if err != nil {
		return nil, err
	}
	for _, f := range plan.Set {
		if !agentfields.IsGenericVocabularyKey(f.Key) {
			keys = append(keys, f.Key)
		}
	}
	return keys, nil
}

// buildRebuildGenericSource turns a deployed agent into a generic source for transform.Apply.
// Content the deployed file owns (project, custom, workflow, infrastructure, protocol, bundle
// and nested user regions) is reinstated by Apply from the deployed file itself, so the
// generic source holds only empty region placeholders. Stamps are carried as the file has
// them; none is invented. dropKeys are the source-only frontmatter keys to remove; name is the
// catalog key written when the source has no name key.
func buildRebuildGenericSource(source []byte, facts deployedAgentFacts, dropKeys []string, tools domain.FieldValue, setTools bool, name string) ([]byte, error) {
	doc, err := ParseGenericSource(source)
	if err != nil {
		return nil, err
	}
	out := doc.Clone()
	fm := out.Frontmatter()
	for _, key := range dropKeys {
		if key != "" {
			fm.Remove(key)
		}
	}
	stamps := []struct{ generic, value string }{{"id", facts.ID}, {"version", facts.Version}, {"role", facts.Role}}
	for _, s := range stamps {
		if s.value != "" {
			fm.Set(s.generic, domain.ScalarValue(s.value, domain.QuotePlain)) //nolint:errcheck // Set only fails for invalid keys.
		}
	}
	// A source harness that writes no name key (the file name carries it) still yields a
	// named agent for targets that write one, as a fresh deploy does.
	if _, has := fm.Get("name"); !has && name != "" {
		fm.Set("name", domain.ScalarValue(name, domain.QuotePlain)) //nolint:errcheck // Set only fails for invalid keys.
	}
	if setTools {
		fm.Set("tools", tools) //nolint:errcheck // Set only fails for invalid keys.
	}

	body := out.Body()
	for _, node := range body.Injections() {
		node.Clear() //nolint:errcheck // Node.Clear always returns nil.
	}
	for _, node := range body.DeployedRegions() {
		clearKeepingNestedInjections(node)
	}

	// Custom regions are reinstated from the deployed file; leaving them in the source would
	// place them twice.
	text := out.Bytes()
	for _, node := range body.CustomRegions() {
		text = bytes.Replace(text, node.Bytes(), nil, 1)
	}
	return text, nil
}

// clearKeepingNestedInjections empties a managed region but keeps empty markers for the
// project regions nested directly inside it, so their deployed content is reinstated.
func clearKeepingNestedInjections(node *docformat.Node) {
	var nested []string
	for _, child := range node.Children() {
		if child.Kind() == docformat.NodeInjection {
			nested = append(nested, child.Name())
		}
	}
	node.Clear() //nolint:errcheck // Node.Clear always returns nil.
	for _, name := range nested {
		node.AppendRegion(docformat.NodeInjection, name, nil) //nolint:errcheck // names are unique in a parsed document.
	}
}
