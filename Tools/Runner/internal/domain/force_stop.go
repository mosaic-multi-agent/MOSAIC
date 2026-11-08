package domain

import (
	"context"

	"mosaic-common/interaction"
)

// ForceStopWatcher runs one invocation inside a force-stop window.
// Implementations are safe for concurrent use.
type ForceStopWatcher interface {
	// Watch opens a window for one invocation. wctx is derived from ctx and is
	// cancelled when a force stop is requested while the window is open.
	// release closes the window and reports whether a force stop was
	// requested during it; it is called exactly once. At most one window is
	// open at a time.
	Watch(ctx context.Context) (wctx context.Context, release func() (forced bool))
}

// ForceStoppedSummary is the Summary of every force-stopped attempt's row.
const ForceStoppedSummary = "Force-stopped by the user; attempt not accepted"

// QuestionForceStopNext is the post-kill SelectOne question.
const QuestionForceStopNext interaction.QuestionID = "force-stop.next"

// Option IDs of QuestionForceStopNext.
const (
	ForceStopNextStop   = "stop"   // label "Stop the run (resumable)"; DefaultOptionID
	ForceStopNextManual = "manual" // label "Open the manual routing dialogue"
)

// Debug log event names of the force stop.
const (
	EventSessionForceStop         = "session.forcestop"           // fields: agent, target
	EventTUIForceStopGateEntered  = "tui.forcestop.gate.entered"  // fields: key
	EventTUIForceStopGateResolved = "tui.forcestop.gate.resolved" // fields: key, confirmed
	EventTUIForceStopRequested    = "tui.forcestop.requested"     // fields: applied
)

// NopForceStopWatcher opens windows that are never force-stopped.
type NopForceStopWatcher struct{}

// Watch implements ForceStopWatcher: ctx is returned unchanged.
func (NopForceStopWatcher) Watch(ctx context.Context) (context.Context, func() bool) {
	return ctx, func() bool { return false }
}

// IsForceStopped reports whether e records a force-stopped attempt. Such a row
// is invisible to every log-derived counter.
func (e ExecutionLogEntry) IsForceStopped() bool {
	return e.Status == StatusPARTIALLY_DONE && e.Summary == ForceStoppedSummary
}
