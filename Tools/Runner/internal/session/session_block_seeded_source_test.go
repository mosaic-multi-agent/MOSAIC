package session_test

// Tests of a seeded schedule whose Source is not named Plan.md: it is read and
// run, a defective one is refused naming its own block and source, and one
// that does not exist at the start of a new run is refused with the run folder
// removed.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"mosaic-run/internal/domain"
)

const seededBacklogWorkflow = "seeded-backlog-first-block"

// backlogDefect is a Backlog.md whose schedule repeats an ID.
var backlogDefect = seededDefect{
	name: "duplicate IDs", block: "Item", source: "Backlog.md",
	plan: `# Backlog

## Backlog

| Item | Name | Goal | HITL |
|------|------|------|:----:|
| DUP-1 | One | First | FALSE |
| B | Two | Second | FALSE |
| dup-1 | Three | Repeats the first ID in other case | FALSE |
`,
	rule: "V2", row: 3, token: "dup-1",
}

// TestSession_Start_SeededBacklogWithFirstRowABlockRow_DispatchesEveryIteration
// verifies that a seeded Source named Backlog.md, section Backlog, is read at
// run start and the block runs its iterations. No file named Plan.md exists, so
// a session that reads a fixed run-root Plan.md has no schedule.
func TestSession_Start_SeededBacklogWithFirstRowABlockRow_DispatchesEveryIteration(t *testing.T) {
	rig := newBlockRig(t, seededBacklogWorkflow+"-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Backlog.md", scheduleDoc("Item", "Backlog", ids("x", "y", "z")))
	rig.queueOKTimes("worker", 3)
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), rig.config(seededBacklogWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "done-", ".md"), []string{"x", "y", "z"})
}

// TestSession_Start_DefectiveSeededBacklog_RefusedBeforeAnyDispatch verifies
// the refusal names the block and Source declared for the schedule.
func TestSession_Start_DefectiveSeededBacklog_RefusedBeforeAnyDispatch(t *testing.T) {
	rig := newBlockRig(t, seededBacklogWorkflow+"-orch.md", nil, "worker", "closer")
	rig.writeRunFile(t, "Backlog.md", backlogDefect.plan)

	got, err := rig.ses.Start(context.Background(), rig.config(seededBacklogWorkflow, domain.ExecutionModeAuto))

	requireSeededRefusal(t, rig, got, err, seededBacklogWorkflow, backlogDefect)
}

// TestSession_Start_DefectiveSeededBacklog_RefusedInEveryMode verifies that
// the refusal of a Source not named Plan.md applies in auto-review and
// orchestrated modes too (auto is covered above).
func TestSession_Start_DefectiveSeededBacklog_RefusedInEveryMode(t *testing.T) {
	for _, mode := range []domain.ExecutionMode{domain.ExecutionModeAutoReview, domain.ExecutionModeOrchestrated} {
		t.Run(string(mode), func(t *testing.T) {
			rig := newBlockRig(t, seededBacklogWorkflow+"-orch.md", nil, "worker", "closer")
			rig.writeRunFile(t, "Backlog.md", backlogDefect.plan)

			got, err := rig.ses.Start(context.Background(), rig.config(seededBacklogWorkflow, mode))

			requireSeededRefusal(t, rig, got, err, seededBacklogWorkflow, backlogDefect)
		})
	}
}

// TestSession_Start_MissingSeededSourceAtNewRun_RefusedAndFolderRemoved
// verifies that a seeded Source that does not exist when a new run starts is
// refused before any dispatch, and the new run's folder does not stay behind.
func TestSession_Start_MissingSeededSourceAtNewRun_RefusedAndFolderRemoved(t *testing.T) {
	rig := newBlockRig(t, seededBacklogWorkflow+"-orch.md", nil, "worker", "closer")

	got, err := rig.ses.Start(context.Background(), rig.config(seededBacklogWorkflow, domain.ExecutionModeAuto))

	requireRefused(t, got, err)
	var defect *domain.ScheduleDefectError
	if !errors.As(got.Cause, &defect) {
		t.Fatalf("want the refusal to wrap a *domain.ScheduleDefectError, got %v", got.Cause)
	}
	if !errors.Is(defect, fs.ErrNotExist) {
		t.Errorf("want the defect to report a missing source (fs.ErrNotExist), got cause %v", defect.Cause)
	}
	if defect.Source != "Backlog.md" || string(defect.Block) != "Item" {
		t.Errorf("defect: want block Item and source Backlog.md, got block %q source %q", defect.Block, defect.Source)
	}
	if n := len(rig.adapter.Invocations()); n != 0 {
		t.Errorf("want no dispatch before the refusal, got %d", n)
	}
	if _, statErr := os.Stat(rig.runFolder); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("want the refused new run's folder removed, stat error: %v", statErr)
	}
}
