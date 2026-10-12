package session_test

// Tests that the seed files of a new run (RunConfig.SeedInputs) are copied into
// the run folder before the declared block schedules are read: a schedule that
// exists only outside the run folder at the start runs, and a defective one is
// refused with the folder removed.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
)

// seedInputFiles writes backlog as Backlog.md, plus the Requirements file a
// seed set must hold, into a directory outside every run folder and returns
// the two paths in the form RunConfig.SeedInputs takes.
func seedInputFiles(t *testing.T, backlog string) []string {
	t.Helper()
	dir := t.TempDir()
	files := []struct{ name, content string }{
		{"Backlog.md", backlog},
		{"Requirements.md", "# Requirements\n"},
	}
	var paths []string
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, []byte(f.content), 0o600); err != nil {
			t.Fatalf("seedInputFiles: write %s: %v", f.name, err)
		}
		paths = append(paths, p)
	}
	return paths
}

// TestSession_Start_SeedInputsHoldingSchedule_BlockRunsItsIterations verifies
// that a schedule supplied only as a seed input, with nothing in the run
// folder beforehand, is read after the seed copy and its iterations run.
func TestSession_Start_SeedInputsHoldingSchedule_BlockRunsItsIterations(t *testing.T) {
	rig := newBlockRig(t, seededBacklogWorkflow+"-orch.md", nil, "worker", "closer")
	cfg := rig.config(seededBacklogWorkflow, domain.ExecutionModeAuto)
	cfg.SeedInputs = seedInputFiles(t, scheduleDoc("Item", "Backlog", ids("x", "y", "z")))
	rig.queueOKTimes("worker", 3)
	rig.queueOK("closer")

	got, err := rig.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireSequence(t, "iterations the worker ran at",
		rig.dispatchedIterations("worker", "done-", ".md"), []string{"x", "y", "z"})
}

// TestSession_Start_SeedInputsHoldingDefectiveSchedule_RefusedAndFolderRemoved
// verifies that a defective schedule that arrives through a seed input is
// refused before any dispatch, naming the rule, token, block and source, and
// the refused new run's folder is removed.
func TestSession_Start_SeedInputsHoldingDefectiveSchedule_RefusedAndFolderRemoved(t *testing.T) {
	rig := newBlockRig(t, seededBacklogWorkflow+"-orch.md", nil, "worker", "closer")
	cfg := rig.config(seededBacklogWorkflow, domain.ExecutionModeAuto)
	cfg.SeedInputs = seedInputFiles(t, backlogDefect.plan)

	got, err := rig.ses.Start(context.Background(), cfg)

	requireSeededRefusal(t, rig, got, err, seededBacklogWorkflow, backlogDefect)
	if _, statErr := os.Stat(rig.runFolder); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("want the refused new run's folder removed, stat error: %v", statErr)
	}
}
