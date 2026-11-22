package main

import "mosaic-deploy/internal/domain"

// wireMCPEntry is one custom tool entry of the "tools" request: a whole MCP server (tool absent
// or empty) or one tool of it.
type wireMCPEntry struct {
	Server string `json:"server"`
	Tool   string `json:"tool,omitempty"`
}

// fromWireCustomTools decodes the request's custom_tools into the domain answer shape, keeping
// answer order. Keys with no entries are dropped.
func fromWireCustomTools(wire map[string][]wireMCPEntry) domain.CustomToolAnswers {
	out := domain.CustomToolAnswers{}
	for generic, entries := range wire {
		if len(entries) == 0 {
			continue
		}
		answers := make([]domain.MCPEntry, len(entries))
		for i, e := range entries {
			answers[i] = domain.MCPEntry{Server: e.Server, Tool: e.Tool}
		}
		out[generic] = answers
	}
	return out
}
