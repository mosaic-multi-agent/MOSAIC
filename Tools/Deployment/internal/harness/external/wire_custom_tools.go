package external

import "mosaic-deploy/internal/domain"

// wireMCPEntry is one custom tool entry on the wire: a whole MCP server (Tool omitted) or one
// tool of it. Server is the name as the user typed it, not harness syntax.
type wireMCPEntry struct {
	Server string `json:"server"`
	Tool   string `json:"tool,omitempty"`
}

// toWireCustomTools converts the user's custom tool answers to their wire form, keeping answer
// order. It returns nil when there are no answers so the field is omitted.
func toWireCustomTools(answers domain.CustomToolAnswers) map[string][]wireMCPEntry {
	if len(answers) == 0 {
		return nil
	}
	out := make(map[string][]wireMCPEntry, len(answers))
	for generic, entries := range answers {
		if len(entries) == 0 {
			continue
		}
		wire := make([]wireMCPEntry, len(entries))
		for i, e := range entries {
			wire[i] = wireMCPEntry{Server: e.Server, Tool: e.Tool}
		}
		out[generic] = wire
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
