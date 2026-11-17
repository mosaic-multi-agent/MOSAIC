package opencodev2_test

// Tests that ending an invocation (timeout or force stop through context
// cancellation) ends the whole process tree, so a V2 run's standalone session
// does not outlive the dispatch. The "tree-agent" scenario starts a long-lived
// grandchild and publishes its pid.

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"
)

const (
	pidFileWait = 10 * time.Second
	pidGoneWait = 8 * time.Second
)

// newPidFile points the helper's grandchild pid publication at a fresh file;
// any process recorded there is killed when the test ends.
func newPidFile(t *testing.T) string {
	t.Helper()
	path := captureFile(t, "GO_HELPER_PID_FILE", "grandchild.pid")
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
	return pid, err == nil && pid > 0
}

func waitForPid(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(pidFileWait)
	for time.Now().Before(deadline) {
		if pid, ok := readPid(path); ok {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the grandchild pid was not published within %s", pidFileWait)
	return 0
}

func requireGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(pidGoneWait)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("grandchild process %d is still running after the invocation ended", pid)
}

func TestInvoke_Timeout_EndsTheWholeProcessTree(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)
	adapter := newAdapter(t, 3*time.Second)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if !errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestInvokeRaw_Timeout_EndsTheWholeProcessTree(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)
	adapter := newAdapter(t, 3*time.Second)

	_, err := adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload)

	if !errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestInvoke_ForceStop_EndsTheWholeProcessTree(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		deadline := time.Now().Add(pidFileWait)
		for time.Now().Before(deadline) {
			if _, ok := readPid(pidFile); ok {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()

	_, err := newAdapter(t, longTimeout).Invoke(ctx, ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	requireGone(t, waitForPid(t, pidFile))
}
