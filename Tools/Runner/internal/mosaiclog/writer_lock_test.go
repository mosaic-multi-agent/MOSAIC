package mosaiclog

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

// recordingDebug collects debug entries; safe for concurrent use.
type recordingDebug struct {
	mu     sync.Mutex
	events []string
}

func (r *recordingDebug) Log(event string, message string, fields ...domain.DebugField) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingDebug) count(event string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e == event {
			n++
		}
	}
	return n
}

// newLockTestWriter is newTestWriter with a recording debug logger.
func newLockTestWriter(t *testing.T) (*Writer, string, *recordingDebug) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dbg := &recordingDebug{}
	return NewWriter(root, "claude-code", WithClock(testClock{fixedNow}), WithDebugLogger(dbg)), root, dbg
}

// appendLockOf is the append lock file of path, as the log format defines it.
func appendLockOf(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".lock")
}

func holdAppendLock(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(appendLockOf(path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("take append lock: %v", err)
	}
	f.Close()
}

func TestRunStart_WaitsForHeldAppendLockThenWrites(t *testing.T) {
	w, root, _ := newLockTestWriter(t)
	path := orchestratorLogPath(root, testRunID)
	holdAppendLock(t, path)
	go func() {
		time.Sleep(600 * time.Millisecond)
		_ = os.Remove(appendLockOf(path))
	}()
	start := time.Now()

	w.RunStart(testRunID, "/work")

	if elapsed := time.Since(start); elapsed < 450*time.Millisecond {
		t.Errorf("RunStart returned after %v without waiting for the append lock", elapsed)
	}
	if events := readEvents(t, path); len(events) != 1 || events[0]["event"] != "run_start" {
		t.Errorf("events = %v, want one run_start", events)
	}
	if _, err := os.Stat(appendLockOf(path)); err == nil {
		t.Error("append lock file left behind")
	}
}

func TestRunEnd_DropsLineAndDebugLogsWhenLockTimesOut(t *testing.T) {
	w, root, dbg := newLockTestWriter(t)
	path := orchestratorLogPath(root, testRunID)
	holdAppendLock(t, path)
	start := time.Now()

	w.RunEnd(testRunID, OutcomeCompleted)

	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("RunEnd took %v, the lock wait must be bounded", elapsed)
	}
	if data, _ := os.ReadFile(path); len(data) != 0 {
		t.Errorf("a timed-out append wrote %q", data)
	}
	if n := dbg.count(debugWriteFailed); n != 1 {
		t.Errorf("debug entries %q = %d, want 1", debugWriteFailed, n)
	}
}

func TestEnsureInvocationEnd_ReportsFailedAndDropsLineWhenLockTimesOut(t *testing.T) {
	w, root, dbg := newLockTestWriter(t)
	makeInvocationFolder(t, root, "Research#3")
	path := invocationLogPath(root, testRunID, "Research#3")
	holdAppendLock(t, path)

	res := w.EnsureInvocationEnd(testRunID, InvocationEnd{AgentInstanceID: "Research#3", StatusCode: "SUCCESS"})

	if res != EnsureFailed {
		t.Errorf("result = %q, want %q", res, EnsureFailed)
	}
	if data, _ := os.ReadFile(path); len(data) != 0 {
		t.Errorf("a timed-out append wrote %q", data)
	}
	if n := dbg.count(debugWriteFailed); n != 1 {
		t.Errorf("debug entries %q = %d, want 1", debugWriteFailed, n)
	}
}

// Writes issued while another writer holds the append lock must all wait for it,
// then land whole once it is released. Without the lock they would be written at once.
func TestWriter_ParallelLongLinesWaitForHeldLockThenKeepEveryLineWhole(t *testing.T) {
	w, root, dbg := newLockTestWriter(t)
	path := orchestratorLogPath(root, testRunID)
	holdAppendLock(t, path)
	const goroutines, perGoroutine = 4, 3
	cwd := "/" + strings.Repeat("c", 150*1024)
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				w.RunStart(testRunID, cwd)
			}
		}()
	}
	time.Sleep(500 * time.Millisecond)
	if data, _ := os.ReadFile(path); len(data) != 0 {
		t.Errorf("%d bytes written while the append lock was held by someone else", len(data))
	}
	if err := os.Remove(appendLockOf(path)); err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	events := readEvents(t, path)
	if len(events) != goroutines*perGoroutine {
		t.Fatalf("got %d events, want %d (debug failures %d)", len(events), goroutines*perGoroutine, dbg.count(debugWriteFailed))
	}
	for i, ev := range events {
		if ev["event"] != "run_start" || ev["cwd"] != cwd {
			t.Fatalf("event %d is torn or altered", i)
		}
	}
}
