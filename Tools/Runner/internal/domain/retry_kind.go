package domain

// RetryKind marks a dispatch step the engine produced as a mechanical
// re-dispatch of the same assignment.
type RetryKind string

const (
	RetryNone            RetryKind = ""
	RetryPartiallyDone   RetryKind = "partially-done"
	RetryToolUnavailable RetryKind = "tool-unavailable" // BLOCKED E501, Tier 1
	// RetryScheduleDefect marks the Runner's own re-dispatch of the row that
	// produced a defective block schedule. Mechanical: the anti-loop guard
	// does not count it.
	RetryScheduleDefect RetryKind = "schedule-defect"
)
