package session_test

// A producing row that needs human approval: the schedule is judged only for
// an attempt the HITL compliance loop accepted, the schedule-defect
// re-dispatch passes the HITL compliance loop again, and HITL-rejected
// attempts count as attempts toward the re-dispatch limit.

import (
	"context"
	"strings"
	"sync"
	"testing"

	"mosaic-run/internal/domain"
)

// sequencedApprovalReader answers the n-th call with answers[n], repeating the
// last answer once the list is used up, and counts the calls. Each dispatch of
// the single-output planner row asks once.
type sequencedApprovalReader struct {
	mu      sync.Mutex
	answers []domain.HumanApproval
	calls   int
}

func (r *sequencedApprovalReader) ReadApproval(_ context.Context, _ string) domain.HumanApproval {
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.calls
	r.calls++
	if i >= len(r.answers) {
		i = len(r.answers) - 1
	}
	return r.answers[i]
}

func (r *sequencedApprovalReader) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

const (
	sdApprove = domain.ApprovalTrue
	sdDeny    = domain.ApprovalFalse
)

// newHITLDefectRig is the rig over the fixture whose planner needs approval.
func newHITLDefectRig(t *testing.T, approvals domain.ApprovalReader) *blockRig {
	t.Helper()
	return newBlockRig(t, "audit-plan-block-hitl-orch.md", approvals, "planner", "auditor", "merger")
}

const hitlWorkflow = "audit-plan-block-hitl"

// TestSession_HITLProducer_RejectedAttemptWithDefectiveSchedule_IsNotJudged
// verifies that a HITL-rejected attempt that wrote a defective schedule is only
// re-dispatched by the HITL loop: its schedule is not judged, so the
// re-dispatch carries no defect message, and the accepted corrected attempt
// lets the block run with no schedule-defect re-dispatch.
func TestSession_HITLProducer_RejectedAttemptWithDefectiveSchedule_IsNotJudged(t *testing.T) {
	approvals := &sequencedApprovalReader{answers: []domain.HumanApproval{sdDeny, sdApprove}}
	rig := newHITLDefectRig(t, approvals)
	rig.queueDefectivePlanner(1)
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config(hitlWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	planners := rig.invocationsFor("planner")
	if len(planners) != 2 {
		t.Fatalf("want 2 planner dispatches (rejected attempt, HITL re-dispatch), got %d", len(planners))
	}
	if strings.Contains(planners[1].Request.TaskDescription, sdDuplicateToken) {
		t.Errorf("want the HITL re-dispatch not to carry a schedule defect, got %q", planners[1].Request.TaskDescription)
	}
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
}

// TestSession_HITLProducer_AcceptedAttemptWithDefectiveSchedule_RedispatchPassesHITLAgain
// verifies that an accepted attempt with a defective schedule leads to a
// schedule-defect re-dispatch whose result is checked by the HITL compliance
// loop again, and that its corrected schedule lets the block run.
func TestSession_HITLProducer_AcceptedAttemptWithDefectiveSchedule_RedispatchPassesHITLAgain(t *testing.T) {
	approvals := &sequencedApprovalReader{answers: []domain.HumanApproval{sdApprove, sdApprove}}
	rig := newHITLDefectRig(t, approvals)
	rig.queueDefectivePlanner(1)
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config(hitlWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	planners := rig.invocationsFor("planner")
	if len(planners) != 2 {
		t.Fatalf("want 2 planner dispatches (attempt, schedule-defect re-dispatch), got %d", len(planners))
	}
	requireFixRequest(t, planners, 1, sdSourceFile, duplicateIDParts()...)
	if n := approvals.callCount(); n != 2 {
		t.Errorf("want the re-dispatch's result checked for approval too (2 checks), got %d", n)
	}
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
}

// TestSession_HITLProducer_RedispatchRejectedByHITL_IsRedispatchedByTheHITLLoop
// verifies that the schedule-defect re-dispatch of a HITL row is itself
// subject to the HITL loop's own re-dispatch: the rejected re-dispatch is
// dispatched once more by that loop, and the accepted corrected schedule runs.
func TestSession_HITLProducer_RedispatchRejectedByHITL_IsRedispatchedByTheHITLLoop(t *testing.T) {
	approvals := &sequencedApprovalReader{answers: []domain.HumanApproval{sdApprove, sdDeny, sdApprove}}
	rig := newHITLDefectRig(t, approvals)
	rig.queueDefectivePlanner(1)
	rig.queueProducer("planner", "fixed, not approved", runFileWrite(sdSourceFile, okSchedule("A")))
	rig.queueProducer("planner", "fixed, approved", runFileWrite(sdSourceFile, okSchedule("A")))
	rig.queueOK("auditor")
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config(hitlWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := rig.invocationsOf("planner"); n != 3 {
		t.Errorf("want 3 planner dispatches (attempt, rejected re-dispatch, HITL re-dispatch), got %d", n)
	}
	if n := approvals.callCount(); n != 3 {
		t.Errorf("want 3 approval checks, got %d", n)
	}
}

// TestSession_HITLProducer_NoHITLRejection_ConsultsAfterTwoRedispatches
// verifies that without any HITL rejection the consultation follows after
// exactly two schedule-defect re-dispatches (three attempts).
func TestSession_HITLProducer_NoHITLRejection_ConsultsAfterTwoRedispatches(t *testing.T) {
	approvals := &sequencedApprovalReader{answers: []domain.HumanApproval{sdApprove}}
	rig := newHITLDefectRig(t, approvals)
	rig.queueDefectivePlanner(3)
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(hitlWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 3 {
		t.Errorf("want 3 planner attempts, got %d", n)
	}
	if rig.consult.CallCount != 1 {
		t.Fatalf("want exactly 1 consultation, got %d", rig.consult.CallCount)
	}
	requireScheduleDefectConsult(t, rig.consult.Requests[0], duplicateIDParts()...)
}

// TestSession_HITLProducer_EveryAttemptRejectedOnce_ConsultsAfterOneRedispatch
// verifies that HITL-rejected attempts count: with each dispatch rejected once
// by the HITL loop, the accepted attempt is the second of the log, one
// schedule-defect re-dispatch follows, and its own rejected attempt and
// accepted re-dispatch bring the count to the limit, so the consultation
// follows after exactly 1 schedule-defect re-dispatch. The guard neither
// blocks nor escalates first.
func TestSession_HITLProducer_EveryAttemptRejectedOnce_ConsultsAfterOneRedispatch(t *testing.T) {
	approvals := &sequencedApprovalReader{answers: []domain.HumanApproval{sdDeny, sdApprove, sdDeny, sdApprove}}
	rig := newHITLDefectRig(t, approvals)
	rig.queueDefectivePlanner(4)
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(hitlWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	planners := rig.invocationsFor("planner")
	if len(planners) != 4 {
		t.Fatalf("want 4 planner dispatches (2 per HITL-gated dispatch, 1 schedule-defect re-dispatch), got %d", len(planners))
	}
	requireNames(t, "task description of the schedule-defect re-dispatch",
		planners[2].Request.TaskDescription, duplicateIDParts()...)
	if rig.consult.CallCount != 1 {
		t.Fatalf("want exactly 1 consultation, got %d", rig.consult.CallCount)
	}
	requireScheduleDefectConsult(t, rig.consult.Requests[0], duplicateIDParts()...)
}

// TestSession_HITLProducerConsulted_AcceptedDefectiveAttempt_RunnerRedispatchPassesHITLAgain
// verifies, for a consulted dispatch of a HITL producer, that the Runner's
// schedule-defect re-dispatch also passes the HITL compliance check, without
// consulting for the re-dispatch, and that the consultant's next block-row
// dispatch then proceeds.
func TestSession_HITLProducerConsulted_AcceptedDefectiveAttempt_RunnerRedispatchPassesHITLAgain(t *testing.T) {
	approvals := &sequencedApprovalReader{answers: []domain.HumanApproval{sdApprove, sdApprove}}
	rig := newHITLDefectRig(t, approvals)
	rig.queueDefectivePlanner(1)
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueOK("auditor")
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStagedDispatch("auditor", "audit item A", 1, "A")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config(hitlWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireSequence(t, "dispatch order", rig.dispatchOrder(), []string{"planner", "planner", "auditor"})
	if n := approvals.callCount(); n != 2 {
		t.Errorf("want the re-dispatch's result checked for approval too (2 checks), got %d", n)
	}
	if rig.consult.CallCount != 3 {
		t.Errorf("want 3 consultations (none for the re-dispatch), got %d", rig.consult.CallCount)
	}
	requireFixRequest(t, rig.invocationsFor("planner"), 1, sdSourceFile, duplicateIDParts()...)
}

// TestSession_HITLProducer_EscalatedAfterRejectedRedispatch_NoScheduleJudgement
// verifies that when the HITL compliance loop rejects the attempt and then its
// own single re-dispatch, the escalation that follows is the HITL one: the
// defective schedule is not judged for that attempt, so no schedule-defect fix
// message is dispatched and no consultation is of kind schedule-defect.
func TestSession_HITLProducer_EscalatedAfterRejectedRedispatch_NoScheduleJudgement(t *testing.T) {
	approvals := &sequencedApprovalReader{answers: []domain.HumanApproval{sdDeny, sdDeny}}
	rig := newHITLDefectRig(t, approvals)
	rig.queueDefectivePlanner(2)
	rig.consult.queueStop("escalated")

	got, err := rig.ses.Start(context.Background(), rig.config(hitlWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	planners := rig.invocationsFor("planner")
	if len(planners) != 2 {
		t.Fatalf("want 2 planner dispatches (attempt, HITL re-dispatch) and no schedule-defect re-dispatch, got %d", len(planners))
	}
	for i, p := range planners {
		if strings.Contains(p.Request.TaskDescription, sdDuplicateToken) {
			t.Errorf("want no schedule-defect message in planner dispatch %d, got %q", i+1, p.Request.TaskDescription)
		}
	}
	if len(rig.consult.Requests) == 0 {
		t.Fatalf("want the HITL escalation to be consulted, got no consultation")
	}
	for i, req := range rig.consult.Requests {
		if req.Deviation != nil && req.Deviation.Kind == domain.DeviationScheduleDefect {
			t.Errorf("want the escalation not to be of kind %q (consultation %d)", domain.DeviationScheduleDefect, i+1)
		}
	}
}
