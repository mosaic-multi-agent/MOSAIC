package domain

import (
	"context"
	"time"
)

// TranscriptAlarm is one adapter report that a run's harness transcript had
// assistant turns but no readable usage (format change).
type TranscriptAlarm struct {
	Run       RunRef
	Harness   Harness
	SessionID string    // "" when the line carried none
	At        time.Time // line timestamp; zero when unparseable
	Path      string    // diagnostics file path
	Line      int       // 1-based line number
}

// AlarmSource reads adapter alarms from the diagnostics file of a source's logs root.
type AlarmSource interface {
	// TranscriptAlarms returns the alarms of the source, in file order. A missing or
	// unreadable diagnostics file yields nil, and so does every source kind other than
	// SourceLogsRoot and SourceSingleRun (digest sources included). Never fails.
	TranscriptAlarms(ctx context.Context, src Source) []TranscriptAlarm
}

// FindingTranscriptFormatChanged reports a run whose harness transcript had
// assistant turns but no readable usage. A data-loss kind.
const FindingTranscriptFormatChanged FindingKind = "transcript_format_changed"
