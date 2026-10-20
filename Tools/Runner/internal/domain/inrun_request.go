package domain

// InRunRequest names a one-shot request made while a run is in progress. The
// string value is the "request" field written to the debug log.
type InRunRequest string

const (
	InRunRequestManualDispatch InRunRequest = "manual_dispatch"
	InRunRequestHITLNext       InRunRequest = "hitl_next"
)

// Debug log event names of the in-run requests.
const (
	EventTUIRequestSet         = "tui.request.set"
	EventTUIRequestCancelled   = "tui.request.cancelled"
	EventTUIRequestDiscarded   = "tui.request.discarded"
	EventSessionRequestApplied = "session.request.applied"
)
