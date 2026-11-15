package harness_test

// Tests for the bounded wait of Run: when the started process exits while a
// descendant still holds its stdout/stderr open, Run stops waiting no later than
// RunOutputGrace after the exit, returns the output received so far, and ends the
// remaining process tree.

import (
	"context"
	"errors"
	"testing"
	"time"

	"mosaic-common/harness"
)

// heldOutputBudget is how long after process exit Run may take: the grace plus
// slack for process start-up and scheduling.
const heldOutputBudget = harness.RunOutputGrace + 4*time.Second

func TestRun_ProcessExitsWhileDescendantHoldsOutput_ReturnsWithinGrace(t *testing.T) {
	setHelperEnv(t, "tree-exit-holds")
	newPidFile(t)

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 60 * time.Second})

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.took > heldOutputBudget {
		t.Errorf("Run took %s after the process exited; want within %s", r.took, heldOutputBudget)
	}
}

func TestRun_ProcessExitsWhileDescendantHoldsOutput_ReturnsOutputReceived(t *testing.T) {
	setHelperEnv(t, "tree-exit-holds")
	newPidFile(t)

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 60 * time.Second})

	if string(r.resp.Stdout) != "done" || r.resp.ExitCode != 0 {
		t.Errorf("want stdout %q and exit code 0, got %q and %d", "done", r.resp.Stdout, r.resp.ExitCode)
	}
}

func TestRun_ProcessExitsWhileDescendantHoldsOutput_EndsTheRemainingProcessTree(t *testing.T) {
	setHelperEnv(t, "tree-exit-holds")
	pidFile := newPidFile(t)

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 60 * time.Second})

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestRun_NonZeroExitWhileDescendantHoldsOutput_ReportsExitWithinGrace(t *testing.T) {
	setHelperEnv(t, "tree-exit1-holds")
	newPidFile(t)

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 60 * time.Second})

	if !errors.Is(r.err, harness.ErrNonZeroExit) {
		t.Fatalf("want ErrNonZeroExit, got %v", r.err)
	}
	if r.resp.ExitCode != 1 || string(r.resp.Stdout) != "done" {
		t.Errorf("want exit code 1 and stdout %q, got %d and %q", "done", r.resp.ExitCode, r.resp.Stdout)
	}
	if r.took > heldOutputBudget {
		t.Errorf("Run took %s after the process exited; want within %s", r.took, heldOutputBudget)
	}
}

func TestRun_ProcessExitsWhileDescendantHoldsOutput_LineObserverStillReceivesOutput(t *testing.T) {
	setHelperEnv(t, "tree-exit-holds")
	newPidFile(t)
	lines := make(chan string, 8)

	r := runBounded(t, context.Background(), harness.RunOptions{
		Timeout:      60 * time.Second,
		OnStdoutLine: func(line []byte) { lines <- string(line) },
	})

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	select {
	case got := <-lines:
		if got != "done" {
			t.Errorf("observer line = %q, want %q", got, "done")
		}
	default:
		t.Errorf("the line observer received nothing; want the output line")
	}
}
