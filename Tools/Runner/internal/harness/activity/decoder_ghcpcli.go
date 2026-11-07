package activity

import (
	"encoding/json"
	"strings"

	"mosaic-run/internal/domain"
)

type ghcpCLIDecoder struct{}

// NewGHCPCLIDecoder decodes --output-format json lines.
func NewGHCPCLIDecoder() Decoder { return ghcpCLIDecoder{} }

type ghcpCLILine struct {
	Type string `json:"type"`
	Data struct {
		Content          string          `json:"content"`
		ToolName         string          `json:"toolName"`
		Arguments        json.RawMessage `json:"arguments"`
		AgentName        string          `json:"agentName"`
		AgentDisplayName string          `json:"agentDisplayName"`
		AgentDescription string          `json:"agentDescription"`
		InputTokens      int             `json:"inputTokens"`
		OutputTokens     int             `json:"outputTokens"`
	} `json:"data"`
}

func (ghcpCLIDecoder) Decode(line []byte) []domain.ActivityItem {
	var l ghcpCLILine
	if json.Unmarshal(line, &l) != nil {
		return nil
	}
	d := l.Data
	switch l.Type {
	case "assistant.message":
		return textItems(d.Content)
	case "tool.execution_start":
		return []domain.ActivityItem{{Kind: domain.ActivityToolCall, Name: d.ToolName, Summary: shortInput(d.Arguments)}}
	case "subagent.started":
		name := d.AgentName
		if name == "" {
			name = d.AgentDisplayName
		}
		summary := d.AgentDescription
		if strings.TrimSpace(summary) == "" {
			summary = d.AgentDisplayName
		}
		return []domain.ActivityItem{{Kind: domain.ActivitySubagent, Name: name, Summary: shorten(summary)}}
	case "assistant.usage":
		if item, ok := usageItem(d.InputTokens, d.OutputTokens, nil); ok {
			return []domain.ActivityItem{item}
		}
	}
	return nil
}
