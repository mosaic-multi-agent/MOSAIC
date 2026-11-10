package domain

import "time"

const (
	// DefaultTimeout bounds non-HITL dispatches, consultations and
	// infrastructure agents.
	DefaultTimeout = 30 * time.Minute
	// DefaultHITLTimeout bounds dispatches sent with human_in_the_loop: true.
	DefaultHITLTimeout = 4 * time.Hour
)

// InvocationTimeouts holds both invocation timeouts.
type InvocationTimeouts struct {
	NonHITL time.Duration
	HITL    time.Duration
}

// Normalized returns t with every value <= 0 replaced by its default.
func (t InvocationTimeouts) Normalized() InvocationTimeouts {
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
