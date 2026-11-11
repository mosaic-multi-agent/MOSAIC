package app

// update_tools_reasons.go adds the tools-field changes of an update to the plan: for every
// agent update, the built-in tools the update adds to or removes from the deployed tools field
// become reasons of the plan item, so the plan review and the dry run show them. The change is
// computed by the same pure function that decides the written tools field.

import (
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const (
	toolsAddedReasonPrefix   = "built-in tools added: "
	toolsRemovedReasonPrefix = "built-in tools removed: "
)

// annotateToolsChanges adds the built-in tools an update adds or removes to the reasons of each
// agent update item. Items that are not updates, have no catalog agent, or whose source or
// deployed file cannot be read are left as they are.
func (s *service) annotateToolsChanges(
	p *domain.Plan,
	module domain.HarnessModule,
	customTools map[string]string,
	skippedTools map[string]bool,
	scope domain.Scope,
	deployedReader func(domain.PlanItem) []byte,
	agentSets ...[]domain.Agent,
) {
	if deployedReader == nil {
		return
	}
	agentByKey := map[string]domain.Agent{}
	for _, set := range agentSets {
		for _, a := range set {
			agentByKey[a.Key] = a
		}
	}
	for i := range p.Items {
		item := &p.Items[i]
		if item.Action != domain.ActionUpdate || item.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		agent, ok := agentByKey[item.Ref.Key]
		if !ok {
			continue
		}
		deployed := deployedReader(*item)
		if deployed == nil {
			continue
		}
		src, err := s.deps.Catalog.ReadSource(agent.SourcePath)
		if err != nil {
			continue
		}
		merge, err := transform.ToolsChange(transform.Request{
			Source: src, Deployed: deployed, Kind: domain.ArtifactAgent, Key: agent.Key,
			Module: module, CustomTools: customTools, SkippedTools: skippedTools, Scope: scope,
		})
		if err != nil {
			continue
		}
		if len(merge.AddedBuiltIn) > 0 {
			item.AddReason(toolsAddedReasonPrefix + strings.Join(merge.AddedBuiltIn, ", "))
		}
		if len(merge.RemovedBuiltIn) > 0 {
			item.AddReason(toolsRemovedReasonPrefix + strings.Join(merge.RemovedBuiltIn, ", "))
		}
	}
}
