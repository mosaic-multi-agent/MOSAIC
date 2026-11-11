package app

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

// DeployedToolsInput is the input of ConvertDeployedTools. It carries everything the
// conversion rules need, so the conversion does no I/O.
type DeployedToolsInput struct {
	Source *domain.HarnessDescriptor
	Target *domain.HarnessDescriptor
	// Tools are the source tool names with their dispositions in document order.
	Tools []domain.ToolGrant
	Role  domain.AgentRole
	// PlaceholderAgent is true when the agent's catalog tools are a placeholder (main orchestrator).
	PlaceholderAgent bool
	// CatalogTools is the catalog entry's generic tools; nil means no catalog entry.
	CatalogTools []string
	// DeclaredServers are server names from the source harness's MCP configuration;
	// nil = none known.
	DeclaredServers []string
	// SplitAnswers maps an ambiguous source name to the chosen server.
	SplitAnswers map[string]string
}

// allowedNames returns the names of the allow grants in order.
func allowedNames(grants []domain.ToolGrant) []string {
	var names []string
	for _, g := range grants {
		if g.Disposition == domain.Allow {
			names = append(names, g.Name)
		}
	}
	return names
}

// allowGrants wraps names as allow grants.
func allowGrants(names []string) []domain.ToolGrant {
	grants := make([]domain.ToolGrant, len(names))
	for i, n := range names {
		grants[i] = domain.ToolGrant{Name: n, Disposition: domain.Allow}
	}
	return grants
}

// ConvertedTools is the output of ConvertDeployedTools.
type ConvertedTools struct {
	Generic     []string          // generic tools, each once
	CustomTools map[string]string // generic -> server; no longer filled by the MCP name conversion
	TargetTools []string          // allowed target names: kept native names and MCP names in target form
	// TargetDenied lists MCP names in target form to write as deny (permission-shape targets only).
	TargetDenied []string
	// UsePlaceholder is true when the target gets its placeholder expansion; Generic,
	// CustomTools, TargetTools, Dropped and Unmapped then hold only the extras.
	UsePlaceholder bool
	Dropped        []DroppedTool    // includes unknown names and denies the target cannot express
	Unmapped       []string         // Generic entries the target has no mapping for
	Ambiguous      []AmbiguousName  // unresolved ambiguous MCP names; not written
	Conversions    []ToolConversion // one per distinct source name, document order
}

// AmbiguousName is a source MCP name with several possible splits.
type AmbiguousName struct {
	Source      string
	Disposition domain.Disposition
	Splits      []mcpname.Name // shortest server first
}

// ToolConversionRule names the rule that converted one source tool name.
type ToolConversionRule string

const (
	ToolRuleMapped         ToolConversionRule = "mapped"
	ToolRuleCustomTemplate ToolConversionRule = "custom-template"
	ToolRuleDropped        ToolConversionRule = "dropped"
	ToolRuleKept           ToolConversionRule = "kept"
	ToolRuleCustom         ToolConversionRule = "custom"
	ToolRulePlaceholder    ToolConversionRule = "placeholder" // covered by the placeholder expansion
	ToolRuleMCP            ToolConversionRule = "mcp"         // parsed with source forms, written with target forms
	ToolRuleUnknown        ToolConversionRule = "unknown"     // dropped: not a known source tool or MCP name
	ToolRuleAmbiguous      ToolConversionRule = "ambiguous"   // unresolved split; not written
)

// ToolConversion is the audit entry of one distinct source tool name.
type ToolConversion struct {
	Source  string             `json:"source"`
	Rule    ToolConversionRule `json:"rule"`
	Generic []string           `json:"generic,omitempty"` // mapped
	Server  string             `json:"server,omitempty"`  // custom-template, custom, mcp
	Tool    string             `json:"tool,omitempty"`    // mcp: the single-tool part
}

// DroppedTool is a source tool with no target equivalent.
type DroppedTool struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
