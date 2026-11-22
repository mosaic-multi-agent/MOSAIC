package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type claudeAssistant struct {
	Type    string `json:"type"`
	UUID    string `json:"uuid"`
	Message struct {
		ID string `json:"id"`
	} `json:"message"`
}

type claudeWithContent struct {
	Type    string `json:"type"`
	UUID    string `json:"uuid"`
	Message struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// readClaudeJSONL streams a Claude Code transcript once and returns the
// distinct assistant ids in first-seen order and, when wantPrompt is set, the
// first user record's prompt. Lines of any length are handled; lines that do
// not parse are skipped.
func readClaudeJSONL(r io.Reader, wantPrompt bool) (ids []string, prompt string, hasPrompt bool) {
	br := bufio.NewReaderSize(r, 1<<20)
	seen := map[string]bool{}
	promptPending := wantPrompt

	addID := func(typ, uuid, msgID string) {
		if typ != "assistant" {
			return
		}
		id := msgID
		if id == "" {
			id = uuid
		}
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if promptPending {
				var rec claudeWithContent
				if json.Unmarshal(line, &rec) == nil {
					if rec.Type == "user" {
						promptPending = false
						prompt, hasPrompt = claudePromptText(rec.Message.Content)
					}
					addID(rec.Type, rec.UUID, rec.Message.ID)
				}
			} else {
				var rec claudeAssistant
				if json.Unmarshal(line, &rec) == nil {
					addID(rec.Type, rec.UUID, rec.Message.ID)
				}
			}
		}
		if err != nil {
			return ids, prompt, hasPrompt
		}
	}
}

// claudePromptText extracts a prompt from a string content or a list of text blocks.
func claudePromptText(content json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s, true
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return "", false
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	if len(texts) == 0 {
		return "", false
	}
	return strings.Join(texts, "\n"), true
}
