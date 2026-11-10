package session_test

// Tests that the refusal of a defective schedule at the start of a run applies
// to seeded schedules only: a produced Source that already exists with a
// defect (a stale file) is not refused, because the row that writes it runs
// first and replaces it.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// staleDefectiveSchedule repeats an ID in different case.
func staleDefectiveSchedule() string {
	return scheduleDoc("Stage", "Stages", ids("DUP-1", "B", "dup-1"))
}

// TestSession_Start_StaleDefectiveProducedSource_NotRefusedAndPlannerRewritesIt
// verifies that a new run whose produced AuditPlan.md already exists with
// duplicate IDs is not refused: the planner rewrites it and the block runs the
// iterations of the new schedule.
func TestSession_Start_StaleDefectiveProducedSource_NotRefusedAndPlannerRewritesIt(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", staleDefectiveSchedule())
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config("audit-plan-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
}

// TestSession_Start_Resume_StaleDefectiveProducedSource_NotRefusedAndPlannerRewritesIt
// is the same for a resume that has completed nothing yet.
func TestSession_Start_Resume_StaleDefectiveProducedSource_NotRefusedAndPlannerRewritesIt(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", staleDefectiveSchedule())
	rig.resumeFrom("audit-plan-block", domain.ExecutionModeAuto)
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig("audit-plan-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
}
