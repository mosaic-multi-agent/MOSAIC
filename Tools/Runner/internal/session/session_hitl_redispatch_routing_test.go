package session_test

// Tests of the routing of a failed HITL repair re-dispatch: a raw-text failure
// is bypassed by one direct re-dispatch that keeps the step's HITL value,
// gated only by the anti-loop guard (never by the HARNESS_ERROR budget); when
// the bypass also fails, the consultation follows at once.

import (
	"context"
	"fmt"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// repairRawText is a bypass-eligible failure of the repair re-dispatch.
func repairRawText() harness.ScriptedEntry {
	return harness.ScriptedEntry{Err: fmt.Errorf("repair reply: %w", commonharness.ErrProtocolNotExtractable)}
}

// requireBypassKeptHITL asserts the last two agent-a dispatches (the failed
// repair and its bypass) carry the same HITL value.
func requireBypassKeptHITL(t *testing.T, f *harness.MockAdapter) {
	t.Helper()
	var flags []bool
	for _, inv := range f.Invocations() {
		if inv.Agent.Identifier == "agent-a" {
			flags = append(flags, inv.Request.HumanInTheLoop)
		}
	}
	if len(flags) < 2 || flags[len(flags)-1] != flags[len(flags)-2] {
		t.Errorf("want the bypass to keep the HITL value of the failed repair, got %v", flags)
	}
}

func TestSession_HITLRepairRedispatchBypass_EnginePath_BypassSucceeds_RunContinuesWithoutConsulting(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	ses, f, store, orchPath := newAutoHITLSession(t, firstDisapprovedThenApproved(), consultant)
	f.Queue("agent-a", successEntry("plan v1"), repairRawText(), successEntry("plan v2"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if consultant.CallCount != 0 {
		t.Errorf("want no consultation when the bypass succeeds, got %d", consultant.CallCount)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 3 {
		t.Errorf("want first attempt, failed repair and bypass, got %d agent-a dispatches", n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 1 {
		t.Errorf("want one HARNESS_ERROR row for the failed repair, got %d", n)
	}
	requireBypassKeptHITL(t, f)
}

func TestSession_HITLRepairRedispatchBypass_EnginePath_BypassFails_ConsultsAtOnce(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	ses, f, store, orchPath := newAutoHITLSession(t, firstDisapprovedThenApproved(), consultant)
	f.Queue("agent-a", successEntry("plan v1"), repairRawText(), repairTimeout(), successEntry("never dispatched"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(consultant.Requests) != 1 {
		t.Fatalf("want one consultation straight after the failed bypass, got %d", len(consultant.Requests))
	}
	requireHarnessErrorDeviation(t, consultant.Requests[0])
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 3 {
		t.Errorf("want no mechanical re-dispatch after the failed bypass, got %d agent-a dispatches", n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 2 {
		t.Errorf("want a HARNESS_ERROR row for the failed repair and for the failed bypass, got %d", n)
	}
	requireNoSubagentLookingBlocked(t, store, "agent-a")
}

func TestSession_HITLRepairRedispatchBypass_EnginePath_SpentHarnessBudget_StillBypassed(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	ses, f, _, orchPath := newAutoHITLSession(t, firstDisapprovedThenApproved(), consultant)
	f.Queue("agent-a", repairTimeout(), repairTimeout(), successEntry("plan v1"), repairRawText(), successEntry("plan v2"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if consultant.CallCount != 0 {
		t.Errorf("want the bypass attempted although the HARNESS_ERROR budget is spent, got %d consultations", consultant.CallCount)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 5 {
		t.Errorf("want two failed dispatches, the first success, the failed repair and its bypass, got %d", n)
	}
}
