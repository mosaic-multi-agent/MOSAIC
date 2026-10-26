package session_test

// With the anti-loop guard switched off every other bound behaves as before:
// the PARTIALLY_DONE re-dispatch bound, the E501 budget, the review loop limit
// and the schedule-defect bound. These are regression tests: they protect
// behaviour that must not change when the waiver is added.

import (
	"context"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

func alwaysWaived() bool { return true }

func TestAntiLoopWaiver_GuardOff_MechanicalRetryBoundsStillStopTheRedispatch(t *testing.T) {
	cases := []struct {
		name  string
		entry harness.ScriptedEntry
		want  int
	}{
		{"PARTIALLY_DONE bound", statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half done"), 1 + engine.PartiallyDoneRedispatchLimit},
		{"E501 budget", e501Entry(), engine.E501AttemptLimit},
	}
	for _, mode := range autoModes {
		for _, tc := range cases {
			t.Run(string(mode)+"/"+tc.name, func(t *testing.T) {
				consultant := &scriptedRoutingConsultant{}
				consultant.queueStop("bound reached")
				rig := newWaiverRig(t, "linear-orch.md", consultant, nil, alwaysWaived)
				queueRepeated(rig.adapter, "agent-a", 20, tc.entry)

				got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

				requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
				if n := countInvocationsFor(rig.adapter.Invocations(), "agent-a"); n != tc.want {
					t.Errorf("want %d agent-a dispatches before the bound hands over to the consultant, got %d", tc.want, n)
				}
				if consultant.CallCount != 1 {
					t.Errorf("want exactly one consultation once the bound is reached, got %d", consultant.CallCount)
				}
			})
		}
	}
}

func TestAntiLoopWaiver_GuardOff_ScheduleDefectBoundStillStopsTheRedispatch(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.ses = session.New(session.Deps{
		Harness: rig.adapter, Store: rig.store, Routing: rig.consult,
		Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
		AntiLoopWaived: alwaysWaived,
	})
	rig.queueDefectivePlanner(10)
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 3 {
		t.Errorf("want 3 planner dispatches (first plus the 2 bounded re-dispatches), got %d", n)
	}
	if len(rig.consult.Requests) != 1 || rig.consult.Requests[0].Deviation == nil ||
		rig.consult.Requests[0].Deviation.Kind != domain.DeviationScheduleDefect {
		t.Fatalf("want one schedule-defect consultation once the bound is reached, got %+v", rig.consult.Requests)
	}
}

func TestAntiLoopWaiver_GuardOff_ReviewLoopLimitStillApplies(t *testing.T) {
	rig := reviewRig(t)
	rig.ses = session.New(session.Deps{
		Harness: rig.adapter, Store: rig.store, Routing: rig.consult,
		Approvals: rig.reader, Outputs: artifact.NewOutputWriteDetector(),
		Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
		AntiLoopWaived: alwaysWaived,
	})
	rig.adapter.Queue("agent-a",
		harness.ScriptedEntry{Response: resp(domain.StatusSUCCESS, "a done")},
		harness.ScriptedEntry{Response: resp(domain.StatusSUCCESS, "a fixed")},
	)
	rig.adapter.Queue("agent-b",
		reviewAttempt(domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone, "findings", "false", "v1"),
		reviewAttempt(domain.StatusSUCCESS, domain.ErrorNone, "repaired", "true", "v2"),
	)
	rig.consult.queueStop("done")

	rig.ses.Start(context.Background(), reviewConfig(rig, pathAuto, domain.ExecutionModeAutoReview, 1)) //nolint:errcheck

	dev := deviationOf(t, rig.consult)
	if dev.Kind != domain.DeviationReviewLoopLimit {
		t.Errorf("deviation kind: want %q, got %q", domain.DeviationReviewLoopLimit, dev.Kind)
	}
	if n := rig.invocationsOf("agent-a"); n != 1 {
		t.Errorf("agent-a must not be re-dispatched once the limit is reached, got %d invocation(s)", n)
	}
}
