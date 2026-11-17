package domain

import "time"

const (
	// DefaultTimeout bounds non-HITL dispatches, consultations and
	// infrastructure agents.
	DefaultTimeout = 30 * time.Minute
	// DefaultHITLTimeout bounds dispatches sent with human_in_the_loop: true.
	DefaultHITLTimeout = 4 * time.Hour
	// DefaultNoOutputTimeout is the opencode-v2 no-output watchdog duration
	// used when none is configured. It is deliberately well above V2's
	// legitimate silences: tool_use is emitted only when a tool completes (a
	// long shell command is silent), text is emitted per block, and after a
	// background_wait the root stream is silent until its background subagents
	// finish. Silent hangs seen in practice lasted 200-600 s. It never applies
	// to human-in-the-loop dispatches.
	DefaultNoOutputTimeout = 10 * time.Minute
	// NoOutputDisabled is the stored value of a disabled no-output watchdog.
	NoOutputDisabled time.Duration = -1
)

// InvocationTimeouts holds the invocation timeouts.
type InvocationTimeouts struct {
	NonHITL  time.Duration
	HITL     time.Duration
	NoOutput time.Duration // 0: default; > 0: value; < 0: disabled
}

// Normalized returns t with every timeout value <= 0 replaced by its default.
// NoOutput is the exception: 0 gets the default, a negative value (disabled)
// is kept.
func (t InvocationTimeouts) Normalized() InvocationTimeouts {
	if t.NoOutput == 0 {
		t.NoOutput = DefaultNoOutputTimeout
	}
	if t.NonHITL <= 0 {
		t.NonHITL = DefaultTimeout
	}
	if t.HITL <= 0 {
		t.HITL = DefaultHITLTimeout
	}
	return t
}

// For returns the normalised timeout for an invocation with the given HITL.
func (t InvocationTimeouts) For(hitl bool) time.Duration {
	n := t.Normalized()
	if hitl {
		return n.HITL
	}
	return n.NonHITL
}
