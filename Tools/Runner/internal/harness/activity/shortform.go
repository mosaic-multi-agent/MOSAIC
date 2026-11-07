package activity

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mosaic-run/internal/domain"
)

// primaryInputKeys are the tool input fields that best describe a call, in
// order of preference.
var primaryInputKeys = []string{
	"command", "file_path", "filePath", "path", "pattern", "query", "url",
	"description", "prompt", "content",
}

// shorten renders s as a single line of at most domain.ActivitySummaryMaxRunes
// runes; truncation is marked by a trailing "...".
func shorten(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= domain.ActivitySummaryMaxRunes {
		return s
	}
	return string(runes[:domain.ActivitySummaryMaxRunes-3]) + "..."
}

// shortInput renders the input object of a tool call in short form: the most
// descriptive string field, or else the scalar fields as key=value pairs.
// Raw JSON is never returned.
func shortInput(raw json.RawMessage) string {
	var input map[string]any
	if json.Unmarshal(raw, &input) != nil {
		return ""
	}
	for _, key := range primaryInputKeys {
		if s, ok := input[key].(string); ok && strings.TrimSpace(s) != "" {
			return shorten(s)
		}
	}
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		switch v := input[k].(type) {
		case string, float64, bool:
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return shorten(strings.Join(parts, " "))
}

// textItems returns one text item per non-empty line of text.
func textItems(text string) []domain.ActivityItem {
	var items []domain.ActivityItem
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			items = append(items, domain.ActivityItem{Kind: domain.ActivityText, Summary: shorten(line)})
		}
	}
	return items
}

// subagentItem builds a subagent spawn item from a Task-style tool input.
func subagentItem(raw json.RawMessage) domain.ActivityItem {
	var in struct {
		SubagentType string `json:"subagent_type"`
		Description  string `json:"description"`
		Prompt       string `json:"prompt"`
	}
	_ = json.Unmarshal(raw, &in)
	summary := in.Description
	if strings.TrimSpace(summary) == "" {
		summary = in.Prompt
	}
	return domain.ActivityItem{Kind: domain.ActivitySubagent, Name: in.SubagentType, Summary: shorten(summary)}
}

func usageItem(in, out int, cost *float64) (domain.ActivityItem, bool) {
	if in == 0 && out == 0 && cost == nil {
		return domain.ActivityItem{}, false
	}
	u := domain.ActivityUsage{InputTokens: in, OutputTokens: out}
	if cost != nil {
		u.CostUSD, u.HasCost = *cost, true
	}
	return domain.ActivityItem{Kind: domain.ActivityKindUsage, Usage: u}, true
}
