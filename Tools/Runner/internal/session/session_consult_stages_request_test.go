package session_test

// Tests that the session hands the routing consultant the schedule currently
// in force with every consultation, including a schedule that was derived or
// refreshed mid-run. Without it a consultant cannot validate a staged dispatch.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_ConsultationRequest_CarriesCurrentSchedule verifies that the
// request given to the routing consultant holds the run's schedule.
func TestSession_ConsultationRequest_CarriesCurrentSchedule(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("nothing to do")
	ses, _, store, orchPath, runFolder := newConsultStagedSession(t, consultant)
	store.state = consultStagedStage2State()
	store.exists = true

	_, err := ses.Start(context.Background(), baseConsultStagedConfig(orchPath, runFolder))
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(consultant.Requests) == 0 {
		t.Fatal("want at least one consultation request, got none")
	}
	got, ok := consultant.Requests[0].Schedules["Stage"]
	if !ok {
		t.Fatalf("want ConsultationRequest.Schedules populated from the run's Plan.md, got %v", consultant.Requests[0].Schedules)
	}
	if len(got.Iterations) != 2 {
		t.Errorf("want 2 iterations in the consultation request, got %d", len(got.Iterations))
	}
}

// TestSession_ConsultationRequest_CarriesScheduleDerivedMidRun verifies that a
// schedule that appears during the run (a planner row writing the block's
// declared Source) reaches every later consultation, and that the consultation
// before the Source existed carried none.
func TestSession_ConsultationRequest_CarriesScheduleDerivedMidRun(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueOK("planner", runFileWrite(sdSourceFile, scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOK("auditor")
	rig.consult.queueDispatch("planner", "write the audit plan", 0)
	rig.consult.queueStagedDispatch("auditor", "audit item A", 1, "A")
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(rig.consult.Requests) != 3 {
		t.Fatalf("want 3 consultation requests, got %d", len(rig.consult.Requests))
	}
	if first, held := rig.consult.Requests[0].Schedules["Stage"]; held {
		t.Errorf("request 0: want no schedule before the planner wrote the Source, got %+v", first)
	}
	for i := 1; i < 3; i++ {
		got, ok := rig.consult.Requests[i].Schedules["Stage"]
		if !ok || len(got.Iterations) != 2 {
			t.Errorf("request %d: want the 2-iteration schedule the planner wrote, got %+v", i, got)
		}
	}
}
