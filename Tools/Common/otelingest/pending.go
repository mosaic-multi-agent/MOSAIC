package otelingest

import "encoding/json"

// pendingRecord is the placement-independent part of a pending span as stored in the state.
type pendingRecord struct {
	RecordID    string      `json:"record_id"`
	RecordIndex int64       `json:"record_index"`
	Model       string      `json:"model,omitempty"`
	Provider    string      `json:"provider"`
	Effort      string      `json:"effort,omitempty"`
	TokenUsage  *tokenUsage `json:"token_usage"`
}

// pendingSpan is the stored shape of a call waiting for the span it needs for attribution.
type pendingSpan struct {
	Record              pendingRecord `json:"record"`
	ParentSpanID        string        `json:"parent_span_id,omitempty"`
	ConversationID      string        `json:"conversation_id,omitempty"`
	ChatSessionID       string        `json:"chat_session_id,omitempty"`
	ParentChatSessionID string        `json:"parent_chat_session_id,omitempty"`
}

// MarshalJSON stores a call in the pending-span shape.
func (c *call) MarshalJSON() ([]byte, error) {
	return json.Marshal(pendingSpan{
		Record: pendingRecord{
			RecordID: c.RecordID, RecordIndex: c.RecordIndex, Model: c.Model,
			Provider: providerName, Effort: c.Effort, TokenUsage: c.TokenUsage,
		},
		ParentSpanID:        c.ParentSpanID,
		ConversationID:      c.ConversationID,
		ChatSessionID:       c.ChatSessionID,
		ParentChatSessionID: c.ParentChatSessionID,
	})
}

// UnmarshalJSON reads a call from the pending-span shape. The dialect is not stored: only VS Code spans carry a
// chat session id, and a pending call is placed through its parent span chain only in the GHCP CLI dialect.
func (c *call) UnmarshalJSON(data []byte) error {
	var p pendingSpan
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*c = call{
		RecordID: p.Record.RecordID, RecordIndex: p.Record.RecordIndex, Model: p.Record.Model,
		Effort: p.Record.Effort, TokenUsage: p.Record.TokenUsage,
		ParentSpanID: p.ParentSpanID, ConversationID: p.ConversationID,
		ChatSessionID: p.ChatSessionID, ParentChatSessionID: p.ParentChatSessionID,
		Dialect: dialectCLI,
	}
	if p.ChatSessionID != "" {
		c.Dialect = dialectVSCode
	}
	return nil
}

// inScope reports whether a final ingest with the given scope may finalise the call: an empty scope covers every
// conversation, otherwise the call's conversation or parent chat session must be listed.
func inScope(scope []string, c *call) bool {
	if len(scope) == 0 {
		return true
	}
	for _, s := range scope {
		if s != "" && (s == c.ConversationID || s == c.ParentChatSessionID) {
			return true
		}
	}
	return false
}
