package logread

import (
	"encoding/json"

	"mosaic-log-analyzer/internal/domain"
)

// dispatchedInstance reads the agent instance id named in the prompt of a
// subagent-launch tool_input. Only the "prompt" member is decoded. It returns
// the empty id when the tool does not launch a subagent, the input is absent or
// not an object, the prompt is absent or not a string, or it names no id.
func dispatchedInstance(toolName string, toolInput json.RawMessage) domain.AgentInstanceID {
	if len(toolInput) == 0 || !domain.IsSubagentLaunchTool(toolName) {
		return ""
	}
	var input struct {
		Prompt *string `json:"prompt"`
	}
	// A type mismatch on prompt leaves it nil; any other malformation yields no id.
	_ = json.Unmarshal(toolInput, &input)
	if input.Prompt == nil {
		return ""
	}
	id, _ := domain.ExtractInstanceID(*input.Prompt)
	return id
}
