package codex

// capabilities.go derives the Codex capability settings from an agent's generic tools:
// the web search level, and network access for fetching.
//
// Codex has no per-tool switches. Web search is a level (live or disabled here). Network
// access exists only inside the workspace-write sandbox, so a read-only agent that asks
// for web_fetch cannot be given it; the module raises a notice instead.

import (
	"slices"

	"mosaic-deploy/internal/domain"
)

const (
	webSearchTool   = "web_search"
	webFetchTool    = "web_fetch"
	webSearchField  = "web_search"
	networkTable    = "sandbox_workspace_write"
	networkLeaf     = "network_access"
	sandboxWritable = "workspace-write"
)

// capabilityFields returns the web_search field and, for a write-capable agent whose
// generic tools include web_fetch, the sandbox_workspace_write table granting network
// access. A read-only agent with web_fetch gets no table and one withheld-capability gap.
func capabilityFields(req domain.ToolRequest, sandboxMode string) ([]domain.FrontmatterField, []domain.Gap) {
	level := "disabled"
	if slices.Contains(req.Generic, webSearchTool) {
		level = "live"
	}
	fields := []domain.FrontmatterField{{
		Key:   webSearchField,
		Value: domain.ScalarValue(level, domain.QuotePlain),
	}}

	if !slices.Contains(req.Generic, webFetchTool) {
		return fields, nil
	}
	if sandboxMode != sandboxWritable {
		return fields, []domain.Gap{{
			Kind:    domain.GapToolCapabilityWithheld,
			Subject: webFetchTool,
			Owner:   req.AgentKey,
			Detail:  "Codex gives network access only inside the workspace-write sandbox; this agent's sandbox is read-only, so it gets no network access and web_fetch does not work.",
		}}
	}
	table := domain.MappingValue([]domain.FieldPair{{
		Key:   networkLeaf,
		Value: domain.ScalarValue("true", domain.QuotePlain),
	}})
	return append(fields, domain.FrontmatterField{Key: networkTable, Value: table}), nil
}
