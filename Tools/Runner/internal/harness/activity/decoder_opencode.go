package activity

import (
	"encoding/json"

	"mosaic-run/internal/domain"
)

type openCodeDecoder struct{}

// NewOpenCodeDecoder decodes run --format json lines.
func NewOpenCodeDecoder() Decoder { return openCodeDecoder{} }

type openCodeLine struct {
	Type string `json:"type"`
	Part struct {
		Text   string   `json:"text"`
		Tool   string   `json:"tool"`
		Cost   *float64 `json:"cost"`
		Tokens struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		} `json:"tokens"`
		State struct {
			Status string          `json:"status"`
			Input  json.RawMessage `json:"input"`
		} `json:"state"`
	} `json:"part"`
}

func (openCodeDecoder) Decode(line []byte) []domain.ActivityItem {
	var l openCodeLine
	if json.Unmarshal(line, &l) != nil {
		return nil
	}
	p := l.Part
	switch l.Type {
	case "text":
		return textItems(p.Text)
	case "tool_use":
		if p.State.Status != "completed" {
			return nil
		}
		if p.Tool == "task" {
			return []domain.ActivityItem{subagentItem(p.State.Input)}
		}
		return []domain.ActivityItem{{Kind: domain.ActivityToolCall, Name: p.Tool, Summary: shortInput(p.State.Input)}}
	case "step_finish":
		if item, ok := usageItem(p.Tokens.Input, p.Tokens.Output, p.Cost); ok {
			return []domain.ActivityItem{item}
		}
	}
	return nil
}
