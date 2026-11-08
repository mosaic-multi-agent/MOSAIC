package activity

import (
	"encoding/json"
	"strings"

	"mosaic-run/internal/domain"
)

type ghcpCLIDecoder struct {
	status domain.ActivityStatus
	tools  openTools
}

// NewGHCPCLIDecoder decodes --output-format json lines.
func NewGHCPCLIDecoder() Decoder { return &ghcpCLIDecoder{} }

type ghcpCLILine struct {
	Type string `json:"type"`
	Data struct {
		Content          string          `json:"content"`
		ToolName         string          `json:"toolName"`
		ToolCallID       string          `json:"toolCallId"`
		ParentToolCallID string          `json:"parentToolCallId"`
		Model            string          `json:"model"`
		Arguments        json.RawMessage `json:"arguments"`
		AgentName        string          `json:"agentName"`
		AgentDisplayName string          `json:"agentDisplayName"`
		AgentDescription string          `json:"agentDescription"`
		InputTokens      int             `json:"inputTokens"`
		OutputTokens     int             `json:"outputTokens"`
		CacheReadTokens  int             `json:"cacheReadTokens"`
		CacheWriteTokens int             `json:"cacheWriteTokens"`
	} `json:"data"`
}

func (d *ghcpCLIDecoder) Status() domain.ActivityStatus { return d.status }

func (d *ghcpCLIDecoder) Decode(line []byte) []domain.ActivityItem {
	var l ghcpCLILine
	if json.Unmarshal(line, &l) != nil {
		return nil
	}
	data := l.Data
	switch l.Type {
	case "assistant.message":
		return textItems(data.Content)
	case "tool.execution_start":
		if data.ParentToolCallID == "" {
			d.tools.start(data.ToolCallID, data.ToolName)
			d.status.RunningTool, d.status.RunningToolID = d.tools.newest()
		}
		return []domain.ActivityItem{{Kind: domain.ActivityToolCall, Name: data.ToolName, Summary: shortInput(data.Arguments)}}
	case "tool.execution_complete":
		d.tools.finish(data.ToolCallID)
		d.status.RunningTool, d.status.RunningToolID = d.tools.newest()
	case "subagent.started":
		name := data.AgentName
		if name == "" {
			name = data.AgentDisplayName
		}
		summary := data.AgentDescription
		if strings.TrimSpace(summary) == "" {
			summary = data.AgentDisplayName
		}
		return []domain.ActivityItem{{Kind: domain.ActivitySubagent, Name: name, Summary: shorten(summary)}}
	case "assistant.usage":
		if data.ParentToolCallID == "" {
			d.recordMainCall(data.Model, data.InputTokens+data.CacheReadTokens+data.CacheWriteTokens)
		}
		if item, ok := usageItem(data.InputTokens, data.OutputTokens, nil); ok {
			return []domain.ActivityItem{item}
		}
	}
	return nil
}

func (d *ghcpCLIDecoder) recordMainCall(model string, context int) {
	if model != "" {
		d.status.Model = model
	}
	d.status.ContextTokens, d.status.HasContext = context, true
}
