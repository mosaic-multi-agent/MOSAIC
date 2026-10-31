package app

import (
	"mosaic-deploy/internal/domain"
)

// CatalogAgentLookup is the catalog cross-check of the deployed-agent rebuild. The catalog
// supplies only the role, the generic tool list and the manifest ref key of an agent; content,
// stamps and versions always come from the deployed source file.
type CatalogAgentLookup interface {
	AgentByNumericID(id string) (domain.Agent, bool)
	Agent(key string) (domain.Agent, bool)
}

// RebuildInput is the input of RebuildDeployedAgent.
type RebuildInput struct {
	Source              []byte // deployed source file bytes
	FileName            string // source base name (role rule b)
	AgentKey            string // agentkey.FromFileName(FileName)
	SourceModule        domain.HarnessModule
	TargetModule        domain.HarnessModule
	Catalog             CatalogAgentLookup // nil = no cross-check
	Model               domain.ModelSelection
	CustomTools         map[string]string // custom-tool answers: generic -> server
	SkippedTools        map[string]bool   // generics the user chose to skip
	ToolMappingsVersion string
	Timestamp           string // RFC3339 UTC, passed to transform.Request
}

// RebuildResult is the output of RebuildDeployedAgent.
type RebuildResult struct {
	Output []byte
	Role   domain.AgentRole
	// RefKey is the catalog entry key when found, else AgentKey (manifest ref).
	RefKey string
	// Stamps: Version is the carried mosaic_version (or ""), HarnessVersion the target
	// transform version, InjectionsVersion the role-selected target injection version.
	Stamps        domain.VersionStamp
	Tools         []string // target tool names written in the tools field(s), each once
	Conversions   []ToolConversion
	Dropped       []DroppedTool
	UnmappedTools []string // unmapped generics with no CustomTools answer; each has a gap
	DroppedFields []string // source frontmatter keys not present in the output
	Gaps          []domain.Gap
}
