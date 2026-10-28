package otelingest

import (
	"bytes"
	"encoding/json"
)

// usageMarker is the attribute name every span that can carry usage contains. A line without it is skipped
// before any JSON parsing.
var usageMarker = []byte(`"gen_ai.operation.name"`)

// Dialects: the GHCP CLI wraps every object as {"type":"span"|"metric",...}; VS Code writes raw SDK objects.
const (
	dialectCLI    = "cli"
	dialectVSCode = "vscode"
)

// Span kinds that matter for attribution besides the per-call chat span.
const (
	kindInvokeAgent = "invoke_agent"
	kindExecuteTool = "execute_tool"
)

// Tool spans that tie a subagent to the tool call that started it.
var spawnToolNames = map[string]bool{"task": true, "runSubagent": true, "execution_subagent": true}

// link is an invoke_agent or subagent-spawning execute_tool span kept in the state. Chat spans are placed through
// these links, which may arrive after the chat spans (children are exported before their parents).
type link struct {
	Kind                string `json:"kind"`
	ParentSpanID        string `json:"parent_span_id,omitempty"`
	ConversationID      string `json:"conversation_id,omitempty"`
	ChatSessionID       string `json:"chat_session_id,omitempty"`
	ParentChatSessionID string `json:"parent_chat_session_id,omitempty"`
	ToolCallID          string `json:"tool_call_id,omitempty"`
	ToolName            string `json:"tool_name,omitempty"`
}

// parsedLine is the effect of one OTel line: at most one of link and call is set.
type parsedLine struct {
	malformed bool
	linkID    string
	link      *link
	call      *call
}

// parseLine parses one complete line that contains the usage marker.
func parseLine(line []byte, offset int64) parsedLine {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return parsedLine{malformed: true}
	}
	if dec.More() {
		return parsedLine{malformed: true}
	}
	dialect := dialectVSCode
	if t, ok := obj["type"].(string); ok {
		if t != "span" {
			return parsedLine{}
		}
		dialect = dialectCLI
	}
	spanID := stringValue(obj["spanId"])
	attrs, _ := obj["attributes"].(map[string]any)
	if spanID == "" || attrs == nil {
		return parsedLine{}
	}
	parent := stringValue(obj["parentSpanId"])
	if ctx, ok := obj["parentSpanContext"].(map[string]any); ok && parent == "" {
		parent = stringValue(ctx["spanId"])
	}
	l := link{
		ParentSpanID:        parent,
		ConversationID:      stringValue(attrs["gen_ai.conversation.id"]),
		ChatSessionID:       stringValue(attrs["copilot_chat.chat_session_id"]),
		ParentChatSessionID: stringValue(attrs["copilot_chat.parent_chat_session_id"]),
	}
	switch stringValue(attrs["gen_ai.operation.name"]) {
	case "chat":
		if c := convertChat(attrs, dialect); c != nil {
			c.RecordID, c.RecordIndex, c.Dialect = spanID, offset, dialect
			c.ParentSpanID, c.ConversationID = l.ParentSpanID, l.ConversationID
			c.ChatSessionID, c.ParentChatSessionID = l.ChatSessionID, l.ParentChatSessionID
			return parsedLine{call: c}
		}
	case kindInvokeAgent:
		l.Kind = kindInvokeAgent
		return parsedLine{linkID: spanID, link: &l}
	case kindExecuteTool:
		name := stringValue(attrs["gen_ai.tool.name"])
		if spawnToolNames[name] {
			l.Kind, l.ToolName = kindExecuteTool, name
			l.ToolCallID = stringValue(attrs["gen_ai.tool.call.id"])
			return parsedLine{linkID: spanID, link: &l}
		}
	}
	return parsedLine{}
}

// stringValue returns v when it is a non-empty string, else "".
func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

// intValue returns v when it is a JSON number written as an integer. Strings and fractional numbers are not usage.
func intValue(v any) *int64 {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	i, err := n.Int64()
	if err != nil {
		return nil
	}
	return &i
}
