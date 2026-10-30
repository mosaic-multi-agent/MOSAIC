package domain

import "time"

// EventType is the wire discriminator. Unrecognised values decode as EventOther
// with Raw preserved, never as an error.
type EventType string

const (
	EventRunStart        EventType = "run_start"
	EventRunEnd          EventType = "run_end"
	EventSessionStart    EventType = "session_start"
	EventSessionEnd      EventType = "session_end"
	EventInvocationStart EventType = "invocation_start"
	EventInvocationEnd   EventType = "invocation_end"
	EventTurn            EventType = "turn"
	EventUsageRecord     EventType = "usage_record"
	EventToolCallStart   EventType = "tool_call_start"
	EventToolCallEnd     EventType = "tool_call_end"
	EventCompaction      EventType = "compaction"
	EventNotification    EventType = "notification"
	EventOther           EventType = "" // any type this tool does not consume
)

// SchemaVersionCurrent is the version this tool is written against.
const SchemaVersionCurrent = "1.5.0"

// SupportedSchemaMajor is the only major version read without a finding.
const SupportedSchemaMajor = 1

// Event is one decoded log line. Only the envelope and the field blocks this tool
// consumes are modelled. Large payload bodies (prompt, response, content,
// tool_input, tool_output) are deliberately NOT decoded.
type Event struct {
	// Envelope
	SchemaVersion string
	Type          EventType
	RawType       string // the wire "event" value, retained for EventOther
	Timestamp     time.Time
	Harness       Harness
	SessionID     string // "" when absent
	RunID         string // "" when absent

	// Field blocks; exactly one is non-nil for a recognised type, all nil for EventOther.
	InvocationStart *InvocationStartFields
	InvocationEnd   *InvocationEndFields
	Turn            *TurnFields
	UsageRecord     *UsageRecordFields
	ToolCallStart   *ToolCallStartFields
	ToolCallEnd     *ToolCallEndFields
	Compaction      *CompactionFields
	RunStart        *RunStartFields
	RunEnd          *RunEndFields
	SessionStart    *SessionStartFields
	SessionEnd      *SessionEndFields
	Notification    *NotificationFields
}

// ToolCallStatus is the outcome reported on a tool_call_end event.
type ToolCallStatus string

const (
	ToolCallSuccess ToolCallStatus = "success"
	ToolCallError   ToolCallStatus = "error"
	// "" = status absent; any other value is kept verbatim.
)

// ToolCallStartFields holds the fields from a tool_call_start event. The
// tool_input payload is deliberately not decoded.
type ToolCallStartFields struct {
	CallID   string // "" when absent
	ToolName string // "" when absent
}

// ToolCallEndFields holds the fields from a tool_call_end event. The
// tool_output and error payloads are deliberately not decoded.
type ToolCallEndFields struct {
	CallID   string
	Status   ToolCallStatus
	ToolName string // "" when absent (schema 1.6.0)
}

// CallKind says what a usage_record call was for (schema 1.6.0).
type CallKind string

const (
	CallMain       CallKind = "main"
	CallCompaction CallKind = "compaction"
	CallAuxiliary  CallKind = "auxiliary"
)

// CompactionPhase is the phase of a compaction event (schema 1.5.0).
type CompactionPhase string

const (
	CompactionPhaseNone CompactionPhase = ""     // absent, or any value other than pre/post
	CompactionPhasePre  CompactionPhase = "pre"
	CompactionPhasePost CompactionPhase = "post"
)

// CompactionFields holds the fields from a compaction event.
type CompactionFields struct {
	Trigger      string // "auto" | "manual" | other verbatim | "" absent
	TokensBefore TokenCount
	TokensAfter  TokenCount
	Phase        CompactionPhase // decoded from "phase"; unknown values decode as CompactionPhaseNone
}

// InvocationStartFields holds the fields from an invocation_start event.
type InvocationStartFields struct {
	AgentInstanceID AgentInstanceID
	AgentType       string // "" when absent
}

// InvocationEndFields holds the fields from an invocation_end event.
type InvocationEndFields struct {
	AgentInstanceID AgentInstanceID
	StatusCode      string // "" when absent
	Model           ModelID
	Provider        ProviderID // "" when absent
	Usage           TokenUsage // all-absent when token_usage was absent
	HasUsage        bool       // false when the token_usage key was absent entirely
}

// UsageRecordFields holds the fields from a usage_record event: the usage of
// exactly one assistant record (one API call) observed in a transcript.
//
// The adapter deliberately does not deduplicate. The same record is emitted
// once per hook firing that observed it, so RecordID is the key by which the
// aggregator counts it exactly once.
type UsageRecordFields struct {
	AgentInstanceID AgentInstanceID // "" on the orchestrator stream
	RecordID        string          // "" only when the adapter could not derive one
	RecordIndex     int64
	HasRecordIndex  bool
	Source          string // "orchestrator_transcript" | "agent_transcript"; forensic only
	Model           ModelID
	ServiceTier     string     // "" when absent; descriptive only, never priced
	Provider        ProviderID // "" when absent
	Effort          string     // verbatim; "" when absent
	Usage           TokenUsage
	HasUsage        bool     // false when the token_usage key was absent entirely
	CallKind        CallKind // CallMain when absent or unknown
	CallLabel       string   // verbatim; "" when absent
}

// TurnRole is the role (user or assistant) of a turn event.
type TurnRole string

const (
	TurnUser      TurnRole = "user"
	TurnAssistant TurnRole = "assistant"
)

// TurnFields holds the fields from a turn event.
type TurnFields struct {
	Role     TurnRole
	Model    ModelID
	Provider ProviderID // "" when absent
	Usage    TokenUsage
	HasUsage bool
}

// RunStartFields holds the fields from a run_start event.
type RunStartFields struct {
	AdapterVersion string     // "" when absent
	Model          ModelID    // orchestrator model; "" when absent
	Provider       ProviderID // "" when absent
}

// RunEndFields holds the fields from a run_end event.
type RunEndFields struct {
	Outcome string // "" when absent
}

// SessionStartFields holds the fields from a session_start event.
type SessionStartFields struct {
	Resumed  OptionalBool
	Model    ModelID
	Provider ProviderID
}

// SessionEndFields holds the fields from a session_end event.
type SessionEndFields struct {
	Reason string // "" when absent
}

// NotificationFields holds the fields from a notification event. The message
// is deliberately not decoded.
type NotificationFields struct {
	NotificationType string // "" when absent
	RequiresResponse OptionalBool
	Resolution       string // "" when absent
	NotificationID   string // "" when absent
}
