package otelingest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"mosaic-common/logappend"
)

const (
	// recordSchemaVersion is the highest MOSAIC log schema version whose fields this package implements.
	recordSchemaVersion = "1.7.0"
	unattributedStream  = "00_unattributed_events.jsonl"
	// timestampLayout is the envelope timestamp format: UTC, milliseconds.
	timestampLayout = "2006-01-02T15:04:05.000Z"
)

// usageRecord is the usage_record event written for one call.
type usageRecord struct {
	SchemaVersion   string      `json:"schema_version"`
	Event           string      `json:"event"`
	Timestamp       string      `json:"timestamp"`
	Harness         string      `json:"harness"`
	SessionID       string      `json:"session_id,omitempty"`
	RunID           string      `json:"run_id"`
	RecordID        string      `json:"record_id"`
	RecordIndex     int64       `json:"record_index"`
	Source          string      `json:"source"`
	AgentInstanceID string      `json:"agent_instance_id,omitempty"`
	Model           string      `json:"model,omitempty"`
	Provider        string      `json:"provider"`
	Effort          string      `json:"effort,omitempty"`
	TokenUsage      *tokenUsage `json:"token_usage,omitempty"`
	CallLabel       string      `json:"call_label,omitempty"`
	CallKind        string      `json:"call_kind,omitempty"`
	RequestedTools  *toolCounts `json:"requested_tools,omitempty"`
}

// writer appends records to the streams of one run.
type writer struct {
	runDir string
	runID  string
	now    time.Time
	keys   *runKeys
}

// write appends the call as one whole line to the stream of dest.
func (w *writer) write(c *call, dest string) error {
	session := w.keys.conversation(c)
	rec := usageRecord{
		SchemaVersion:  recordSchemaVersion,
		Event:          "usage_record",
		Timestamp:      c.Timestamp,
		Harness:        w.harness(c, session),
		SessionID:      session,
		RunID:          w.runID,
		RecordID:       c.RecordID,
		RecordIndex:    c.RecordIndex,
		Source:         "otel",
		Model:          c.Model,
		Provider:       providerName,
		Effort:         c.Effort,
		TokenUsage:     c.TokenUsage,
		CallLabel:      c.CallLabel,
		CallKind:       c.CallKind,
		RequestedTools: c.RequestedTools,
	}
	path := filepath.Join(w.runDir, unattributedStream)
	switch {
	case dest == destOrchestrator:
		path = filepath.Join(w.runDir, orchestratorStream)
	case strings.HasPrefix(dest, destInvocation):
		rec.AgentInstanceID = strings.TrimPrefix(dest, destInvocation)
		path = filepath.Join(w.runDir, rec.AgentInstanceID, invocationStream)
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return appendLine(path, append(line, '\n'))
}

// harness is the envelope harness of the session the call is tied through, else of the invocation whose
// spawn_call_id is the call's chat session.
func (w *writer) harness(c *call, session string) string {
	if h := w.keys.harnessBySession[session]; h != "" {
		return h
	}
	if ids := w.keys.invBySpawn[c.ChatSessionID]; len(ids) > 0 {
		if h := w.keys.harnessByInv[ids[0]]; h != "" {
			return h
		}
	}
	return w.keys.firstHarness
}

// appendLine appends one whole line under the append lock of the log format. A lock timeout
// is returned as logappend.ErrLockTimeout, with nothing written.
func appendLine(path string, line []byte) error {
	return logappend.Append(path, line)
}
