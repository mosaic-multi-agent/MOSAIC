package domain

// Runner-log event names added after the original vocabulary in ports.go. This
// file is the one place new event names are added.
const (
	// An infrastructure dispatch (trigger dispatch or commit setup) ended with a
	// status other than SUCCESS.
	// Message: the reason (response ErrorReason; StatusMessage when ErrorReason is
	// empty; the harness error text for a harness-level error).
	// Fields: agent (instance id), class, triggers (matched triggers, comma-separated,
	// declaration order; "" for commit setup), status, error_code ("" when none),
	// policy ("halt" | "continue"; always "halt" for commit setup).
	EventSessionInfraFailed = "session.infra.failed"
)

const (
	// The TUI changed the session loop-guard waiver.
	// Fields: waived ("true" | "false"), trigger ("key" | "resume-guard-off" | "start").
	EventTUIGuardSwitched = "tui.guard.switched"

	// The session observed a changed waiver value at a guard check.
	// Fields: waived ("true" | "false"), count_reset ("true" when re-enabled).
	EventSessionGuardApplied = "session.guard.applied"
)

const (
	// Live subagent stdout lines of one invocation.
	// Message: one or more consecutive lines joined by "\n".
	// Fields: agent, first (1-based ordinal of the first line), lines (count).
	EventHarnessStdoutLines = "harness.stdout.lines"

	// The harness process of one invocation ended.
	// Message: "process ended: <reason>".
	// Fields: agent, reason (exit|nonzero_exit|timeout|no_output|cancelled),
	// stdout_bytes, exit_code (exit and nonzero_exit only).
	EventHarnessProcessEnd = "harness.process.end"
)

const (
	// The run's standing guidance was set or changed.
	// Message: the full guidance in the guidance file format (both fields).
	// Fields: change (a GuidanceChange value).
	EventSessionGuidanceChanged = "session.guidance.changed"
)
