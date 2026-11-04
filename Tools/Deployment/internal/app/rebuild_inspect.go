package app

import (
	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// DeployedAgentInspection is what the transform service learns about a deployed source agent
// before it rebuilds it: the role it will be rebuilt with, the manifest ref key and the tool
// conversion, whose Unmapped list drives the custom-tool questions.
type DeployedAgentInspection struct {
	Role   domain.AgentRole
	RefKey string // catalog entry key when found, else AgentKey (manifest ref)
	// Converted is the tool conversion RebuildDeployedAgent starts from.
	Converted ConvertedTools
}

// InspectDeployedAgent reads a deployed source agent the way RebuildDeployedAgent does and
// returns its role, ref key and tool conversion without building any output. It returns the
// same error as RebuildDeployedAgent for unparseable input or a missing frontmatter block.
func InspectDeployedAgent(in RebuildInput) (DeployedAgentInspection, error) {
	doc, err := docformat.Parse(in.Source)
	if err != nil {
		return DeployedAgentInspection{}, err
	}
	if !doc.Frontmatter().Present() {
		return DeployedAgentInspection{}, errRebuildNoFrontmatter
	}
	srcDesc := in.SourceModule.Descriptor()
	facts := readDeployedAgentFacts(doc.Frontmatter(), srcDesc)

	entry := lookupCatalogEntry(in.Catalog, facts.ID, in.AgentKey)
	role := rebuildRole(facts.Role, in.FileName, entry)
	refKey := in.AgentKey
	var catalogTools []string
	placeholder := ""
	if entry != nil {
		refKey = entry.Key
		catalogTools = entry.Tools
		placeholder = entry.ToolsPlaceholder
	}
	conv := ConvertDeployedTools(DeployedToolsInput{
		Source: srcDesc, Target: in.TargetModule.Descriptor(), Tools: facts.Tools, Role: role,
		PlaceholderAgent: placeholder != "", CatalogTools: catalogTools,
	})
	return DeployedAgentInspection{Role: role, RefKey: refKey, Converted: conv}, nil
}
