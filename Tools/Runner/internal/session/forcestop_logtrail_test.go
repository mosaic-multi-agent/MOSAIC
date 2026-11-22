package session_test

// Tests that a force-stopped invocation and a cancelled run can be told apart
// in the debug log. Both leave the killed invocation's output entries and a
// harness.process.end with reason=cancelled; only the force stop is followed by
// the session's session.forcestop entry (naming the same agent), before the next
// invocation starts. The harness double writes the entries the CLI adapters
// write when their process is killed.
//
// These pin the existing session behaviour the log contract relies on, so they
// are regression coverage rather than new behaviour.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/testutil"
)

// killTrailHarness wraps a blocking harness: it logs the start of every
// invocation and, when the wrapped invocation is killed, the entries a CLI
// adapter leaves for a killed process.
type killTrailHarness struct {
	inner *blockingHarness
	log   *testutil.RecordingLogger
}

func (h *killTrailHarness) Invoke(ctx context.Context, agent domain.AgentReference, request domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	id := domain.F("agent", request.AgentInstanceID)
	h.log.Log(domain.EventHarnessInvokeStart, "", id)
	resp, err := h.inner.Invoke(ctx, agent, request)
	if err != nil {
		h.log.Log(domain.EventHarnessStdoutLines, "q1", id, domain.F("first", "1"), domain.F("lines", "1"))
		h.log.Log(domain.EventHarnessStderr, "err-before-kill", id)
		h.log.Log(domain.EventHarnessProcessEnd, "process ended: cancelled", id,
			domain.F("reason", "cancelled"), domain.F("stdout_bytes", "3"))
	}
	return resp, err
}

func indexOfEvent(entries []testutil.LogEntry, event string) int {
	for i, e := range entries {
		if e.Event == event {
			return i
		}
	}
	return -1
}

func TestSession_ForceStop_LogShowsTheKilledInvocationThenSessionForceStopForTheSameAgent(t *testing.T) {
	log := &testutil.RecordingLogger{}
	r := newKillTrailRig(t, log)

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	entries := log.Entries()
	end := indexOfEvent(entries, domain.EventHarnessProcessEnd)
	stop := indexOfEvent(entries, domain.EventSessionForceStop)
	if end < 0 || stop < 0 || stop < end {
		t.Fatalf("want harness.process.end followed by session.forcestop, got events %v", log.Events())
	}
	if v, _ := entries[end].Field("reason"); v != "cancelled" {
		t.Errorf("want the killed invocation to end with reason=cancelled, got %q", v)
	}
	endAgent, _ := entries[end].Field("agent")
	stopAgent, _ := entries[stop].Field("agent")
	if endAgent != "agent-a#1" || stopAgent != endAgent {
		t.Errorf("want both entries to name agent-a#1, got %q and %q", endAgent, stopAgent)
	}
	if next := indexOfEvent(entries[stop:], domain.EventHarnessInvokeStart); next >= 0 {
		t.Errorf("want session.forcestop before any later invocation starts, got events %v", log.Events())
	}
}

// newKillTrailRig builds the force-stop rig: the harness raises the force-stop
// request on the rig's own window.
func newKillTrailRig(t *testing.T, log *testutil.RecordingLogger) *forceStopRig {
	t.Helper()
	r := newForceStopRig(t, domain.ForceStopNextStop)
	inner := newBlockingHarness(r.f, r.win, "agent-a", 1)
	r.ses = session.New(session.Deps{
		Harness: &killTrailHarness{inner: inner, log: log}, Store: r.store, Clock: fixedClock{t: epoch},
		Interact: r.user, Routing: r.routing, Manual: r.manual, ForceStop: r.win, Debug: log,
	})
	return r
}

func TestSession_RunCancellation_LogShowsTheKilledInvocationButNoSessionForceStop(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextStop)
	log := &testutil.RecordingLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inner := newBlockingHarness(r.f, r.win, "agent-a", 1)
	inner.win = nil // no force-stop request: only the parent context is cancelled
	inner.onBlock = cancel
	r.ses = session.New(session.Deps{
		Harness: &killTrailHarness{inner: inner, log: log}, Store: r.store, Clock: fixedClock{t: epoch},
		Interact: r.user, Routing: r.routing, Manual: r.manual, ForceStop: r.win, Debug: log,
	})

	got, err := r.ses.Start(ctx, r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	entries := log.Entries()
	end := indexOfEvent(entries, domain.EventHarnessProcessEnd)
	if end < 0 {
		t.Fatalf("want the killed invocation's end entry, got events %v", log.Events())
	}
	if v, _ := entries[end].Field("reason"); v != "cancelled" {
		t.Errorf("want reason=cancelled, got %q", v)
	}
	if stop := indexOfEvent(entries, domain.EventSessionForceStop); stop >= 0 {
		t.Errorf("want no session.forcestop entry for a run cancellation, got events %v", log.Events())
	}
}
