package session_test

// The schedule-defect re-dispatches are mechanical: the anti-loop guard does
// not count them, so the dedicated limit always acts first. The consultant
// still gets its own dispatches of the producer counted as usual, and the guard
// must neither block nor escalate across them.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// requireOnlyScheduleDefectConsultations asserts that every consultation from
// index first on is a schedule-defect consultation: the guard escalated none.
func requireOnlyScheduleDefectConsultations(t *testing.T, reqs []domain.ConsultationRequest, first int) {
	t.Helper()
	for i := first; i < len(reqs); i++ {
		if reqs[i].Deviation == nil || reqs[i].Deviation.Kind != domain.DeviationScheduleDefect {
			t.Errorf("consultation %d: want kind %q (no guard escalation), got %+v",
				i+1, domain.DeviationScheduleDefect, reqs[i].Deviation)
		}
	}
}

// TestSession_Auto_ScheduleDefectRedispatches_AreNotCountedByTheGuard verifies
// that in an auto mode the two re-dispatches and three further dispatches the
// consultant requests (each defective again) all run: counted, the
// re-dispatches would make the guard block the consultant's third request.
func TestSession_Auto_ScheduleDefectRedispatches_AreNotCountedByTheGuard(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(6)
	for i := 0; i < 3; i++ {
		rig.consult.queueDispatch("planner", "write the audit plan again", 0)
	}
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 6 {
		t.Errorf("want 6 planner dispatches (first, 2 re-dispatches, 3 consulted), got %d", n)
	}
	if rig.consult.CallCount != 4 {
		t.Fatalf("want 4 consultations, got %d", rig.consult.CallCount)
	}
	requireOnlyScheduleDefectConsultations(t, rig.consult.Requests, 0)
}

// TestSession_Consulted_ScheduleDefectRedispatches_AreNotCountedByTheGuard is
// the same in orchestrated mode, where the first dispatch is itself consulted:
// four consulted dispatches of the planner and two re-dispatches all run.
func TestSession_Consulted_ScheduleDefectRedispatches_AreNotCountedByTheGuard(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(6)
	for i := 0; i < 4; i++ {
		rig.consult.queueDispatch("planner", "write the audit plan", 0)
	}
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := rig.invocationsOf("planner"); n != 6 {
		t.Errorf("want 6 planner dispatches (4 consulted, 2 re-dispatches), got %d", n)
	}
	if rig.consult.CallCount != 5 {
		t.Fatalf("want 5 consultations, got %d", rig.consult.CallCount)
	}
	requireOnlyScheduleDefectConsultations(t, rig.consult.Requests, 1)
}
