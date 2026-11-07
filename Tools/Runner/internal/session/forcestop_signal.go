package session

import (
	"context"
	"sync"
)

// ForceStopSignal implements domain.ForceStopWatcher and is the TUI's way to
// request a force stop. Safe for concurrent use. Zero value is ready to use.
//
// A request only ever applies to the invocation window open at that moment; a
// request raised while nothing runs is discarded, so it can never kill a later
// invocation.
type ForceStopSignal struct {
	mu     sync.Mutex
	cancel context.CancelFunc // non-nil while a window is open
	forced bool               // a request arrived during the open window
}

// NewForceStopSignal returns a signal with no open window.
func NewForceStopSignal() *ForceStopSignal { return &ForceStopSignal{} }

// Request force-stops the invocation in the open window, if any. Returns true
// when a window was open (its context is now cancelled), false when no
// invocation was running (the request is discarded and affects no later
// window).
func (f *ForceStopSignal) Request() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancel == nil {
		return false
	}
	f.forced = true
	f.cancel()
	return true
}

// Watch implements domain.ForceStopWatcher.
func (f *ForceStopSignal) Watch(ctx context.Context) (context.Context, func() bool) {
	wctx, cancel := context.WithCancel(ctx)
	f.mu.Lock()
	f.cancel = cancel
	f.forced = false
	f.mu.Unlock()

	return wctx, func() bool {
		f.mu.Lock()
		forced := f.forced
		f.cancel = nil
		f.forced = false
		f.mu.Unlock()
		cancel()
		return forced
	}
}
