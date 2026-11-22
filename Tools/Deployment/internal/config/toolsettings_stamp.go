package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"mosaic-deploy/internal/domain"
)

// stampTool is the hashed form of one universe tool.
type stampTool struct {
	Name         string `json:"name"`
	Unused       string `json:"unused,omitempty"`
	ByConvention bool   `json:"by_convention,omitempty"`
	Action       string `json:"action,omitempty"`
}

// stampDest is the hashed form of one tool destination.
type stampDest struct {
	Kind      string   `json:"kind"`
	Field     string   `json:"field,omitempty"`
	Format    string   `json:"format,omitempty"`
	Separator string   `json:"separator,omitempty"`
	Names     []string `json:"names,omitempty"`
}

// stampMapping is the hashed form of one tool mapping.
type stampMapping struct {
	Generic      string      `json:"generic"`
	Destinations []stampDest `json:"destinations,omitempty"`
}

// stampMCP is the hashed form of the MCP name forms.
type stampMCP struct {
	ServerForm           string   `json:"server_form,omitempty"`
	ToolForm             string   `json:"tool_form,omitempty"`
	ExtraServerReadForms []string `json:"extra_server_read_forms,omitempty"`
	NormalizeServerName  bool     `json:"normalize_server_name,omitempty"`
}

// stampRule is the hashed form of one permission rule.
type stampRule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

// stampInput is everything the stamp covers, in a fixed field order.
type stampInput struct {
	Shape                  string         `json:"shape,omitempty"`
	Universe               []stampTool    `json:"universe,omitempty"`
	Mappings               []stampMapping `json:"mappings,omitempty"`
	MCP                    stampMCP       `json:"mcp"`
	ReportUnknownNames     bool           `json:"report_unknown_names,omitempty"`
	CustomToolDestinations []stampDest    `json:"custom_tool_destinations,omitempty"`
	PlaceholderExpansion   []string       `json:"placeholder_expansion,omitempty"`
	RulesLeading           []stampRule    `json:"rules_leading,omitempty"`
	RulesTrailing          []stampRule    `json:"rules_trailing,omitempty"`
	ConfigMappings         []stampMapping `json:"config_mappings,omitempty"`
}

// ToolSettingsStamp returns the tool_mappings_version stamp of one harness: a stable hash of
// the tool settings that shape a written tools field, namely the harness's effective tool spec
// and the harness's tool_destinations from the project and user configuration.
//
// The result is 64 lowercase hex characters and never empty. Nil and empty lists hash alike.
// harnessID only selects the configuration entries; it is not part of the hashed value.
func ToolSettingsStamp(harnessID string, tools domain.ToolSpec, project, user ToolDestinationsByHarness) string {
	in := stampInput{
		Shape:                  string(tools.Shape),
		Universe:               stampTools(tools.Universe),
		Mappings:               stampMappings(tools.Mappings),
		MCP:                    stampMCP{tools.MCP.ServerForm, tools.MCP.ToolForm, tools.MCP.ExtraServerReadForms, tools.MCP.NormalizeServerName},
		ReportUnknownNames:     tools.ReportUnknownNames,
		CustomToolDestinations: stampDests(tools.CustomToolDestinations),
		PlaceholderExpansion:   tools.PlaceholderExpansion,
		RulesLeading:           stampRules(tools.Rules.Leading),
		RulesTrailing:          stampRules(tools.Rules.Trailing),
		ConfigMappings:         stampMappings(MergeToolMappings(nil, project[harnessID], user[harnessID])),
	}
	data, err := json.Marshal(in)
	if err != nil {
		// Plain structs and strings cannot fail to marshal.
		panic("config: marshal tool settings stamp: " + err.Error())
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func stampTools(ts []domain.HarnessTool) []stampTool {
	out := make([]stampTool, len(ts))
	for i, t := range ts {
		out[i] = stampTool{Name: t.Name, Unused: string(t.Unused), ByConvention: t.ByConvention, Action: t.Action}
	}
	return out
}

func stampDests(ds []domain.ToolDestination) []stampDest {
	out := make([]stampDest, len(ds))
	for i, d := range ds {
		out[i] = stampDest{Kind: string(d.Kind), Field: d.Field, Format: string(d.Format), Separator: d.Separator, Names: d.Names}
	}
	return out
}

func stampMappings(ms []domain.ToolMapping) []stampMapping {
	out := make([]stampMapping, len(ms))
	for i, m := range ms {
		out[i] = stampMapping{Generic: m.Generic, Destinations: stampDests(m.Destinations)}
	}
	return out
}

func stampRules(rs []domain.PermissionRule) []stampRule {
	out := make([]stampRule, len(rs))
	for i, r := range rs {
		out[i] = stampRule{Action: r.Action, Resource: r.Resource, Effect: string(r.Effect)}
	}
	return out
}
