package otelingest

import (
	"time"

	"mosaic-common/otelingest/diag"
)

// EnvVar is the only configuration source for the OTel file path.
const EnvVar = "COPILOT_OTEL_FILE_EXPORTER_PATH"

// StateVersion is the .otel-ingest state version this package reads and writes.
const StateVersion = 1

// LockStaleAfter is the age after which a lock left by a crashed ingest is broken.
const LockStaleAfter = 10 * time.Minute

// Request describes one ingest of one OTel file into one run's MOSAIC log folder.
type Request struct {
	RunDir      string           // absolute path of OrchestrationLogs/{run_id}; its base name is the run_id
	OTelPath    string           // path of the OTel JSONL file (never modified)
	Final       bool             // final ingest: pending spans in Scope go to the unattributed stream
	WholeFile   bool             // recovery: read from byte 0 regardless of the stored cursor
	StartOffset *int64           // first cursor when the state has no cursor yet; nil = 0
	Scope       []string         // final ingests only: conversation IDs whose pending spans may be finalised
	Now         func() time.Time // envelope timestamp and lock time source; nil = time.Now
}

// Status is the outcome class of one ingest.
type Status string

const (
	StatusIngested     Status = "ingested"
	StatusLockHeld     Status = "lock_held"
	StatusFileMissing  Status = "file_missing"
	StatusUnknownState Status = "unknown_state"
	StatusFailed       Status = "failed"
)

// Result reports one ingest. Counts refer to this ingest only.
type Result struct {
	Status         Status
	RecordsWritten int
	Finalized      int
	Pending        int
	LinesRead      int
	LinesMalformed int
	// ToolEndsWritten counts tool_call_end lines this ingest wrote for failed tool calls.
	ToolEndsWritten int
	Diagnostics     []diag.Diagnostic // sorted by Kind
	Err             error             // non-nil only with StatusFailed
}

// PendingStatus is the read-only view of a run's ingest state.
type PendingStatus struct {
	StatePresent bool
	StateKnown   bool
	Pending      int
}
