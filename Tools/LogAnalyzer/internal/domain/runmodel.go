package domain

import "time"

// InvocationState says whether an invocation_end was observed.
type InvocationState uint8

const (
	InvocationClosed InvocationState = iota // at least one invocation_end observed
	InvocationOpen                          // no invocation_end observed
)

// ToolCallPairing says how a tool call's start and end events relate.
type ToolCallPairing uint8

const (
	ToolCallPaired    ToolCallPairing = iota
	ToolCallStartOnly                 // start with no unambiguous end
	ToolCallEndOnly                   // end with no unambiguous start
)

// ToolCall is one tool call as logged. Payload bodies are never kept.
type ToolCall struct {
	CallID    string
	ToolName  string         // from tool_call_start; "" for ToolCallEndOnly or when not logged
	Status    ToolCallStatus // from tool_call_end; "" when no end or not logged
	StartedAt time.Time      // zero when no start or unparseable timestamp
	EndedAt   time.Time      // zero when no end or unparseable timestamp
	Pairing   ToolCallPairing
}

// Duration is EndedAt-StartedAt; ok only for Paired calls with both timestamps
// non-zero and EndedAt >= StartedAt.
func (c ToolCall) Duration() (time.Duration, bool) {
	if c.Pairing != ToolCallPaired {
		return 0, false
	}
	return timeSpan(c.StartedAt, c.EndedAt)
}

// timeSpan returns end-start when both are set and end is not before start.
func timeSpan(start, end time.Time) (time.Duration, bool) {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0, false
	}
	return end.Sub(start), true
}

// Invocation is one subagent instance's activity within a run.
type Invocation struct {
	Actor      ActorRef // ActorAgentInstance; Type as resolved for the matching ActorAggregate ("" when unknown)
	State      InvocationState
	StatusCode string    // from the last invocation_end, verbatim; "" when absent or Open
	Model      ModelID   // from the last invocation_end; "" when absent or Open
	StartedAt  time.Time // first invocation_start timestamp; zero when none or unparseable
	EndedAt    time.Time // last invocation_end timestamp; zero when Open or unparseable
	EndEvents  int       // number of invocation_end events observed
	ToolCalls  []ToolCall
	Usage      []InvocationUsage // identical to the matching ActorAggregate.Invocations; nil when none
}

// Duration is EndedAt-StartedAt; ok only when Closed, both timestamps are
// non-zero and EndedAt >= StartedAt.
func (i Invocation) Duration() (time.Duration, bool) {
	if i.State != InvocationClosed {
		return 0, false
	}
	return timeSpan(i.StartedAt, i.EndedAt)
}

// Compaction is one context compaction as logged.
type Compaction struct {
	Actor        ActorRef  // Orchestrator() or the agent instance of the stream
	At           time.Time // zero when unparseable
	Trigger      string
	TokensBefore TokenCount
	TokensAfter  TokenCount
}

// RunActivity is the per-run model built from tool-call, compaction and
// invocation events.
type RunActivity struct {
	OrchestratorToolCalls []ToolCall   // ordered by the file position of each call's first event
	Invocations           []Invocation // sorted by instance id
	Compactions           []Compaction // sorted by At (zero last), then actor label, then feed order
}
