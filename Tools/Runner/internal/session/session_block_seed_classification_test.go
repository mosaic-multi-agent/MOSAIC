package session_test

// Tests that the seeded or produced classification of a block's Source and its
// producing rows come from the admitted workflow and not from the session
// matching row outputs again: a seeded schedule is refused whether or not a
// row of its block also writes it, and a row outside the block that outputs a
// seeded Source triggers no re-read.

import (
	"context"
	"errors"
	"testing"

	"mosaic-run/internal/domain"
)

const (
	writerSeededWorkflow = "writer-seeded"
	seedPrecedenceFlow   = "seed-precedence"
)

// TestSession_Start_DefectiveSeededSourceWithInBlockWriter_RefusedBeforeAnyDispatch
// verifies that a seeded schedule is refused at the start of a new run even
// though a row of its own block also outputs the Source. A session that took a
// row outputting the Source to mean a produced schedule would not refuse.
func TestSession_Start_DefectiveSeededSourceWithInBlockWriter_RefusedBeforeAnyDispatch(t *testing.T) {
	rig := newBlockRig(t, writerSeededWorkflow+"-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Plan.md", seededDefects[0].plan)

	got, err := rig.ses.Start(context.Background(), rig.config(writerSeededWorkflow, domain.ExecutionModeAuto))

	requireSeededRefusal(t, rig, got, err, writerSeededWorkflow, seededDefects[0])
}

// TestSession_Start_Resume_DefectiveSeededSourceWithInBlockWriter_RefusedBeforeAnyDispatch
// is the same refusal at resume, naming the run as the resource, with the
// existing run folder and its Plan.md left in place.
func TestSession_Start_Resume_DefectiveSeededSourceWithInBlockWriter_RefusedBeforeAnyDispatch(t *testing.T) {
	rig := newBlockRig(t, writerSeededWorkflow+"-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Plan.md", seededDefects[0].plan)
	rig.resumeFrom(writerSeededWorkflow, domain.ExecutionModeAuto)

	got, err := rig.ses.Start(context.Background(), rig.resumeConfig(writerSeededWorkflow, domain.ExecutionModeAuto))

	d := seededDefects[0]
	requireRefused(t, got, err)
	var defect *domain.ScheduleDefectError
	if !errors.As(got.Cause, &defect) {
		t.Fatalf("want the refusal to wrap a *domain.ScheduleDefectError, got %v", got.Cause)
	}
	if defect.Rule != d.rule || defect.Token != d.token || string(defect.Block) != d.block {
		t.Errorf("defect: want rule %q token %q block %q, got rule %q token %q block %q",
			d.rule, d.token, d.block, defect.Rule, defect.Token, defect.Block)
	}
	var refusal *domain.RefusalError
	if errors.As(got.Cause, &refusal) && refusal.Resource != testRunID {
		t.Errorf("RefusalError.Resource: want the run %q, got %q", testRunID, refusal.Resource)
	}
	if n := len(rig.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch before the refusal, got %d", n)
	}
	requireRunFolderSurvives(t, rig, "Plan.md")
}

// TestSession_Start_SeededSourceAlsoOutputBeforeBlock_RunsTheSeededSchedule
// verifies that a row before the block that outputs a Source which is also a
// seed does not count as a producer: its completion triggers no re-read, so the
// block runs the schedule held at the start and not the one that row wrote.
func TestSession_Start_SeededSourceAlsoOutputBeforeBlock_RunsTheSeededSchedule(t *testing.T) {
	rig := newBlockRig(t, seedPrecedenceFlow+"-orch.md", nil, "planner", "worker", "closer")
	rig.writeRunFile(t, "Backlog.md", scheduleDoc("Item", "Backlog", ids("x", "y")))
	rig.queueOK("planner", runFileWrite("Backlog.md", scheduleDoc("Item", "Backlog", ids("p", "q", "r"))))
	rig.queueOKTimes("worker", 2)
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), rig.config(seedPrecedenceFlow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "done-", ".md"), []string{"x", "y"})
}
