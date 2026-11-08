package claudecode_test

// Stream-json fake-CLI scenarios and line builders for the Claude Code
// adapter tests. Scenarios register themselves into helperScenarios from
// init so the shared helper file only needs the common ones.

import (
	"encoding/json"
	"os"
)

const streamProtocolOK = `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`

const streamInitLine = `{"type":"system","subtype":"init","session_id":"s1","tools":["Read"]}`

const streamToolUseLine = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Read","input":{"file_path":"a.txt"}}]},"session_id":"s1"}`

const streamToolResultLine = `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"hello"}]},"session_id":"s1"}`

// streamAssistantTextLine renders an assistant message carrying one text block.
func streamAssistantTextLine(text string) string {
	line := map[string]any{
		"type":       "assistant",
		"session_id": "s1",
		"message": map[string]any{
			"role":    "assistant",
			"content": []any{map[string]any{"type": "text", "text": text}},
		},
	}
	data, _ := json.Marshal(line)
	return string(data)
}

// streamResultLine renders a final result line carrying result as its text.
func streamResultLine(isError bool, subtype, result string) string {
	line := map[string]any{
		"type":           "result",
		"subtype":        subtype,
		"is_error":       isError,
		"result":         result,
		"session_id":     "s1",
		"total_cost_usd": 0.01,
	}
	data, _ := json.Marshal(line)
	return string(data)
}

// streamResultLineWithoutText renders a result line with no result field, as
// the CLI prints for error subtypes such as error_max_turns.
func streamResultLineWithoutText(subtype string) string {
	line := map[string]any{
		"type":       "result",
		"subtype":    subtype,
		"is_error":   true,
		"session_id": "s1",
	}
	data, _ := json.Marshal(line)
	return string(data)
}

// writeStreamLines prints each line followed by a newline, as the CLI does.
func writeStreamLines(lines ...string) {
	for _, l := range lines {
		os.Stdout.WriteString(l + "\n") //nolint:errcheck
	}
}

func init() {
	helperScenarios["stream-multi-result"] = func() {
		writeStreamLines(streamInitLine,
			streamResultLine(false, "success", "first turn, not a protocol response"),
			streamAssistantTextLine("second turn"),
			streamResultLine(false, "success", streamProtocolOK))
	}
	helperScenarios["stream-error-max-turns"] = func() {
		writeStreamLines(streamInitLine, streamAssistantTextLine("still going"),
			streamResultLineWithoutText("error_max_turns"))
	}
	helperScenarios["stream-is-error"] = func() {
		writeStreamLines(streamInitLine,
			streamResultLine(true, "success", "Invalid API key - please run /login"))
	}
	helperScenarios["stream-plain-text"] = func() {
		writeStreamLines(streamInitLine, streamAssistantTextLine("thinking"),
			streamResultLine(false, "success", "plain consultation reply, no protocol JSON"))
	}
	helperScenarios["stream-fenced"] = func() {
		writeStreamLines(streamInitLine,
			streamResultLine(false, "success", "```json\n"+streamProtocolOK+"\n```"))
	}
	helperScenarios["stream-protocol-in-assistant-only"] = func() {
		// The protocol text appears only in an assistant message; the result
		// line carries something else. The result line is authoritative.
		writeStreamLines(streamInitLine, streamAssistantTextLine(streamProtocolOK),
			streamResultLine(false, "success", "final words without protocol"))
	}
}
