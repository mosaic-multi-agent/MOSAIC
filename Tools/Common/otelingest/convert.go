package otelingest

import "encoding/json"

// providerName is the provider written on every ingested record; the OTel value "github" is not used.
const providerName = "github-copilot"

// tokenUsage holds the token sub-fields of a usage_record. Each is present only when its source is a genuine number.
type tokenUsage struct {
	Input         *int64 `json:"input_tokens,omitempty"`
	Output        *int64 `json:"output_tokens,omitempty"`
	CacheRead     *int64 `json:"cache_read_tokens,omitempty"`
	CacheCreation *int64 `json:"cache_creation_tokens,omitempty"`
	Reasoning     *int64 `json:"reasoning_tokens,omitempty"`
}

// call is one API call (a per-call chat span with usage) waiting for placement. It is also the pending-span
// shape of the state.
type call struct {
	RecordID            string      `json:"record_id"`
	RecordIndex         int64       `json:"record_index"`
	Dialect             string      `json:"dialect"`
	ParentSpanID        string      `json:"parent_span_id,omitempty"`
	ConversationID      string      `json:"conversation_id,omitempty"`
	ChatSessionID       string      `json:"chat_session_id,omitempty"`
	ParentChatSessionID string      `json:"parent_chat_session_id,omitempty"`
	Model               string      `json:"model,omitempty"`
	Effort              string      `json:"effort,omitempty"`
	TokenUsage          *tokenUsage `json:"token_usage"`
	Timestamp           string      `json:"-"`
	CallLabel           string      `json:"-"`
	CallKind            string      `json:"-"`
}

// convertChat maps the usage, model and effort attributes of a chat span; nil when the span carries no usage.
// Only the named attributes are read, so content attributes never reach a record.
func convertChat(attrs map[string]any, dialect string) *call {
	usage := convertTokens(attrs)
	if usage == nil {
		return nil
	}
	c := &call{TokenUsage: usage, Model: stringValue(attrs["gen_ai.response.model"])}
	if c.Model == "" {
		c.Model = stringValue(attrs["gen_ai.request.model"])
	}
	if dialect == dialectCLI {
		c.Effort = stringValue(attrs["gen_ai.request.reasoning.level"])
	} else {
		c.Effort = vscodeEffort(stringValue(attrs["copilot_chat.request.options"]))
		c.CallLabel = stringValue(attrs["gen_ai.agent.name"])
		c.CallKind = callKindOf(c.CallLabel)
	}
	return c
}

// convertTokens applies the token mapping. Copilot's input count includes the cache, so the record's input is the
// reported input minus cache read and cache write; a negative difference is not recorded.
func convertTokens(attrs map[string]any) *tokenUsage {
	u := &tokenUsage{
		Output:        intValue(attrs["gen_ai.usage.output_tokens"]),
		CacheRead:     intValue(attrs["gen_ai.usage.cache_read.input_tokens"]),
		CacheCreation: firstInt(attrs, "gen_ai.usage.cache_write.input_tokens", "gen_ai.usage.cache_creation.input_tokens"),
		Reasoning:     firstInt(attrs, "gen_ai.usage.reasoning.output_tokens", "gen_ai.usage.reasoning_tokens"),
	}
	reported := intValue(attrs["gen_ai.usage.input_tokens"])
	if reported != nil {
		diff := *reported - deref(u.CacheRead) - deref(u.CacheCreation)
		if diff >= 0 {
			u.Input = &diff
		}
	}
	if u.Input == nil && u.Output == nil && u.CacheRead == nil && u.CacheCreation == nil && u.Reasoning == nil {
		return nil
	}
	return u
}

func firstInt(attrs map[string]any, keys ...string) *int64 {
	for _, k := range keys {
		if v := intValue(attrs[k]); v != nil {
			return v
		}
	}
	return nil
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// vscodeEffort reads the effort verbatim from the request options JSON: output_config.effort before
// reasoning.effort. Unparsable options and non-string values give no effort.
func vscodeEffort(options string) string {
	if options == "" {
		return ""
	}
	var o struct {
		OutputConfig map[string]any `json:"output_config"`
		Reasoning    map[string]any `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(options), &o); err != nil {
		return ""
	}
	if e := stringValue(o.OutputConfig["effort"]); e != "" {
		return e
	}
	return stringValue(o.Reasoning["effort"])
}
