package activity

import (
	"encoding/json"
	"strings"

	"mosaic-run/internal/domain"
)

// codexDecoder decodes `codex exec --json` lines. Codex reports neither model,
// context size nor cost, so the status only tracks the running tool.
type codexDecoder struct {
	reported map[string]bool // tool item ids already shown in the feed
	tools    openTools
	status   domain.ActivityStatus
}

// NewCodexDecoder decodes `codex exec --json` lines.
func NewCodexDecoder() Decoder { return &codexDecoder{reported: map[string]bool{}} }

type codexLine struct {
	Type  string `json:"type"`
	Item  codexItem
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type codexItem struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Command   string          `json:"command"`
	Server    string          `json:"server"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Query     string          `json:"query"`
	Prompt    string          `json:"prompt"`
	Changes   []struct {
		Path string `json:"path"`
	} `json:"changes"`
}

func (l *codexLine) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type  string          `json:"type"`
		Item  json.RawMessage `json:"item"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	l.Type, l.Usage = raw.Type, raw.Usage
	if len(raw.Item) > 0 {
		// A malformed item leaves the zero item, which decodes to nothing.
		_ = json.Unmarshal(raw.Item, &l.Item)
	}
	return nil
}

func (d *codexDecoder) Status() domain.ActivityStatus { return d.status }

func (d *codexDecoder) Decode(line []byte) []domain.ActivityItem {
	var l codexLine
	if json.Unmarshal(line, &l) != nil {
		return nil
	}
	switch l.Type {
	case "item.started":
		return d.toolItem(l.Item, true)
	case "item.completed":
		if l.Item.Type == "agent_message" {
			return textItems(l.Item.Text)
		}
		return d.toolItem(l.Item, false)
	case "turn.completed":
		if l.Usage != nil {
			if item, ok := usageItem(l.Usage.InputTokens, l.Usage.OutputTokens, nil); ok {
				return []domain.ActivityItem{item}
			}
		}
	}
	return nil
}

// toolItem reports a tool item once per id and keeps the running tool in step:
// a start opens it, a completion closes it.
func (d *codexDecoder) toolItem(it codexItem, started bool) []domain.ActivityItem {
	name, summary, ok := codexTool(it)
	if !ok {
		return nil
	}
	var items []domain.ActivityItem
	if !d.reported[it.ID] {
		d.reported[it.ID] = true
		items = append(items, domain.ActivityItem{Kind: domain.ActivityToolCall, Name: name, Summary: shorten(summary)})
	}
	if started {
		d.tools.start(it.ID, name)
	} else {
		d.tools.finish(it.ID)
	}
	d.status.RunningTool, d.status.RunningToolID = d.tools.newest()
	return items
}

// codexTool names a tool item and describes it; ok is false for item types
// that are not tool calls.
func codexTool(it codexItem) (name, summary string, ok bool) {
	switch it.Type {
	case "command_execution":
		return "command", it.Command, true
	case "mcp_tool_call":
		summary = shortInput(it.Arguments)
		return it.Server + "." + it.Tool, summary, true
	case "web_search":
		return "web_search", it.Query, true
	case "file_change":
		paths := make([]string, 0, len(it.Changes))
		for _, c := range it.Changes {
			paths = append(paths, c.Path)
		}
		return "file_change", strings.Join(paths, ", "), true
	case "collab_tool_call":
		return it.Tool, it.Prompt, true
	}
	return "", "", false
}
