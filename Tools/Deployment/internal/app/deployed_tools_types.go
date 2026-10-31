package app

import "mosaic-deploy/internal/domain"

// DeployedToolsInput is the input of ConvertDeployedTools. It carries everything the
// conversion rules need, so the conversion does no I/O.
type DeployedToolsInput struct {
	Source *domain.HarnessDescriptor
	Target *domain.HarnessDescriptor
	// Tools are the source tool names in document order.
	Tools []string
	Role  domain.AgentRole
	// PlaceholderAgent is true when the agent's catalog tools are a placeholder (main orchestrator).
	PlaceholderAgent bool
	// CatalogTools is the catalog entry's generic tools; nil means no catalog entry.
	CatalogTools []string
}

// ConvertedTools is the output of ConvertDeployedTools.
type ConvertedTools struct {
	Generic     []string          // generic tools (including custom server keys), each once
	CustomTools map[string]string // server -> server
	TargetTools []string          // target-native names kept verbatim
	// UsePlaceholder is true when the target gets its placeholder expansion; Generic,
	// CustomTools, TargetTools, Dropped and Unmapped then hold only the extras.
	UsePlaceholder bool
	Dropped        []DroppedTool
	Unmapped       []string         // Generic entries the target has no mapping for
	Conversions    []ToolConversion // one per distinct source name, document order
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
)

// ToolConversion is the audit entry of one distinct source tool name.
type ToolConversion struct {
	Source  string             `json:"source"`
	Rule    ToolConversionRule `json:"rule"`
	Generic []string           `json:"generic,omitempty"` // mapped
	Server  string             `json:"server,omitempty"`  // custom-template, custom
}

// DroppedTool is a source tool with no target equivalent.
type DroppedTool struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
