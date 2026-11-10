package session

import (
	"sync"

	"mosaic-run/internal/domain"
)

// InRunRequests holds the pending one-shot in-run requests. The TUI goroutine
// sets, cancels and resets; the session goroutine takes. Every request that
// becomes pending ends in exactly one of: taken (applied), cancelled, or reset
// (discarded). Safe for concurrent use by multiple goroutines. The zero value
// is not usable; construct with NewInRunRequests.
type InRunRequests struct {
	mu      sync.Mutex
	pending map[domain.InRunRequest]bool
}

// inRunRequestOrder is the order Reset reports discarded kinds.
var inRunRequestOrder = []domain.InRunRequest{
	domain.InRunRequestManualDispatch,
	domain.InRunRequestHITLNext,
}

// NewInRunRequests returns an instance with no request pending.
func NewInRunRequests() *InRunRequests {
	return &InRunRequests{pending: make(map[domain.InRunRequest]bool, len(inRunRequestOrder))}
}

func isKnownInRunRequest(kind domain.InRunRequest) bool {
	for _, k := range inRunRequestOrder {
		if k == kind {
			return true
		}
	}
	return false
}

// Request makes kind pending. Returns true when kind was not pending before,
// false when it was already pending or kind is unknown.
func (r *InRunRequests) Request(kind domain.InRunRequest) bool {
	if !isKnownInRunRequest(kind) {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending[kind] {
		return false
	}
	r.pending[kind] = true
	return true
}

// Cancel clears a pending kind. Returns true when kind was pending.
func (r *InRunRequests) Cancel(kind domain.InRunRequest) bool {
	return r.Take(kind)
}

// Pending reports whether kind is pending. For display only.
func (r *InRunRequests) Pending(kind domain.InRunRequest) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending[kind]
}

// Take reports whether kind was pending and clears it in the same atomic
// operation.
func (r *InRunRequests) Take(kind domain.InRunRequest) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	was := r.pending[kind]
	delete(r.pending, kind)
	return was
}

// Reset clears every pending kind and returns the kinds that were pending, in
// InRunRequestManualDispatch, InRunRequestHITLNext order; nil when none.
func (r *InRunRequests) Reset() []domain.InRunRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	var cleared []domain.InRunRequest
	for _, k := range inRunRequestOrder {
		if r.pending[k] {
			cleared = append(cleared, k)
		}
	}
	clear(r.pending)
	return cleared
}
