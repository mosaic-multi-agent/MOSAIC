package app

import (
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
	_, doc, err := decodeRebuildSource(in)
	if err != nil {
		return DeployedAgentInspection{}, err
	}
	if !doc.Frontmatter().Present() {
		return DeployedAgentInspection{}, errRebuildNoFrontmatter
	}
	srcDesc := in.SourceModule.Descriptor()
	facts := readDeployedAgentFacts(doc.Frontmatter(), srcDesc)

	entry := lookupCatalogEntry(in.Catalog, facts.ID, in.AgentKey)
	role := rebuildRole(facts.Role, in.FileName, srcDesc, entry)
	refKey := in.AgentKey
	if entry != nil {
		refKey = entry.Key
	}
	conv, _, err := rebuildToolsConversion(in, facts, role, entry)
	if err != nil {
		return DeployedAgentInspection{}, err
	}
	return DeployedAgentInspection{Role: role, RefKey: refKey, Converted: conv}, nil
}
