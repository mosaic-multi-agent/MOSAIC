package session_test

// Tests that a resume reads each block's schedule from the Source the
// workflow declares for it, and that a produced Source that does not exist yet
// is not a refusal: the producing row runs first.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_Start_Resume_ScheduleFromProducedSource_DispatchesItsIterations
// verifies that a resume after the planner row reads the schedule from
// AuditPlan.md although no run-root Plan.md exists, and runs the block at the
// iterations of that schedule.
func TestSession_Start_Resume_ScheduleFromProducedSource_DispatchesItsIterations(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, "AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B", "C")))
	rig.resumeAfterFirstRow("audit-plan-block", domain.ExecutionModeAuto, "planner", "PLANNING")
	rig.queueOKTimes("auditor", 3)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig("audit-plan-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B", "C"})
	if n := rig.invocationsOf("planner"); n != 0 {
		t.Errorf("want the completed planner row not dispatched again, got %d dispatches", n)
	}
}

// TestSession_Start_Resume_ScheduleFromNonPlanArtifact_DispatchesItsIterations
// is the same resume for a Source whose name does not end in Plan.md, with the
// schedule under its own section.
func TestSession_Start_Resume_ScheduleFromNonPlanArtifact_DispatchesItsIterations(t *testing.T) {
	rig := newBlockRig(t, "kb-progress-block-orch.md", nil, "bootstrapper", "writer", "indexer")
	rig.writeRunFile(t, "KBProgress.md", scheduleDoc("Generation", "Generation", ids("intro", "design")))
	rig.resumeAfterFirstRow("kb-progress-block", domain.ExecutionModeAuto, "bootstrapper", "PLANNING")
	rig.queueOKTimes("writer", 2)
	rig.queueOK("indexer")

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig("kb-progress-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the writer ran at",
		rig.dispatchedIterations("writer", "docs/", ".md"), []string{"intro", "design"})
}

// TestSession_Start_Resume_ProducedSourceNotYetExisting_RunsProducerFirst
// verifies that a resume of a run that has not reached the producing row is
// not refused for the missing Source: the planner is dispatched first and its
// schedule drives the block.
func TestSession_Start_Resume_ProducedSourceNotYetExisting_RunsProducerFirst(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.resumeFrom("audit-plan-block", domain.ExecutionModeAuto)
	rig.queueOK("planner", runFileWrite("AuditPlan.md", scheduleDoc("Stage", "Stages", ids("A", "B"))))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig("audit-plan-block", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := rig.adapter.Invocations()
	if len(invs) == 0 || invs[0].Agent.Identifier != "planner" {
		t.Errorf("want the planner as first dispatch, got %v", invs)
	}
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
}
