package diag

// Kind names one class of ingest diagnostic.
type Kind string

const (
	KindVariableUnset       Kind = "variable_unset" // emitted by callers, never by Ingest
	KindFileMissing         Kind = "file_missing"
	KindLockHeld            Kind = "lock_held"
	KindUnknownStateVersion Kind = "unknown_state_version" // IDs: [raw version text]
	KindLinesSkipped        Kind = "lines_skipped"         // Counts: malformed, read; only when malformed > 0
	KindConversationNotTied Kind = "conversation_not_tied" // Counts: usage_spans; IDs: run session IDs
	KindPendingFinalized    Kind = "pending_finalized"     // Counts: records; final ingests only, when > 0
	KindBudgetExhausted     Kind = "budget_exhausted"      // Python only; Counts: remaining_bytes
	KindAppendTimedOut      Kind = "append_timed_out"      // Counts: lines (lines not written because the append lock timed out)
	KindInternalError       Kind = "internal_error"
)

// Diagnostic is one reported condition of an ingest.
type Diagnostic struct {
	Kind    Kind
	Message string         // human-readable; not compared by fixtures
	Counts  map[string]int // nil when none
	IDs     []string       // sorted; nil when none
}
