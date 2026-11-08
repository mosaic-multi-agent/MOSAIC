package harness_test

// Tests for the whole-descendant kill of Run: when a timeout or a context
// cancellation ends an invocation, the started process and every descendant end
// with it, including a descendant whose intermediate parent already exited and
// which still holds the inherited stdout pipe. Run must return promptly instead
// of waiting for that pipe to close.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mosaic-common/harness"
)

func TestRun_Timeout_EndsGrandchildOfExitedParent(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 4 * time.Second})

	if !errors.Is(r.err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", r.err)
	}
	if r.resp.ExitCode != 0 || len(r.resp.Stdout) != 0 {
		t.Errorf("want a zero Response on timeout, got %+v", r.resp)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestRun_Cancellation_EndsGrandchildOfExitedParent(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// Cancel only once the grandchild exists, so the kill has a
		// descendant to find.
		deadline := time.Now().Add(pidFileWait)
		for time.Now().Before(deadline) {
			if _, ok := readPid(pidFile); ok {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()

	r := runBounded(t, ctx, harness.RunOptions{Timeout: 60 * time.Second})

	if !errors.Is(r.err, harness.ErrCancelled) {
		t.Fatalf("want ErrCancelled, got %v", r.err)
	}
	if errors.Is(r.err, harness.ErrTimeout) {
		t.Errorf("cancellation must not satisfy ErrTimeout, got %v", r.err)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestRun_TimeoutReturnsPromptlyWhileDescendantHoldsPipe(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	newPidFile(t)

	const timeout = 3 * time.Second
	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: timeout})

	if !errors.Is(r.err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", r.err)
	}
	if r.took > timeout+3*time.Second {
		t.Errorf("Run took %s for a %s timeout; it waited on the pipe held by a descendant", r.took, timeout)
	}
}

func TestRun_NormalExit_LeavesDetachedDescendantRunning(t *testing.T) {
	setHelperEnv(t, "tree-exit-leaves")
	pidFile := newPidFile(t)

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 10 * time.Second})

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if string(r.resp.Stdout) != "done" || r.resp.ExitCode != 0 {
		t.Errorf("want stdout %q and exit code 0, got %q and %d", "done", r.resp.Stdout, r.resp.ExitCode)
	}
	pid := waitForPid(t, pidFile)
	if !pidAlive(pid) {
		t.Errorf("descendant %d was ended by a normal exit; descendants must be left running", pid)
	}
}

func TestRun_SuccessfulRun_ReturnsFullOutputAndExitCode(t *testing.T) {
	setHelperEnv(t, "stream-three")

	resp, err := harness.Run(context.Background(), harness.Command{Path: helperExe(t)}, nil, harness.RunOptions{
		Timeout: 10 * time.Second,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Stdout) != "l1\nl2\nl3\n" {
		t.Errorf("want the full stdout, got %q", resp.Stdout)
	}
	if !strings.Contains(resp.Stderr, "warn") {
		t.Errorf("want the full stderr, got %q", resp.Stderr)
	}
	if resp.ExitCode != 0 {
		t.Errorf("want exit code 0, got %d", resp.ExitCode)
	}
}
