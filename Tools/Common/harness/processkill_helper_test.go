package harness_test

// Process-tree modes of the fake-subprocess helper (see helper_test.go) and the
// shared pieces of the whole-descendant kill tests.
//
// Modes:
//   - tree-agent: the process Run starts. It starts tree-intermediate with the
//     inherited stdout/stderr, waits for it to exit, then blocks.
//   - tree-intermediate: starts tree-grandchild (inheriting stdout/stderr, so
//     the grandchild holds the pipe Run reads), records the grandchild PID in
//     the file named by GO_HELPER_PID_FILE and exits at once. The grandchild's
//     parent is therefore gone, which a walk over the live parent chain misses.
//   - tree-grandchild: sleeps for a bounded lifetime so a failing test leaves
//     no orphan behind for long.
//   - tree-exit-leaves: starts a grandchild detached from the pipes, records
//     its PID, writes "done" and exits zero.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mosaic-common/harness"
)

const (
	grandchildLifetime = 20 * time.Second
	pidFileWait        = 10 * time.Second
	pidGoneWait        = 5 * time.Second
	returnPromptly     = 8 * time.Second
)

// runTreeHelper handles the process-tree helper modes. It returns normally for
// modes that exit zero.
func runTreeHelper(cmd string) {
	switch cmd {
	case "tree-agent":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "GO_HELPER_CMD=tree-intermediate")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Run(); err != nil {
			os.Stderr.WriteString("intermediate failed: " + err.Error()) //nolint:errcheck
			os.Exit(3)
		}
		time.Sleep(24 * time.Hour)

	case "tree-intermediate":
		gc := exec.Command(os.Args[0])
		gc.Env = append(os.Environ(), "GO_HELPER_CMD=tree-grandchild")
		gc.Stdout = os.Stdout
		gc.Stderr = os.Stderr
		if err := gc.Start(); err != nil {
			os.Exit(3)
		}
		writePidFile(gc.Process.Pid)

	case "tree-grandchild":
		time.Sleep(grandchildLifetime)

	case "tree-exit-leaves":
		gc := exec.Command(os.Args[0])
		gc.Env = append(os.Environ(), "GO_HELPER_CMD=tree-grandchild")
		if err := gc.Start(); err != nil {
			os.Exit(3)
		}
		writePidFile(gc.Process.Pid)
		writeOut("done")

	default:
		os.Stderr.WriteString("unknown GO_HELPER_CMD: " + cmd) //nolint:errcheck
		os.Exit(2)
	}
}

// writePidFile publishes pid atomically to GO_HELPER_PID_FILE.
func writePidFile(pid int) {
	path := os.Getenv("GO_HELPER_PID_FILE")
	if path == "" {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return
	}
	os.Rename(tmp, path) //nolint:errcheck
}

// newPidFile configures the PID file for helper processes spawned by the
// current test and returns its path. Any process recorded in it is killed when
// the test ends, so a failing test leaves no orphan.
func newPidFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "grandchild.pid")
	t.Setenv("GO_HELPER_PID_FILE", path)
	t.Cleanup(func() {
		if pid, ok := readPid(path); ok {
			if p, err := os.FindProcess(pid); err == nil {
				p.Kill() //nolint:errcheck
			}
		}
	})
	return path
}

func readPid(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// waitForPid polls until the helper published a PID, failing after pidFileWait.
func waitForPid(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(pidFileWait)
	for time.Now().Before(deadline) {
		if pid, ok := readPid(path); ok {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("grandchild PID was not published within %s", pidFileWait)
	return 0
}

// requireGone fails unless the process with pid ends within pidGoneWait.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(pidGoneWait)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("grandchild process %d is still running after the kill", pid)
}

type runResult struct {
	resp harness.Response
	err  error
	took time.Duration
}

// runBounded calls harness.Run in a goroutine and fails the test when it has
// not returned within returnPromptly, so a Run blocked on a pipe held by a
// descendant is reported instead of hanging until the descendant exits.
func runBounded(t *testing.T, ctx context.Context, opts harness.RunOptions) runResult {
	t.Helper()
	ch := make(chan runResult, 1)
	go func() {
		start := time.Now()
		resp, err := harness.Run(ctx, harness.Command{Path: helperExe(t)}, nil, opts)
		ch <- runResult{resp, err, time.Since(start)}
	}()
	select {
	case r := <-ch:
		return r
	case <-time.After(returnPromptly):
		t.Fatalf("Run did not return within %s of its deadline: blocked on a pipe held by a descendant", returnPromptly)
		return runResult{}
	}
}
