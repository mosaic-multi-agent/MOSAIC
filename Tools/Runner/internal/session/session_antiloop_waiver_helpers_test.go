package session_test

// Shared fixtures for the anti-loop guard waiver tests: a stand-in for the
// TUI-owned waiver, a consultant that changes the waiver between routing
// decisions, and session builders that wire Deps.AntiLoopWaived.

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// waiverSwitch stands in for the TUI-owned waiver: tests flip it, the session
// reads it through Deps.AntiLoopWaived.
type waiverSwitch struct{ on atomic.Bool }

func (w *waiverSwitch) Waived() bool { return w.on.Load() }

func (w *waiverSwitch) set(waived bool) { w.on.Store(waived) }

// switchingConsultant forwards every consultation to inner. Before it answers
// consultation number n (1-based) it runs before[n], so a test changes the
// waiver exactly between two routing decisions.
type switchingConsultant struct {
	inner  domain.RoutingConsultant
	before map[int]func()
	calls  int
}

func (c *switchingConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.calls++
	if hook := c.before[c.calls]; hook != nil {
		hook()
	}
	return c.inner.ConsultRouting(ctx, req)
}

// partiallyDoneEntries scripts n PARTIALLY_DONE replies.
func partiallyDoneEntries(n int) []harness.ScriptedEntry {
	return repeatedEntries(n, statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "still working"))
}

// waiverRig is a session over the linear fixture with a Runner-log recorder
// and the waiver wired through Deps.AntiLoopWaived.
type waiverRig struct {
	ses      session.Session
	adapter  *harness.MockAdapter
	store    *memStore
	logger   *sessionRecordingLogger
	orchPath string
}

// newWaiverRig builds the rig. approvals may be nil; hitl selects the HITL
// fixture (agent-a has HITL=true). waived may be nil (guard always on).
func newWaiverRig(t *testing.T, fixture string, consultant domain.RoutingConsultant,
	approvals domain.ApprovalReader, waived func() bool) *waiverRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, fixture)
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")

	f := harness.NewMockAdapter()
	store := &memStore{}
	logger := &sessionRecordingLogger{}
	ses := session.New(session.Deps{
		Harness:        f,
		Store:          store,
		Routing:        consultant,
		Approvals:      approvals,
		Clock:          fixedClock{t: epoch},
		Interact:       &noopInteraction{},
		Debug:          logger,
		AntiLoopWaived: waived,
	})
	return &waiverRig{ses: ses, adapter: f, store: store, logger: logger, orchPath: orchPath}
}

// guardAppliedEntries returns the session.guard.applied entries in log order.
func (r *waiverRig) guardAppliedEntries() []sessionLogEntry {
	return r.logger.entriesFor(domain.EventSessionGuardApplied)
}

// guardTriggeredCount counts the "anti-loop guard triggered" deviation entries.
func (r *waiverRig) guardTriggeredCount() int {
	n := 0
	for _, e := range r.logger.entriesFor(domain.EventSessionDeviation) {
		if strings.Contains(e.Message, "anti-loop guard triggered") {
			n++
		}
	}
	return n
}

// dispatchStartsBefore counts the dispatch-start events logged before the
// idx-th (0-based) session.guard.applied entry.
func (r *waiverRig) dispatchStartsBefore(idx int) int {
	seen, starts := 0, 0
	for _, name := range r.logger.allEvents() {
		switch name {
		case domain.EventSessionDispatchStart:
			starts++
		case domain.EventSessionGuardApplied:
			if seen == idx {
				return starts
			}
			seen++
		}
	}
	return -1
}

