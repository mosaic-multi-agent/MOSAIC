package app

// rebuild_unrecoverable_tools.go decides the target tools of a deployed agent whose source
// harness records no tool list (its descriptor declares tool information unrecoverable). The
// catalog entry wins when it declares tools or a placeholder for an orchestrator; otherwise
// the minimal read-only grant applies and the result says so.

import (
	"fmt"
	"path/filepath"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// rebuildToolsConversion converts the tools of one deployed agent for the target. minimal is
// true when the minimal read-only grant stands in for the tools the source does not record.
func rebuildToolsConversion(in RebuildInput, facts deployedAgentFacts, role domain.AgentRole,
	entry *domain.Agent) (conv ConvertedTools, minimal bool, err error) {
	srcDesc, tgtDesc := in.SourceModule.Descriptor(), in.TargetModule.Descriptor()
	var catalogTools []string
	placeholder := ""
	if entry != nil {
		catalogTools, placeholder = entry.Tools, entry.ToolsPlaceholder
	}
	if !srcDesc.ToolInfoUnrecoverable || len(facts.Grants) > 0 || len(facts.Tools) > 0 {
		conv = ConvertDeployedTools(deployedToolsInput(in, facts, role, placeholder != "", catalogTools))
		return conv, false, nil
	}
	switch {
	case placeholder != "" && role == domain.RoleOrchestrator:
		return ConvertedTools{UsePlaceholder: true}, false, nil
	case len(catalogTools) > 0:
		return catalogToolsConversion(tgtDesc, catalogTools), false, nil
	}
	if _, err := descriptor.RenderMinimalToolGrant(in.TargetModule, in.AgentKey); err != nil {
		return ConvertedTools{}, false, fmt.Errorf("minimal read-only tool grant: %w", err)
	}
	return catalogToolsConversion(tgtDesc, descriptor.MinimalGenericToolSet), true, nil
}

// minimalGrantWarning is the message of the note naming the agents that got the minimal grant.
const minimalGrantWarning = "The source harness does not record tool information; a minimal read-only grant was applied to the agents listed."

// minimalGrantNote warns about the agents rebuilt with the minimal read-only grant, listing
// their destination files in plan order; ok is false when no agent got the grant.
func (r *transformRun) minimalGrantNote() (TransformNote, bool) {
	var paths []string
	for i := range r.agents {
		st := r.agents[i]
		if isWrite(r.plan.Agents[i]) && st.active() && st.built && st.result.MinimalToolGrant {
			paths = append(paths, filepath.ToSlash(r.plan.Agents[i].DestinationPath))
		}
	}
	if len(paths) == 0 {
		return TransformNote{}, false
	}
	return TransformNote{Kind: NoteMinimalToolGrant, Message: minimalGrantWarning, Paths: paths}, true
}

// catalogToolsConversion takes generic tool names as the agent's tools; those the target has
// no mapping for are reported unmapped.
func catalogToolsConversion(tgt *domain.HarnessDescriptor, generics []string) ConvertedTools {
	conv := ConvertedTools{Generic: uniqueStrings(generics)}
	for _, g := range conv.Generic {
		if !hasMappingEntry(tgt, g) {
			conv.Unmapped = append(conv.Unmapped, g)
		}
	}
	return conv
}
