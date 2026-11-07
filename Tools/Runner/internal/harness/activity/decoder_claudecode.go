package activity

import (
	"encoding/json"

	"mosaic-run/internal/domain"
)

type claudeCodeDecoder struct {
	usageSeen map[string]bool // message ids whose usage was already reported
}

// NewClaudeCodeDecoder decodes --output-format stream-json lines.
func NewClaudeCodeDecoder() Decoder {
	return &claudeCodeDecoder{usageSeen: map[string]bool{}}
}

type claudeLine struct {
	Type    string `json:"type"`
	Message struct {
		ID      string            `json:"id"`
		Content []json.RawMessage `json:"content"`
		Usage   *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type claudeBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

func (d *claudeCodeDecoder) Decode(line []byte) []domain.ActivityItem {
	var l claudeLine
	if json.Unmarshal(line, &l) != nil || l.Type != "assistant" {
		return nil
	}
	var items []domain.ActivityItem
	for _, raw := range l.Message.Content {
		var b claudeBlock
		if json.Unmarshal(raw, &b) != nil {
			continue
		}
		switch b.Type {
		case "text":
			items = append(items, textItems(b.Text)...)
		case "tool_use":
			if b.Name == "Task" || b.Name == "Agent" {
				items = append(items, subagentItem(b.Input))
			} else {
				items = append(items, domain.ActivityItem{Kind: domain.ActivityToolCall, Name: b.Name, Summary: shortInput(b.Input)})
			}
		}
	}
	return append(items, d.usage(l)...)
}

// usage reports a message's usage once, however many lines carry it.
func (d *claudeCodeDecoder) usage(l claudeLine) []domain.ActivityItem {
	u := l.Message.Usage
	if u == nil || (l.Message.ID != "" && d.usageSeen[l.Message.ID]) {
		return nil
	}
	item, ok := usageItem(u.InputTokens, u.OutputTokens, nil)
	if !ok {
		return nil
	}
	if l.Message.ID != "" {
		d.usageSeen[l.Message.ID] = true
	}
	return []domain.ActivityItem{item}
}
