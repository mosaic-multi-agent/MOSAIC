package domain

import "time"

// TimeInterval is a closed span of logged time.
type TimeInterval struct{ Start, End time.Time }

// OpenWaitKind says why a wait has no usable end.
type OpenWaitKind uint8

const (
	OpenWaitToolCall   OpenWaitKind = iota // user-interaction tool call without a usable duration
	OpenWaitPermission                     // permission notification that cannot be paired
	OpenWaitIdlePrompt                     // idle_prompt notification (no end marker)
)

// OpenWait is a wait with no end; it is excluded from the waiting figure.
type OpenWait struct {
	Actor ActorRef
	At    time.Time
	Kind  OpenWaitKind
	Tool  string // OpenWaitToolCall only
}

// CountedTool is a tool name that counted as user-interaction for one harness.
type CountedTool struct {
	Tool    string
	Harness Harness
	Calls   int // matched calls, including open ones
}

// WallTimeSplit splits the run's logged span into waiting on the user,
// subagents working, orchestrator only and between sessions; each instant is
// counted once, in that order of precedence.
type WallTimeSplit struct {
	Span             TimeInterval // == Timeline.Axis
	HasSpan          bool
	Waiting          time.Duration
	Subagents        time.Duration
	OrchestratorOnly time.Duration
	BetweenSessions  time.Duration
	WaitIntervals    []TimeInterval // merged, sorted; for shading
	SessionGaps      []TimeInterval // sorted; for shading
	CountedTools     []CountedTool  // sorted by tool, then harness
	OpenWaits        []OpenWait     // sorted by At (zero last)

	MissingContactToolsHint bool
	UnconfiguredTools       []string // distinct, sorted; non-empty exactly when the hint is set
}

// PermissionPairingNote is shown with the waiting figure.
const PermissionPairingNote = "permission notifications cannot be paired with their resolution in current logs; they are shown as open-ended"
