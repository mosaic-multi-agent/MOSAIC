package session_test

// A block schedule a producing row produced and that is defective is never
// silently used: in the auto modes the Runner re-dispatches the producing row
// with the non-compliant artifact among its inputs and a message naming the
// defect, the corrected schedule lets the block run, and a schedule still
// defective after two re-dispatches is handed to the routing consultant.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

// TestSession_ProducedScheduleDefective_RedispatchesProducerWithFixRequest
// verifies that after a planner writes a schedule with duplicate IDs the next
// dispatch is the planner again, carrying the defective artifact and a task
// description naming the rule, block, Source, Section and offending ID. A valid
// schedule is on disk at start, so the previous schedule must not be used to run
// the block against.
func TestSession_ProducedScheduleDefective_RedispatchesProducerWithFixRequest(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
			rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
			rig.queueDefectivePlanner(1)
			rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
			rig.queueOKTimes("auditor", 2)
			rig.queueOK("merger")

			_, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			order := rig.dispatchOrder()
			if len(order) < 2 || order[0] != "planner" || order[1] != "planner" {
				t.Fatalf("want the planner dispatched again right after the defective schedule, got %q", order)
			}
			requireFixRequest(t, rig.adapter.Invocations(), 1, sdSourceFile, duplicateIDParts()...)
		})
	}
}

// TestSession_ProducedScheduleCorrectedOnRedispatch_BlockRunsItsIterations
// verifies that a corrected schedule on re-dispatch lets the block run its
// iterations and the run complete.
func TestSession_ProducedScheduleCorrectedOnRedispatch_BlockRunsItsIterations(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
			rig.queueDefectivePlanner(1)
			rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
			rig.queueOKTimes("auditor", 2)
			rig.queueOK("merger")

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			requireSequence(t, "dispatch order", rig.dispatchOrder(),
				[]string{"planner", "planner", "auditor", "auditor", "merger"})
			requireSequence(t, "iterations the auditor ran at",
				rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
		})
	}
}

// TestSession_ProducedScheduleStillDefectiveAfterLimit_ConsultsWithDefect
// verifies that a schedule still defective after two re-dispatches (three
// producer attempts) leads to one consultation whose request carries the defect
// description in last_error_reason, the producer's status message in
// last_status_message and a deviation of kind schedule-defect, and that no
// block row ran.
func TestSession_ProducedScheduleStillDefectiveAfterLimit_ConsultsWithDefect(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
			rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
			rig.queueDefectivePlanner(3)
			rig.consult.queueStop("giving up")

			got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := rig.invocationsOf("planner"); n != 3 {
				t.Errorf("want 3 planner attempts (initial plus 2 re-dispatches), got %d", n)
			}
			if n := rig.invocationsOf("auditor"); n != 0 {
				t.Errorf("want no block row dispatched against the defective schedule, got %d", n)
			}
			if rig.consult.CallCount != 1 {
				t.Fatalf("want exactly 1 consultation, got %d", rig.consult.CallCount)
			}
			requireScheduleDefectConsult(t, rig.consult.Requests[0], duplicateIDParts()...)
		})
	}
}

// TestSession_ProducedScheduleDefective_ConsultationSeesNoScheduleForTheBlock
// verifies that while the block's schedule is defective the consultation
// request holds no schedule for it, although a valid one was read before.
func TestSession_ProducedScheduleDefective_ConsultationSeesNoScheduleForTheBlock(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
	rig.queueDefectivePlanner(3)
	rig.consult.queueStop("giving up")

	_, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if rig.consult.CallCount != 1 {
		t.Fatalf("want exactly 1 consultation, got %d", rig.consult.CallCount)
	}
	if _, held := rig.consult.Requests[0].Schedules[sdBlockName]; held {
		t.Errorf("want no schedule for block %s in the consultation request, got %+v",
			sdBlockName, rig.consult.Requests[0].Schedules)
	}
}

// TestSession_ProducedSourceMissing_RedispatchesProducerNamingTheMissingSource
// verifies that a producer that returns SUCCESS without writing the declared
// Source is treated as a defect of the producer: it is re-dispatched with a
// message naming the missing Source, and the corrected run completes.
func TestSession_ProducedSourceMissing_RedispatchesProducerNamingTheMissingSource(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueProducer("planner", "forgot to write the file")
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A")))
	rig.queueOK("auditor")
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	planners := rig.invocationsFor("planner")
	if len(planners) != 2 {
		t.Fatalf("want the planner dispatched twice, got %d", len(planners))
	}
	requireNames(t, "task description", planners[1].Request.TaskDescription,
		"missing", sdBlockName, sdSourceFile, sdSection)
}

// TestSession_ProducedSourceMissingAfterLimit_ConsultsWithTheMissingSource
// verifies that a Source still missing after two re-dispatches is consulted
// like any other defect.
func TestSession_ProducedSourceMissingAfterLimit_ConsultsWithTheMissingSource(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	for i := 0; i < 3; i++ {
		rig.queueProducer("planner", sdProducerMessage)
	}
	rig.consult.queueStop("giving up")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if rig.consult.CallCount != 1 {
		t.Fatalf("want exactly 1 consultation, got %d", rig.consult.CallCount)
	}
	requireScheduleDefectConsult(t, rig.consult.Requests[0], "missing", sdBlockName, sdSourceFile, sdSection)
}

// TestSession_ProducedScheduleWithUnsatisfiableDependsOn_RedispatchesProducer
// verifies that a Depends On entry naming no earlier row is handled like any
// other defect: the planner is re-dispatched with the rule and offending entry
// named, and a corrected schedule lets the block run.
func TestSession_ProducedScheduleWithUnsatisfiableDependsOn_RedispatchesProducer(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueProducer("planner", sdProducerMessage, runFileWrite(sdSourceFile, unsatisfiableDependsSchedule()))
	rig.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
	rig.queueOKTimes("auditor", 2)
	rig.queueOK("merger")

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	planners := rig.invocationsFor("planner")
	if len(planners) != 2 {
		t.Fatalf("want the planner dispatched twice, got %d", len(planners))
	}
	requireFixRequest(t, planners, 1, sdSourceFile, "V4", sdBlockName, sdSourceFile, sdSection, "Z")
	requireSequence(t, "iterations the auditor ran at",
		rig.dispatchedIterations("auditor", "out-", ".md"), []string{"A", "B"})
}

// TestSession_ProducedScheduleDefective_NoConsultant_EndsUnresolvedNamingTheKind
// verifies that with no routing consultant wired the re-dispatches still
// happen, and a schedule still defective after the limit ends the run as an
// unresolved deviation naming the kind schedule-defect, with no block row
// dispatched.
func TestSession_ProducedScheduleDefective_NoConsultant_EndsUnresolvedNamingTheKind(t *testing.T) {
	rig := newNoConsultantRig(t, "audit-plan-block-orch.md", "planner", "auditor", "merger")
	rig.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
	rig.queueDefectivePlanner(3)

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunDeviationUnresolved)
	requireNames(t, "outcome message", got.Message, string(domain.DeviationScheduleDefect))
	if n := rig.invocationsOf("planner"); n != 3 {
		t.Errorf("want 3 planner attempts without a consultant, got %d", n)
	}
	if n := rig.invocationsOf("auditor"); n != 0 {
		t.Errorf("want no block row dispatched against the defective schedule, got %d", n)
	}
}
