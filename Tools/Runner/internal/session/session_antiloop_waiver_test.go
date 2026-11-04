package session_test

// Tests for the session-only waiver of the anti-loop guard: with the guard
// switched off the consultant-routed same-agent sequence is not stopped, a
// switch applies from the next routing decision, switching back on counts
// from the re-enabling, and every observed switch is written to the Runner log.

import (
	"context"
	"reflect"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// queueAgentADispatches queues n consultant dispatches of agent-a (row 1).
func queueAgentADispatches(c *scriptedRoutingConsultant, n int) {
	for i := 0; i < n; i++ {
		c.queueDispatch("agent-a", "attempt", 0)
	}
}

func startWaiverRun(t *testing.T, rig *waiverRig) (domain.RunOutcome, error) {
	t.Helper()
	return rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))
}

// ===== the guard off: the sequence that would stop the run does not =====

func TestAntiLoopWaiver_GuardOff_ConsultantRoutedSequenceBeyondTheLimitKeepsDispatching(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	queueAgentADispatches(consultant, 6)
	consultant.queueStop("consultant finished the run")
	sw := &waiverSwitch{}
	sw.set(true)
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	rig.adapter.Queue("agent-a", partiallyDoneEntries(8)...)

	got, err := startWaiverRun(t, rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if got.StopReason != "consultant finished the run" {
		t.Errorf("want the run to end with the consultant's own stop, got reason %q (message %q)", got.StopReason, got.Message)
	}
	if n := countInvocationsFor(rig.adapter.Invocations(), "agent-a"); n != 6 {
		t.Errorf("want all 6 requested agent-a dispatches to run with the guard off, got %d", n)
	}
	if n := rig.guardTriggeredCount(); n != 0 {
		t.Errorf("want no guard escalation with the guard off, got %d", n)
	}
	if got.AntiLoopStop {
		t.Error("AntiLoopStop = true for a run the guard did not stop")
	}
}

func TestAntiLoopWaiver_GuardOff_HITLRedispatchSitesAreNotBlocked(t *testing.T) {
	// Each consultant dispatch of the HITL agent is the initial dispatch plus
	// one automatic HITL re-dispatch; with the guard on the fifth invocation is
	// blocked (the existing guard tests count four).
	cases := []struct {
		name    string
		fixture string
		auto    bool
		queued  int
	}{
		{"orchestrated, consult-path re-dispatch", "hitl-linear-orch.md", false, 3},
		{"auto, engine-path re-dispatch", "hitl-linear-orch.md", true, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			queueAgentADispatches(consultant, tc.queued)
			consultant.queueStop("consultant finished the run")
			sw := &waiverSwitch{}
			sw.set(true)
			rig := newWaiverRig(t, tc.fixture, consultant, &fixedApprovalReader{domain.ApprovalFalse}, sw.Waived)
			rig.adapter.Queue("agent-a", repeatedEntries(12, statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))...)
			cfg := baseOrchestratedConfig(rig.orchPath)
			if tc.auto {
				cfg.RunSettings.Mode = domain.ExecutionModeAuto
			}

			got, err := rig.ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := countInvocationsFor(rig.adapter.Invocations(), "agent-a"); n != 6 {
				t.Errorf("want 6 agent-a invocations (no dispatch blocked), got %d", n)
			}
			if n := rig.guardTriggeredCount(); n != 0 {
				t.Errorf("want no guard escalation with the guard off, got %d", n)
			}
		})
	}
}

// With the guard off the raw-text re-dispatch after a non-JSON reply is not
// bounded by the guard: it runs even where the guard would have blocked it.
func TestAntiLoopWaiver_GuardOff_RawTextBypassIsNotBlocked(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	queueAgentADispatches(consultant, 4)
	consultant.queueStop("consultant finished the run")
	sw := &waiverSwitch{}
	sw.set(true)
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	// Dispatches 1-3 are partial; the 4th reply is not JSON, and its bypass
	// re-dispatch would be the 5th consecutive dispatch.
	rig.adapter.Queue("agent-a", partiallyDoneEntries(3)...)
	rig.adapter.Queue("agent-a",
		harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrMalformedJSON)},
		statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done on the bypass"),
	)

	got, err := startWaiverRun(t, rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(rig.adapter.Invocations(), "agent-a"); n != 5 {
		t.Errorf("want 5 agent-a invocations (the bypass re-dispatch ran), got %d", n)
	}
}

// ===== a switch applies from the next routing decision =====

func TestAntiLoopWaiver_SwitchingOff_AppliesFromTheNextRoutingDecision(t *testing.T) {
	scripted := &scriptedRoutingConsultant{}
	queueAgentADispatches(scripted, 7)
	scripted.queueStop("consultant finished the run")
	sw := &waiverSwitch{}
	// Four dispatches run with the guard on; it is switched off before the
	// answer to the fifth consultation.
	consultant := &switchingConsultant{inner: scripted, before: map[int]func(){5: func() { sw.set(true) }}}
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	rig.adapter.Queue("agent-a", partiallyDoneEntries(8)...)

	got, err := startWaiverRun(t, rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(rig.adapter.Invocations(), "agent-a"); n != 7 {
		t.Errorf("want 7 agent-a dispatches (the fifth and later ones run after the switch), got %d", n)
	}
	if n := rig.guardTriggeredCount(); n != 0 {
		t.Errorf("want no guard escalation, got %d", n)
	}
	applied := rig.guardAppliedEntries()
	if len(applied) != 1 {
		t.Fatalf("want one session.guard.applied entry for the switch, got %d (events %v)", len(applied), rig.logger.allEvents())
	}
	if got := fieldOf(applied[0], "waived"); got != "true" {
		t.Errorf("session.guard.applied waived = %q, want \"true\"", got)
	}
	if starts := rig.dispatchStartsBefore(0); starts != 4 {
		t.Errorf("want the switch observed after 4 dispatches and before the fifth, got %d dispatch starts before it", starts)
	}
}

func TestAntiLoopWaiver_SwitchingBackOn_CountsFromTheReEnabling(t *testing.T) {
	scripted := &scriptedRoutingConsultant{}
	queueAgentADispatches(scripted, 11) // 6 with the guard off, then 4 allowed + 1 blocked
	scripted.queueStop("consultant finished the run")
	sw := &waiverSwitch{}
	sw.set(true)
	consultant := &switchingConsultant{inner: scripted, before: map[int]func(){7: func() { sw.set(false) }}}
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	rig.adapter.Queue("agent-a", partiallyDoneEntries(14)...)

	got, err := startWaiverRun(t, rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	// Six dispatches ran with the guard off. After re-enabling, the first four
	// consecutive dispatches run and the fifth is the first the guard blocks.
	if n := countInvocationsFor(rig.adapter.Invocations(), "agent-a"); n != 10 {
		t.Errorf("want 10 agent-a dispatches (6 waived + 4 after re-enabling), got %d", n)
	}
	if n := rig.guardTriggeredCount(); n != 1 {
		t.Errorf("want the guard to fire once, on the fifth dispatch after re-enabling, got %d", n)
	}
}

// ===== every observed switch is logged =====

func TestAntiLoopWaiver_EveryObservedSwitchIsLogged(t *testing.T) {
	scripted := &scriptedRoutingConsultant{}
	queueAgentADispatches(scripted, 8)
	scripted.queueStop("consultant finished the run")
	sw := &waiverSwitch{}
	sw.set(true)
	consultant := &switchingConsultant{inner: scripted, before: map[int]func(){7: func() { sw.set(false) }}}
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	rig.adapter.Queue("agent-a", partiallyDoneEntries(10)...)

	startWaiverRun(t, rig) //nolint:errcheck

	applied := rig.guardAppliedEntries()
	if len(applied) != 2 {
		t.Fatalf("want exactly 2 session.guard.applied entries (off, then on again), got %d", len(applied))
	}
	if got := fieldOf(applied[0], "waived"); got != "true" {
		t.Errorf("first entry waived = %q, want \"true\" (a Start whose first check reads true logs it)", got)
	}
	if got := fieldOf(applied[0], "count_reset"); got == "true" {
		t.Error("first entry count_reset = \"true\", want not set: switching off resets nothing")
	}
	if got := fieldOf(applied[1], "waived"); got != "false" {
		t.Errorf("second entry waived = %q, want \"false\"", got)
	}
	if got := fieldOf(applied[1], "count_reset"); got != "true" {
		t.Errorf("second entry count_reset = %q, want \"true\" on re-enabling", got)
	}
}

func TestAntiLoopWaiver_GuardNeverSwitched_LogsNoSwitch(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	queueAgentADispatches(consultant, 3)
	consultant.queueStop("consultant finished the run")
	sw := &waiverSwitch{}
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	rig.adapter.Queue("agent-a", partiallyDoneEntries(4)...)

	startWaiverRun(t, rig) //nolint:errcheck

	if n := len(rig.guardAppliedEntries()); n != 0 {
		t.Errorf("want no session.guard.applied entry while the guard stays on, got %d", n)
	}
}

// ===== the waiver is session-only =====

// Each Start begins with the guard regarded as on: the second Start of the
// same session logs the switch again even though the waiver never changed.
func TestAntiLoopWaiver_NewStartBeginsWithTheGuardOn(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "attempt", 0)
	consultant.queueStop("first stop")
	consultant.queueDispatch("agent-a", "attempt", 0)
	consultant.queueStop("second stop")
	sw := &waiverSwitch{}
	sw.set(true)
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	rig.adapter.Queue("agent-a", partiallyDoneEntries(4)...)

	first, err := startWaiverRun(t, rig)
	requireRunStatus(t, first, err, domain.RunStoppedByConsultant)
	resume := baseOrchestratedConfig(rig.orchPath)
	markResume(&resume)
	second, err := rig.ses.Start(context.Background(), resume)
	requireRunStatus(t, second, err, domain.RunStoppedByConsultant)

	applied := rig.guardAppliedEntries()
	if len(applied) != 2 {
		t.Fatalf("want one session.guard.applied entry per Start (2), got %d: nothing may carry over from one Start to the next", len(applied))
	}
	for i, e := range applied {
		if got := fieldOf(e, "waived"); got != "true" {
			t.Errorf("entry %d waived = %q, want \"true\"", i, got)
		}
	}
}

// Regression coverage: the waiver never reaches the artifact.
func TestAntiLoopWaiver_NothingAboutTheWaiverIsWrittenToTheArtifact(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	queueAgentADispatches(consultant, 5)
	consultant.queueStop("consultant finished the run")
	sw := &waiverSwitch{}
	sw.set(true)
	rig := newWaiverRig(t, "linear-orch.md", consultant, nil, sw.Waived)
	rig.adapter.Queue("agent-a", partiallyDoneEntries(6)...)

	startWaiverRun(t, rig) //nolint:errcheck

	want := domain.RunSettings{Mode: domain.ExecutionModeOrchestrated}
	if !reflect.DeepEqual(rig.store.state.RunSettings, want) {
		t.Errorf("recorded run settings = %+v, want %+v: the waiver is not a recorded setting", rig.store.state.RunSettings, want)
	}
	if len(rig.store.AdoptCalls) != 0 || len(rig.store.BranchCalls) != 0 {
		t.Errorf("want no settings or branch write, got adopt=%v branch=%v", rig.store.AdoptCalls, rig.store.BranchCalls)
	}
}
