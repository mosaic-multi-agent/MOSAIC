package tui

import (
	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens/runflow"
)

// Reasons recorded on a discarded-request debug entry.
const (
	requestDiscardRestart      = "restart"
	requestDiscardSessionStart = "session_start"
	requestDiscardRunEnd       = "run_end"
)

// requestAcks holds the status-line acknowledgements for one request kind.
type requestAcks struct {
	set, cancelled, alreadyApplied string
}

var requestAckTexts = map[domain.InRunRequest]requestAcks{
	domain.InRunRequestManualDispatch: {
		set:            "Manual dispatch requested - the manual routing dialogue opens at the next routing decision.",
		cancelled:      "Manual dispatch request cancelled.",
		alreadyApplied: "Manual dispatch request was already applied.",
	},
	domain.InRunRequestHITLNext: {
		set:            "HITL requested for the next agent.",
		cancelled:      "HITL request cancelled.",
		alreadyApplied: "HITL request was already applied.",
	},
}

// syncPendingRequests makes the progress screen show exactly what the shared
// object holds, so a request the session has taken stops being shown within one
// update.
func (m *rootModel) syncPendingRequests() {
	if m.progressScreen == nil {
		return
	}
	m.progressScreen.SetPendingRequests(runflow.PendingRequests{
		ManualDispatch: m.inRunRequests.Pending(domain.InRunRequestManualDispatch),
		HITLNext:       m.inRunRequests.Pending(domain.InRunRequestHITLNext),
	})
}

// applyRequestKeyEvents applies the request key presses the progress screen
// recorded: it sets or cancels the shared request, logs it, and acknowledges it
// on the status line.
func (m *rootModel) applyRequestKeyEvents() {
	for _, ev := range m.progressScreen.TakeRequestKeyEvents() {
		acks := requestAckTexts[ev.Request]
		if !ev.Cancel {
			m.inRunRequests.Request(ev.Request)
			m.debug.Log(domain.EventTUIRequestSet, "in-run request set",
				domain.F("request", string(ev.Request)), domain.F("key", ev.Key))
			m.progressScreen.SetStatus(acks.set, false)
			continue
		}
		outcome, text := "already_applied", acks.alreadyApplied
		if m.inRunRequests.Cancel(ev.Request) {
			outcome, text = "cancelled", acks.cancelled
		}
		m.debug.Log(domain.EventTUIRequestCancelled, "in-run request cancelled",
			domain.F("request", string(ev.Request)), domain.F("key", ev.Key), domain.F("outcome", outcome))
		m.progressScreen.SetStatus(text, false)
	}
	m.syncPendingRequests()
}

// discardRequests drops every pending in-run request and the screen's display
// of them, logging one entry per discarded request. Safe when no progress screen
// exists.
func (m *rootModel) discardRequests(reason string) {
	for _, kind := range m.inRunRequests.Reset() {
		m.debug.Log(domain.EventTUIRequestDiscarded, "in-run request discarded",
			domain.F("request", string(kind)), domain.F("reason", reason))
	}
	if m.progressScreen != nil {
		m.progressScreen.ResetRequestState()
	}
}
