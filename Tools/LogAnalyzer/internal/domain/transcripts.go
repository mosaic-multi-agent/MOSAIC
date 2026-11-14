package domain

import "context"

// TranscriptFormat is the sidecar native_format of a raw transcript copy.
type TranscriptFormat string

const (
	TranscriptClaudeCodeJSONL  TranscriptFormat = "claude-code-transcript-jsonl"
	TranscriptOpenCodeMessages TranscriptFormat = "opencode-sdk-session-messages-json"
)

// TranscriptFacts are the facts the transcript integrity checks need from one
// raw transcript, keyed so they match a stream by (Run, Owner, InstanceHint).
type TranscriptFacts struct {
	Run            RunRef           // always a named run
	Owner          StreamKind       // StreamOrchestrator or StreamAgentInstance
	InstanceHint   AgentInstanceID  // AgentEntry.InstanceHint (folder id); "" for the orchestrator
	Path           string           // the raw transcript file
	Format         TranscriptFormat // one of the known formats only
	AssistantIDs   []string         // distinct, first-seen order; claude-code only, nil otherwise
	FirstPrompt    string           // agents only; "" when HasFirstPrompt is false
	HasFirstPrompt bool
}

// TranscriptSource reads the harness transcripts copied into a log source.
// Implementations: transcript.Reader (production), in-memory fake (tests).
type TranscriptSource interface {
	// Transcripts returns facts for inv.Runs only (never the unattributable
	// bucket), and nil unless inv.Source.Kind is SourceLogsRoot or SourceSingleRun.
	//   - per RunEntry: <Dir>/00_orchestrator_session.raw, read when
	//     <Dir>/00_orchestrator_session.meta.json names a known format
	//   - per AgentEntry: <Dir>/04_session.raw, read when <Dir>/04_session.meta.json
	//     names a known format
	// An unknown native_format, a missing/unparseable sidecar or a missing/
	// unopenable raw file yields no entry for that transcript. Each raw file is
	// opened at most once per call. Never fails. A cancelled ctx returns the
	// entries gathered so far. Order: by run id, then the orchestrator entry, then
	// agents by InstanceHint.
	Transcripts(ctx context.Context, inv Inventory) []TranscriptFacts
}
