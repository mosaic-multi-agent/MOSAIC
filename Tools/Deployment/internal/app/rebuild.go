package app

import (
	"strings"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// RebuildDeployedAgent builds the target-harness file for one deployed source agent through
// transform.Apply, as deploy does, with content taken from the source file.
//
// Carried unchanged from the source: body, stamps where present, protocol and bundle regions
// with their versions, workflow tables, infrastructure declarations, project and custom
// regions. Taken from the target: tool names, frontmatter shape, role-dependent fields and
// harness-filled regions. The function performs no I/O; the catalog is consulted only through
// in.Catalog for the role, the generic tool list and the manifest ref key.
func RebuildDeployedAgent(in RebuildInput) (RebuildResult, error) {
	canonical, doc, err := decodeRebuildSource(in)
	if err != nil {
		return RebuildResult{}, err
	}
	if !doc.Frontmatter().Present() {
		return RebuildResult{}, errRebuildNoFrontmatter
	}
	srcDesc := in.SourceModule.Descriptor()
	tgtDesc := in.TargetModule.Descriptor()
	facts := readDeployedAgentFacts(doc.Frontmatter(), srcDesc)

	entry := lookupCatalogEntry(in.Catalog, facts.ID, in.AgentKey)
	role := rebuildRole(facts.Role, in.FileName, srcDesc, entry)
	refKey := in.AgentKey
	placeholder, catalogName := "", ""
	if entry != nil {
		refKey, catalogName = entry.Key, entry.Key
		placeholder = entry.ToolsPlaceholder
	}

	conv, minimalGrant, err := rebuildToolsConversion(in, facts, role, entry)
	if err != nil {
		return RebuildResult{}, err
	}
	customTools := mergeCustomTools(conv.CustomTools, in.CustomTools)
	appendTools := append([]string(nil), conv.TargetTools...)
	if conv.UsePlaceholder {
		appendTools = append(appendTools, placeholderExtraTools(in, conv, customTools)...)
	}

	dropKeys, err := sourceOnlyKeys(in, facts, role, doc.Frontmatter().Fields())
	if err != nil {
		return RebuildResult{}, err
	}
	stripped, strippedKeys := sourceStrippedFields(in, role, doc.Frontmatter())
	dropKeys = append(dropKeys, strippedKeys...)
	toolsValue, _ := genericToolsValue(conv, placeholder)
	hadTools := len(facts.Grants) > 0 || len(facts.Tools) > 0 || len(conv.Generic) > 0 || conv.UsePlaceholder
	generic, err := buildRebuildGenericSource(canonical, facts, dropKeys, toolsValue, hadTools, catalogName)
	if err != nil {
		return RebuildResult{}, err
	}

	generic, destGaps, err := carryDestination(in, tgtDesc, generic)
	if err != nil {
		return RebuildResult{}, err
	}

	res, err := transform.Apply(transform.Request{
		Source: generic, Deployed: canonical, Kind: domain.ArtifactAgent, Key: refKey,
		Module: in.TargetModule, Model: in.Model, CustomTools: customTools, SkippedTools: in.SkippedTools, DenyTools: conv.TargetDenied,
		Scope: domain.ScopeProject, PreserveDeployedWorkflows: true,
		ToolMappingsVersion: in.ToolMappingsVersion, Role: role, Timestamp: in.Timestamp,
		InjectionsVersion:             tgtDesc.InjectionsVersion,
		OrchestratorInjectionsVersion: tgtDesc.OrchestratorInjectionsVersion,
		PreserveDeployedManaged:       true, IgnoreDeployedFrontmatter: true, AppendTools: appendTools,
		Op: in.Op, DeployedRaw: in.DestinationRaw,
	})
	if err != nil {
		return RebuildResult{}, err
	}
	res.Report.Gaps = append(res.Report.Gaps, destGaps...)
	result, err := assembleRebuildResult(in, tgtDesc, facts, role, refKey, conv, res)
	if err != nil {
		return RebuildResult{}, err
	}
	result.StrippedFields, result.MinimalToolGrant = stripped, minimalGrant
	return result, nil
}

// mergeCustomTools combines the custom servers found in the source file with the user's
// answers; an answer wins over a found server of the same generic name.
func mergeCustomTools(found, answers map[string]string) map[string]string {
	if len(found) == 0 && len(answers) == 0 {
		return nil
	}
	merged := make(map[string]string, len(found)+len(answers))
	for k, v := range found {
		merged[k] = v
	}
	for k, v := range answers {
		merged[k] = v
	}
	return merged
}

// placeholderExtraTools resolves the generic tools a placeholder agent has beyond the target's
// placeholder expansion to target tool names, so they can be appended to the tools field.
func placeholderExtraTools(in RebuildInput, conv ConvertedTools, customTools map[string]string) []string {
	if len(conv.Generic) == 0 {
		return nil
	}
	result, err := in.TargetModule.Tools(domain.ToolRequest{
		AgentKey: in.AgentKey, Generic: conv.Generic, CustomNames: customTools, SkippedTools: in.SkippedTools,
	})
	if err != nil {
		return nil
	}
	toolsKey := in.TargetModule.Descriptor().Frontmatter.ToolsKey
	var names []string
	for _, field := range result.Fields {
		if field.Key == toolsKey {
			names = append(names, outputToolNames(field.Value)...)
		}
	}
	return names
}

// outputToolNames lists the tool names a tools field value holds, whatever its shape; a
// permission mapping contributes only the allowed names.
func outputToolNames(v domain.FieldValue) []string {
	var names []string
	switch v.Kind {
	case domain.KindList:
		for _, item := range v.Items {
			names = append(names, item.Scalar)
		}
	case domain.KindScalar:
		names = splitCommaNames(v.Scalar)
	case domain.KindMapping:
		for _, pair := range v.Pairs {
			if pair.Value.Scalar == string(domain.Allow) {
				names = append(names, pair.Key)
			}
		}
	}
	return names
}

// assembleRebuildResult builds the result record from the transform output and the facts
// gathered before it.
func assembleRebuildResult(in RebuildInput, tgtDesc *domain.HarnessDescriptor, facts deployedAgentFacts,
	role domain.AgentRole, refKey string, conv ConvertedTools, res transform.Result) (RebuildResult, error) {
	// The output is in the target's on-disk format; decode it to the canonical Markdown form
	// before inspecting the frontmatter.
	decoded := decodeDeployedBytes(res.Output, domain.ArtifactAgent, tgtDesc)
	if decoded.DecodeErr != nil {
		return RebuildResult{}, decoded.DecodeErr
	}
	outDoc, err := ParseGenericSource(decoded.Canonical)
	if err != nil {
		return RebuildResult{}, err
	}
	outFM := outDoc.Frontmatter()

	var tools []string
	if v, ok := outFM.Get(tgtDesc.Frontmatter.ToolsKey); ok && tgtDesc.Frontmatter.ToolsKey != "" {
		tools = uniqueStrings(outputToolNames(v))
	}
	var dropped []string
	present := map[string]bool{}
	for _, key := range outFM.Keys() {
		present[key] = true
	}
	for _, key := range facts.Keys {
		if !present[key] && !agentformat.IsCarriageKey(key) {
			dropped = append(dropped, key)
		}
	}
	gaps := append([]domain.Gap(nil), res.Report.Gaps...)
	var unmapped []string
	for _, g := range conv.Unmapped {
		if _, answered := in.CustomTools[g]; !answered {
			unmapped = append(unmapped, g)
		}
	}
	for _, amb := range conv.Ambiguous {
		unmapped = append(unmapped, amb.Source)
		gaps = append(gaps, domain.Gap{
			Kind: domain.GapUnmappedTool, Subject: amb.Source, Owner: refKey,
			Detail: "unmapped tool: " + amb.Source + " for agent " + refKey,
		})
	}
	stamps := domain.VersionStamp{
		Version: facts.Version, HarnessVersion: tgtDesc.TransformVersion,
		InjectionsVersion: tgtDesc.InjectionsVersion, ToolMappingsVersion: in.ToolMappingsVersion,
	}
	if role == domain.RoleOrchestrator {
		stamps.OrchestratorInjectionsVersion = tgtDesc.OrchestratorInjectionsVersion
	}
	return RebuildResult{
		Output: res.Output, Role: role, RefKey: refKey,
		Stamps: stamps,
		Tools:  tools, Conversions: conv.Conversions, Dropped: conv.Dropped,
		UnmappedTools: unmapped, DroppedFields: dropped, Gaps: uniqueGaps(gaps),
	}, nil
}

// splitCommaNames splits a comma-separated scalar into its trimmed, non-empty names.
func splitCommaNames(s string) []string {
	var names []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			names = append(names, p)
		}
	}
	return names
}

// uniqueStrings removes repeated names, keeping the first occurrence.
func uniqueStrings(names []string) []string {
	seen := make(map[string]bool, len(names))
	var out []string
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// uniqueGaps removes repeated gaps, keeping the first occurrence.
func uniqueGaps(gaps []domain.Gap) []domain.Gap {
	seen := make(map[domain.Gap]bool, len(gaps))
	var out []domain.Gap
	for _, g := range gaps {
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}
