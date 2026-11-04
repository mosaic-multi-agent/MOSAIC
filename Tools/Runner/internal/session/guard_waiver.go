package session

import (
	"strconv"
	"sync/atomic"

	"mosaic-run/internal/domain"
)

// GuardWaiver is the session-only anti-loop guard waiver, shared between the TUI
// (writer) and the session (reader). Safe for concurrent use. The zero value is
// usable and means the guard is on; NewGuardWaiver is the conventional
// constructor. Never persisted.
type GuardWaiver struct{ waived atomic.Bool }

// NewGuardWaiver returns a waiver with the guard on (not waived).
func NewGuardWaiver() *GuardWaiver { return &GuardWaiver{} }

// Set sets the waiver. It returns true when the value changed.
func (w *GuardWaiver) Set(waived bool) bool { return w.waived.Swap(waived) != waived }

// Waived reports whether the guard is currently waived. Suitable as
// Deps.AntiLoopWaived via method value.
func (w *GuardWaiver) Waived() bool { return w.waived.Load() }

// observeWaiver reads the waiver at a guard check and reports whether the guard
// is waived. A change against the value seen at the previous check is logged;
// re-enabling the guard also resets its count, so it counts from the
// re-enabling. Every Start begins with the guard regarded as on.
func (a *antiLoopState) observeWaiver() bool {
	if a.waiver == nil {
		return false
	}
	waived := a.waiver()
	if waived == a.waivedSeen {
		return waived
	}
	a.waivedSeen = waived
	reset := !waived
	if reset {
		a.rowIndex = -1
		a.lastAgent = ""
		a.count = 0
		a.escalations = 0
	}
	if a.log != nil {
		a.log.Log(domain.EventSessionGuardApplied, "anti-loop guard switch observed",
			domain.F("waived", strconv.FormatBool(waived)),
			domain.F("count_reset", strconv.FormatBool(reset)),
		)
	}
	return waived
}
