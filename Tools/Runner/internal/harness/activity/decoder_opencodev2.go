package activity

import (
	"encoding/json"
	"strings"

	"mosaic-run/internal/domain"
)

const (
	openCodeV2ToolSubagent = "subagent"
	openCodeV2ToolPatch    = "patch"
	openCodeV2PatchFileTag = "*** "
)

// openCodeV2Decoder reports the activity of an OpenCode V2 run. It tracks the
// context size of the latest step and never reports money cost: the stream
// carries a cost value that this harness deliberately ignores.
type openCodeV2Decoder struct {
	status domain.ActivityStatus
}

// NewOpenCodeV2Decoder decodes OpenCode V2 `run --format json` lines.
func NewOpenCodeV2Decoder() Decoder { return &openCodeV2Decoder{} }

type openCodeV2Line struct {
	Type string `json:"type"`
	Part struct {
		Text   string `json:"text"`
		Tool   string `json:"tool"`
		Tokens struct {
			Input     int `json:"input"`
			Output    int `json:"output"`
			Reasoning int `json:"reasoning"`
			Cache     struct {
				Read  int `json:"read"`
				Write int `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
		State struct {
			Status string          `json:"status"`
			Input  json.RawMessage `json:"input"`
			Error  string          `json:"error"`
		} `json:"state"`
	} `json:"part"`
}

func (d *openCodeV2Decoder) Status() domain.ActivityStatus { return d.status }

func (d *openCodeV2Decoder) Decode(line []byte) []domain.ActivityItem {
	var l openCodeV2Line
	if json.Unmarshal(line, &l) != nil {
		return nil
	}
	p := l.Part
	switch l.Type {
	case "text":
		return textItems(p.Text)
	case "tool_use":
		return openCodeV2ToolItems(p.Tool, p.State.Status, p.State.Input, p.State.Error)
	case "step_finish":
		t := p.Tokens
		d.status.ContextTokens, d.status.HasContext = t.Input+t.Cache.Read+t.Cache.Write, true
		if item, ok := usageItem(t.Input, t.Output+t.Reasoning, nil); ok {
			return []domain.ActivityItem{item}
		}
	}
	return nil
}

// openCodeV2ToolItems reports a finished tool call: a subagent launch, a tool
// error, or an ordinary call. A tool that has not finished yields nothing.
func openCodeV2ToolItems(tool, status string, input json.RawMessage, errText string) []domain.ActivityItem {
	switch status {
	case "error":
		return []domain.ActivityItem{{Kind: domain.ActivityToolCall, Name: tool, Summary: shorten("error: " + errText)}}
	case "completed":
	default:
		return nil
	}
	switch tool {
	case openCodeV2ToolSubagent:
		return []domain.ActivityItem{openCodeV2SubagentItem(input)}
	case openCodeV2ToolPatch:
		return []domain.ActivityItem{{Kind: domain.ActivityToolCall, Name: tool, Summary: openCodeV2PatchSummary(input)}}
	}
	return []domain.ActivityItem{{Kind: domain.ActivityToolCall, Name: tool, Summary: shortInput(input)}}
}

// openCodeV2SubagentItem builds a subagent launch item from the subagent tool
// input {agent, description, prompt[, sessionID]}.
func openCodeV2SubagentItem(raw json.RawMessage) domain.ActivityItem {
	var in struct {
		Agent       string `json:"agent"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
	}
	_ = json.Unmarshal(raw, &in)
	summary := in.Description
	if strings.TrimSpace(summary) == "" {
		summary = in.Prompt
	}
	return domain.ActivityItem{Kind: domain.ActivitySubagent, Name: in.Agent, Summary: shorten(summary)}
}

// openCodeV2PatchSummary names the files a patch touches instead of showing
// the patch body.
func openCodeV2PatchSummary(raw json.RawMessage) string {
	var in struct {
		PatchText string `json:"patchText"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return ""
	}
	var files []string
	for _, line := range strings.Split(in.PatchText, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, openCodeV2PatchFileTag) {
			continue
		}
		if _, name, ok := strings.Cut(strings.TrimPrefix(line, openCodeV2PatchFileTag), " File: "); ok {
			files = append(files, strings.TrimSpace(name))
		}
	}
	return shorten(strings.Join(files, ", "))
}
