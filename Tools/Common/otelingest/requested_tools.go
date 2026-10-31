package otelingest

import "encoding/json"

// toolCounts maps a tool name to the number of times a call requested it.
type toolCounts map[string]int64

// requestedTools reads the tool names of the tool_call parts of a chat span's gen_ai.output.messages attribute
// (a JSON string) and counts them by name. Only names are kept: text, arguments and ids are never retained.
// A missing attribute or a value that is not an array of messages gives nil (field absent); messages without
// tool calls give a non-nil empty value.
func requestedTools(attrs map[string]any) *toolCounts {
	raw, ok := attrs["gen_ai.output.messages"].(string)
	if !ok {
		return nil
	}
	var messages []any
	if err := json.Unmarshal([]byte(raw), &messages); err != nil {
		return nil
	}
	counts := toolCounts{}
	for _, m := range messages {
		message, ok := m.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := message["parts"].([]any)
		if !ok {
			continue
		}
		for _, p := range parts {
			part, ok := p.(map[string]any)
			if !ok || part["type"] != "tool_call" {
				continue
			}
			if name, ok := part["name"].(string); ok && name != "" {
				counts[name]++
			}
		}
	}
	return &counts
}
