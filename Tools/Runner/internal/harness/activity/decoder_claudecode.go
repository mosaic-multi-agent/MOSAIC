package activity

import (
	"encoding/json"

	"mosaic-run/internal/domain"
)

type claudeCodeDecoder struct {
	usageSeen map[string]bool // message ids whose usage was already reported
	ctxMsgID  string          // message id whose context size is already in status
	status    domain.ActivityStatus
	tools     openTools
}

// NewClaudeCodeDecoder decodes --output-format stream-json lines.
func NewClaudeCodeDecoder() Decoder {
	return &claudeCodeDecoder{usageSeen: map[string]bool{}}
}

type claudeLine struct {
	Type         string   `json:"type"`
	Model        string   `json:"model"`
	ParentToolID *string  `json:"parent_tool_use_id"`
	TotalCostUSD *float64 `json:"total_cost_usd"`
	Message      struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
			CacheReadTokens  int `json:"cache_read_input_tokens"`
			CacheWriteTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
}

func (d *claudeCodeDecoder) Status() domain.ActivityStatus { return d.status }

func (d *claudeCodeDecoder) Decode(line []byte) []domain.ActivityItem {
	var l claudeLine
	if json.Unmarshal(line, &l) != nil {
		return nil
	}
	mainThread := l.ParentToolID == nil
	switch l.Type {
	case "system":
		if l.Model != "" {
			d.status.Model = l.Model
		}
	case "result":
		if l.TotalCostUSD != nil {
			d.status.CostUSD, d.status.HasCost = *l.TotalCostUSD, true
		}
	case "user":
		if mainThread {
			d.finishTools(l)
		}
	case "assistant":
		return d.assistant(l, mainThread)
	}
	return nil
}

func claudeBlocks(l claudeLine) []claudeBlock {
	var raws []json.RawMessage
	if json.Unmarshal(l.Message.Content, &raws) != nil {
		return nil
	}
	blocks := make([]claudeBlock, 0, len(raws))
	for _, raw := range raws {
		var b claudeBlock
		if json.Unmarshal(raw, &b) == nil {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// finishTools closes the main-thread tools whose results a user line carries.
func (d *claudeCodeDecoder) finishTools(l claudeLine) {
	for _, b := range claudeBlocks(l) {
		if b.Type == "tool_result" {
			d.tools.finish(b.ToolUseID)
		}
	}
	d.syncTool()
}

func (d *claudeCodeDecoder) syncTool() {
	d.status.RunningTool, d.status.RunningToolID = d.tools.newest()
}

func (d *claudeCodeDecoder) assistant(l claudeLine, mainThread bool) []domain.ActivityItem {
	var items []domain.ActivityItem
	for _, b := range claudeBlocks(l) {
		switch b.Type {
		case "text":
			items = append(items, textItems(b.Text)...)
		case "tool_use":
			if mainThread {
				d.tools.start(b.ID, b.Name)
			}
			if b.Name == "Task" || b.Name == "Agent" {
				items = append(items, subagentItem(b.Input))
			} else {
				items = append(items, domain.ActivityItem{Kind: domain.ActivityToolCall, Name: b.Name, Summary: shortInput(b.Input)})
			}
		}
	}
	if mainThread {
		d.syncTool()
		d.recordContext(l)
	}
	return append(items, d.usage(l)...)
}

// recordContext stores the context size of a main-thread call, once per message id.
func (d *claudeCodeDecoder) recordContext(l claudeLine) {
	u := l.Message.Usage
	if u == nil || (l.Message.ID != "" && l.Message.ID == d.ctxMsgID) {
		return
	}
	d.ctxMsgID = l.Message.ID
	d.status.ContextTokens = u.InputTokens + u.CacheReadTokens + u.CacheWriteTokens
	d.status.HasContext = true
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
