package activity

import (
	"encoding/json"

	"mosaic-run/internal/domain"
)

type openCodeDecoder struct {
	status domain.ActivityStatus
}

// NewOpenCodeDecoder decodes run --format json lines.
func NewOpenCodeDecoder() Decoder { return &openCodeDecoder{} }

type openCodeLine struct {
	Type string `json:"type"`
	Part struct {
		Text   string   `json:"text"`
		Tool   string   `json:"tool"`
		Cost   *float64 `json:"cost"`
		Tokens struct {
			Input  int `json:"input"`
			Output int `json:"output"`
			Cache  struct {
				Read  int `json:"read"`
				Write int `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
		State struct {
			Status string          `json:"status"`
			Input  json.RawMessage `json:"input"`
		} `json:"state"`
	} `json:"part"`
}

func (d *openCodeDecoder) Status() domain.ActivityStatus { return d.status }

func (d *openCodeDecoder) Decode(line []byte) []domain.ActivityItem {
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
		t := p.Tokens
		d.status.ContextTokens, d.status.HasContext = t.Input+t.Cache.Read+t.Cache.Write, true
		if p.Cost != nil {
			d.status.CostUSD += *p.Cost
			d.status.HasCost = true
		}
		if item, ok := usageItem(t.Input, t.Output, p.Cost); ok {
			return []domain.ActivityItem{item}
		}
	}
	return nil
}
