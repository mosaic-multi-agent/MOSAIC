package runflow

import (
	"strings"

	"mosaic-run/internal/domain"
)

const (
	helpBarText = "s graceful-stop  m manual-dispatch  h hitl-next  g loop-guard  a artifact-view  ctrl+c force-quit"

	pendingManualDispatchText = "Requested: manual dispatch at the next routing decision (m to cancel)"
	pendingHITLNextText       = "Requested: HITL for the next agent (h to cancel)"
)

// PendingRequests is what the progress screen shows as pending.
type PendingRequests struct {
	ManualDispatch bool
	HITLNext       bool
}

// RequestKeyEvent records one accepted request key press for the root model to
// apply and log. Ignored keys record nothing.
type RequestKeyEvent struct {
	Request domain.InRunRequest
	Key     string // verbatim bubbletea key: "m", "M", "h", "H"
	Cancel  bool   // true when the screen showed the request pending at the press
}

// SetPendingRequests sets which in-run requests the screen shows as pending.
func (s *ProgressScreen) SetPendingRequests(p PendingRequests) { s.pendingRequests = p }

// PendingRequests returns what the screen currently shows as pending.
func (s *ProgressScreen) PendingRequests() PendingRequests { return s.pendingRequests }

// TakeRequestKeyEvents returns the request key presses recorded since the last
// call and clears them.
func (s *ProgressScreen) TakeRequestKeyEvents() []RequestKeyEvent {
	events := s.requestEvents
	s.requestEvents = nil
	return events
}

// ResetRequestState clears the pending display and any undrained request key
// events.
func (s *ProgressScreen) ResetRequestState() {
	s.pendingRequests = PendingRequests{}
	s.requestEvents = nil
}

// recordRequestKey records a request key press and reports whether key is one.
// It records nothing for any other key. The caller has already established
// that no stop is pending or confirmed.
func (s *ProgressScreen) recordRequestKey(key string) bool {
	var kind domain.InRunRequest
	var shown bool
	switch key {
	case "m", "M":
		kind, shown = domain.InRunRequestManualDispatch, s.pendingRequests.ManualDispatch
	case "h", "H":
		kind, shown = domain.InRunRequestHITLNext, s.pendingRequests.HITLNext
	default:
		return false
	}
	s.requestEvents = append(s.requestEvents, RequestKeyEvent{Request: kind, Key: key, Cancel: shown})
	return true
}

// pendingLines renders one line per pending request, or "" when none.
func (s *ProgressScreen) pendingLines() string {
	var lines []string
	if s.pendingRequests.ManualDispatch {
		lines = append(lines, s.styles.Warning.Width(s.width).Render(pendingManualDispatchText))
	}
	if s.pendingRequests.HITLNext {
		lines = append(lines, s.styles.Warning.Width(s.width).Render(pendingHITLNextText))
	}
	return strings.Join(lines, "\n")
}
