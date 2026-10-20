package session_test

// Tests of the in-run HITL request at consultant-routed and manual-routed
// dispatches: a pending request wins over a false HITL override in the
// orchestrator's instruction and over "off" in the manual dialogue, a routing
// consultation never consumes it, and without a request both paths behave as
// before.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// hitlLinearRig is a rig over the two-step workflow whose steps both have HITL
// on in the table, with the orchestrator scripted by consultant.
func hitlLinearRig(t *testing.T, approvals *approvalLog, consultant *scriptedRoutingConsultant) *inRunRig {
	t.Helper()
	return newHITLRig(t, "hitl-linear-orch.md", linearAgents, approvals, consultant, nil)
}

func startOrchestrated(rig *inRunRig) (domain.RunOutcome, error) {
	return rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))
}

func TestSession_InRunHITL_ConsultantInstructionWithHITLOverrideFalse_StillRunsWithHITLWhileRequestPending(t *testing.T) {
	off := false
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "plan", 0)
	consultant.queueDispatchWithHITL("agent-b", "build", 1, &off)
	consultant.queueStop("escalated")
	approvals := &approvalLog{answer: refuseFile("result.md")}
	rig := hitlLinearRig(t, approvals, consultant)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 2)
	rig.requestAfter("agent-a", rig.hitl)

	got, err := startOrchestrated(rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := []string{"agent-a=true", "agent-b=true", "agent-b=true"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want the override overridden and the gate re-dispatching: %v, got %v", want, seq)
	}
	if approvals.readCount("result.md") == 0 {
		t.Error("want the raised step's output artifact checked for approval")
	}
	if rig.hitl.isPending() {
		t.Error("want the request consumed")
	}
}

func TestSession_InRunHITL_ConsultantInstructionWithHITLOverrideFalse_NoRequestPending_StillSuppressesHITL(t *testing.T) {
	off := false
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "plan", 0)
	consultant.queueDispatchWithHITL("agent-b", "build", 1, &off)
	consultant.queueStop("done")
	approvals := &approvalLog{answer: refuseFile("result.md")}
	rig := hitlLinearRig(t, approvals, consultant)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)

	got, err := startOrchestrated(rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := []string{"agent-a=true", "agent-b=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want %v, got %v", want, seq)
	}
	if n := approvals.readCount("result.md"); n != 0 {
		t.Errorf("want no approval check without HITL, got %d reads", n)
	}
}

func TestSession_InRunHITL_ConsultantDispatchOfUnflaggedRow_RaisesNextStepOnlyOnce(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "plan", 0)
	consultant.queueDispatch("agent-b", "build", 1)
	consultant.queueDispatch("agent-a", "again", 0)
	consultant.queueStop("done")
	rig := newHITLRig(t, "linear-orch.md", linearAgents, &approvalLog{}, consultant, nil)
	rig.queueOK("agent-a", 2)
	rig.queueOK("agent-b", 1)
	rig.requestAfter("agent-a", rig.hitl)

	got, err := startOrchestrated(rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := []string{"agent-a=false", "agent-b=true", "agent-a=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want the first dispatch after the request raised only: %v, got %v", want, seq)
	}
	if f := rig.appliedFor(domain.InRunRequestHITLNext); f == nil || !strings.Contains(f["agent"], "agent-b") {
		t.Errorf("want the application logged for agent-b, got %v", f)
	}
}

func TestSession_InRunHITL_RoutingConsultation_DoesNotConsumeTheRequest(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "plan", 0)
	consultant.queueStop("stop after the consultation")
	rig := newHITLRig(t, "linear-orch.md", linearAgents, &approvalLog{}, consultant, nil)
	rig.queueOK("agent-a", 1)
	rig.requestAfter("agent-a", rig.hitl)

	got, err := startOrchestrated(rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if consultant.CallCount != 2 {
		t.Fatalf("want the consultation after agent-a to have run, got %d consultations", consultant.CallCount)
	}
	if !rig.hitl.isPending() {
		t.Error("want the request still pending: a consultation is not a workflow dispatch")
	}
	if n := rig.appliedCount(); n != 0 {
		t.Errorf("want no application logged, got %d", n)
	}
}

func TestSession_InRunHITL_ManualDialogueChosenStepWithHITLOff_RunsWithHITLWhileRequestPending(t *testing.T) {
	approvals := &approvalLog{}
	rig := newHITLRig(t, "linear-orch.md", linearAgents, approvals, nil, dialogueWithHITL(t, "1", "redo", domain.ManualHITLOff))
	rig.queueOK("agent-a", 2)
	rig.queueOK("agent-b", 1)
	rig.requestAfter("agent-a", rig.flag, rig.hitl)

	got, err := rig.startAuto(domain.ExecutionModeAuto)

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := []string{"agent-a=false", "agent-a=true", "agent-b=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want the dialogue's off overridden for that step only: %v, got %v", want, seq)
	}
	if approvals.readCount("plan.md") == 0 {
		t.Error("want the raised step's output artifact checked for approval")
	}
	if rig.hitl.isPending() {
		t.Error("want the request consumed")
	}
	if f := rig.appliedFor(domain.InRunRequestHITLNext); f == nil || f["row"] != "1" {
		t.Errorf("want the application logged for row 1, got %v", f)
	}
}

func TestSession_InRunHITL_ManualDialogueChosenStepWithHITLOff_NoRequestPending_RunsWithoutHITL(t *testing.T) {
	approvals := &approvalLog{}
	rig := newHITLRig(t, "linear-orch.md", linearAgents, approvals, nil, dialogueWithHITL(t, "1", "redo", domain.ManualHITLOff))
	rig.queueOK("agent-a", 2)
	rig.queueOK("agent-b", 1)
	rig.requestAfter("agent-a", rig.flag)

	got, err := rig.startAuto(domain.ExecutionModeAuto)

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := []string{"agent-a=false", "agent-a=false", "agent-b=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want %v, got %v", want, seq)
	}
	if f := rig.appliedFor(domain.InRunRequestHITLNext); f != nil {
		t.Errorf("want no HITL application logged, got %v", f)
	}
}

func TestSession_InRunHITL_ConsultantRaisedStepRefusedOnce_RedispatchKeepsHITLAndNextDispatchIsNormal(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "plan", 0)
	consultant.queueDispatch("agent-b", "build", 1)
	consultant.queueDispatch("agent-a", "again", 0)
	consultant.queueStop("done")
	approvals := &approvalLog{answer: func(file string, nth int) domain.HumanApproval {
		if file == "result.md" && nth == 1 {
			return domain.ApprovalFalse
		}
		return domain.ApprovalTrue
	}}
	rig := newHITLRig(t, "linear-orch.md", linearAgents, approvals, consultant, nil)
	rig.queueOK("agent-a", 2)
	rig.queueOK("agent-b", 2)
	rig.requestAfter("agent-a", rig.hitl)

	got, err := startOrchestrated(rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := []string{"agent-a=false", "agent-b=true", "agent-b=true", "agent-a=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want the gate's re-dispatch of the raised step to keep HITL: %v, got %v", want, seq)
	}
}

// In orchestrated mode an E501 failure is not retried mechanically: the
// orchestrator is consulted and its next dispatch is a new routing decision.

func TestSession_InRunHITL_ConsultantRaisedStepFailingWithE501_ConsultantRedispatchIsNormal(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "plan", 0)
	consultant.queueDispatch("agent-b", "build", 1)
	consultant.queueDispatch("agent-b", "build again", 1)
	consultant.queueStop("done")
	rig := newHITLRig(t, "linear-orch.md", linearAgents, &approvalLog{}, consultant, nil)
	rig.queueOK("agent-a", 1)
	rig.f.Queue("agent-b", e501Entry(), okEntry())
	rig.requestAfter("agent-a", rig.hitl)

	got, err := startOrchestrated(rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := []string{"agent-a=false", "agent-b=true", "agent-b=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want the consultant's re-dispatch after the failure to use its normal HITL: %v, got %v", want, seq)
	}
	if rig.hitl.isPending() {
		t.Error("want the request consumed by the raised dispatch")
	}
}

func TestSession_InRunHITL_ConsultantStepFailingWithE501_NextConsultantDispatchIsRaised(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "plan", 0)
	consultant.queueDispatch("agent-a", "plan again", 0)
	consultant.queueDispatch("agent-b", "build", 1)
	consultant.queueStop("done")
	rig := newHITLRig(t, "linear-orch.md", linearAgents, &approvalLog{}, consultant, nil)
	rig.f.Queue("agent-a", e501Entry(), okEntry())
	rig.queueOK("agent-b", 1)
	rig.requestAfter("agent-a", rig.hitl)

	got, err := startOrchestrated(rig)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := []string{"agent-a=false", "agent-a=true", "agent-b=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want the consultant's dispatch after the failure raised, then normal: %v, got %v", want, seq)
	}
	if rig.hitl.isPending() {
		t.Error("want the request consumed by the next consultant dispatch")
	}
}

func TestSession_InRunHITL_ManualDialogueChosenStepWithHITLDefaultOrOn_RunsWithHITLAndConsumesTheRequest(t *testing.T) {
	for _, tc := range []struct{ option, prior string }{
		{domain.ManualHITLDefault, "false"}, {domain.ManualHITLOn, "true"},
	} {
		option, prior := tc.option, tc.prior
		t.Run(option, func(t *testing.T) {
			rig := newHITLRig(t, "linear-orch.md", linearAgents, &approvalLog{}, nil, dialogueWithHITL(t, "1", "redo", option))
			rig.queueOK("agent-a", 2)
			rig.queueOK("agent-b", 1)
			rig.requestAfter("agent-a", rig.flag, rig.hitl)

			got, err := rig.startAuto(domain.ExecutionModeAuto)

			requireRunStatus(t, got, err, domain.RunCompleted)
			want := []string{"agent-a=false", "agent-a=true", "agent-b=false"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Errorf("want the chosen step raised for that step only: %v, got %v", want, seq)
			}
			if rig.hitl.isPending() {
				t.Error("want the request consumed")
			}
			if f := rig.appliedFor(domain.InRunRequestHITLNext); f == nil || f["prior_hitl"] != prior {
				t.Errorf("want the application logged with prior_hitl=%s, got %v", prior, f)
			}
		})
	}
}
