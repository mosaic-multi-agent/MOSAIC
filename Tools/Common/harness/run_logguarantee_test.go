package harness_test

// Tests for the remaining Run logging guarantees: Sink.Log is never entered
// concurrently, very long lines reach the sink intact, and a launch failure
// emits no events.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mosaic-common/harness"
)

// serialSink records every event and notes whether Log was ever entered while
// another call was still running.
type serialSink struct {
	inside     atomic.Int32
	overlapped atomic.Bool
	mu         sync.Mutex
	events     []harness.Event
}

func (s *serialSink) Log(ev harness.Event) {
	if s.inside.Add(1) > 1 {
		s.overlapped.Store(true)
	}
	time.Sleep(time.Millisecond) // widen the window a concurrent call would hit
	s.mu.Lock()
	s.events = append(s.events, ev)
	s.mu.Unlock()
	s.inside.Add(-1)
}

func (s *serialSink) snapshot() []harness.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]harness.Event(nil), s.events...)
}

func TestRun_LogGuarantee_SinkLogIsNeverCalledConcurrentlyOnNormalExit(t *testing.T) {
	setHelperEnv(t, "stream-many")
	t.Setenv("GO_HELPER_LINE_COUNT", "500")
	t.Setenv("GO_HELPER_LINE_PAD", "10")
	sink := &serialSink{}

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 30 * time.Second, Sink: sink})

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if sink.overlapped.Load() {
		t.Errorf("Sink.Log was entered concurrently with itself")
	}
	if got := liveLines(t, sink.snapshot()); len(got) != 500 {
		t.Errorf("want 500 logged lines, got %d", len(got))
	}
}

func TestRun_LogGuarantee_SinkLogIsNeverCalledConcurrentlyOnKill(t *testing.T) {
	setHelperEnv(t, "ll-kill-partial")
	sink := &serialSink{}

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 2 * time.Second, Sink: sink})

	if !errors.Is(r.err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", r.err)
	}
	if sink.overlapped.Load() {
		t.Errorf("Sink.Log was entered concurrently with itself")
	}
	if got := lineTexts(liveLines(t, sink.snapshot())); len(got) != 3 {
		t.Errorf("want the 3 pre-kill lines logged, got %q", got)
	}
}

func TestRun_LogGuarantee_MultiMegabyteLineReachesSinkIntact(t *testing.T) {
	const size = 3 << 20
	setHelperEnv(t, "stream-long")
	t.Setenv("GO_HELPER_LONG_BYTES", strconv.Itoa(size))
	sink := &recordingSink{}

	r := runBounded(t, context.Background(), harness.RunOptions{Timeout: 30 * time.Second, Sink: sink})

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	got := lineTexts(liveLines(t, sink.snapshot()))
	if len(got) != 2 || got[1] != "tail" {
		t.Fatalf("want the long line and the tail, got %d lines", len(got))
	}
	if len(got[0]) != size || strings.Trim(got[0], "x") != "" {
		t.Errorf("want one intact %d-byte line, got length %d", size, len(got[0]))
	}
}

func TestRun_LogGuarantee_AbsentExecutableEmitsNoEvents(t *testing.T) {
	sink := &recordingSink{}

	_, err := harness.Run(context.Background(), harness.Command{Path: "/nonexistent/path/to/claude"}, nil, harness.RunOptions{
		Timeout: 5 * time.Second, Sink: sink,
	})

	if !errors.Is(err, harness.ErrExecutableNotFound) {
		t.Fatalf("want ErrExecutableNotFound, got %v", err)
	}
	if events := sink.snapshot(); len(events) != 0 {
		t.Errorf("want no events when the process never started, got %v", eventNames(events))
	}
}
