package harness_test

// Helper modes and sink fixtures for the live and kill-time stdout logging
// tests (see helper_test.go for the dispatch).
//
//   - ll-kill-partial: stderr "err-before-kill\n", then (after a short pause so
//     stderr is read first) one write "one\ntwo\npart" with no trailing newline,
//     then silence for a long time.
//   - ll-lines-then-hang: publishes its pid to GO_HELPER_PID_FILE (when set),
//     writes "q1\nq2\nq3\n", then stays silent for a long time.

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mosaic-common/harness"
)

const killPartialStdout = "one\ntwo\npart"

// runLiveLogHelper handles the live-logging helper modes.
func runLiveLogHelper(cmd string) {
	switch cmd {
	case "ll-kill-partial":
		os.Stderr.WriteString("err-before-kill\n") //nolint:errcheck
		time.Sleep(300 * time.Millisecond)
		writeOut(killPartialStdout)
		time.Sleep(24 * time.Hour)
	case "ll-lines-then-hang":
		writePidFile(os.Getpid()) // lets a test observe that the kill happened
		writeOut("q1\nq2\nq3\n")
		time.Sleep(24 * time.Hour)
	default:
		os.Stderr.WriteString("unknown live-log helper mode " + cmd + "\n") //nolint:errcheck
		os.Exit(2)
	}
}

// hookSink records every event and calls onLog (when set) after recording, so a
// test can react to what the sink has received while the process still runs.
type hookSink struct {
	mu     sync.Mutex
	events []harness.Event
	onLog  func(harness.Event)
}

func (s *hookSink) Log(ev harness.Event) {
	s.mu.Lock()
	s.events = append(s.events, ev)
	cb := s.onLog
	s.mu.Unlock()
	if cb != nil {
		cb(ev)
	}
}

func (s *hookSink) snapshot() []harness.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]harness.Event(nil), s.events...)
}

// slowSink blocks every Log call until release is closed, simulating a sink
// that lags far behind the process. Events are recorded once Log is allowed to
// proceed.
type slowSink struct {
	release chan struct{}
	entered chan struct{}
	once    sync.Once
	rec     recordingSink
}

func newSlowSink() *slowSink {
	return &slowSink{release: make(chan struct{}), entered: make(chan struct{})}
}

func (s *slowSink) Log(ev harness.Event) {
	s.once.Do(func() { close(s.entered) })
	<-s.release
	s.rec.Log(ev)
}

func (s *slowSink) snapshot() []harness.Event { return s.rec.snapshot() }

// waitEntered fails the test unless the sink received its first call in time.
func (s *slowSink) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-s.entered:
	case <-time.After(slowSinkWait):
		t.Fatalf("the sink was never called")
	}
}

// loggedLine is one stdout line recovered from a spawn.stdout.lines event.
type loggedLine struct {
	ordinal int
	text    string
}

func fieldOf(ev harness.Event, key string) (string, bool) {
	for _, f := range ev.Fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return "", false
}

func mustField(t *testing.T, ev harness.Event, key string) string {
	t.Helper()
	v, ok := fieldOf(ev, key)
	if !ok {
		t.Fatalf("event %s has no field %q (fields %v)", ev.Name, key, ev.Fields)
	}
	return v
}

// liveLines rebuilds the individually identified lines from the
// spawn.stdout.lines events without assuming how many lines one event carries.
// It fails the test when an event's first/lines fields are inconsistent with
// its message or when ordinals are not contiguous from 1.
func liveLines(t *testing.T, events []harness.Event) []loggedLine {
	t.Helper()
	var out []loggedLine
	for _, ev := range events {
		if ev.Name != "spawn.stdout.lines" {
			continue
		}
		first, err1 := strconv.Atoi(mustField(t, ev, "first"))
		count, err2 := strconv.Atoi(mustField(t, ev, "lines"))
		if err := errors.Join(err1, err2); err != nil {
			t.Fatalf("non-numeric first/lines fields: %v", err)
		}
		parts := strings.Split(ev.Message, "\n")
		if len(parts) != count {
			t.Fatalf("event says lines=%d but message holds %d lines: %q", count, len(parts), ev.Message)
		}
		if first != len(out)+1 {
			t.Fatalf("want first=%d (contiguous ordinals), got %d", len(out)+1, first)
		}
		for i, p := range parts {
			out = append(out, loggedLine{ordinal: first + i, text: p})
		}
	}
	return out
}

func lineTexts(lines []loggedLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return out
}

func eventNames(events []harness.Event) []string {
	out := make([]string, len(events))
	for i, ev := range events {
		out[i] = ev.Name
	}
	return out
}

func hasLine(events []harness.Event, text string) bool {
	for _, l := range liveLinesQuiet(events) {
		if l == text {
			return true
		}
	}
	return false
}

// liveLinesQuiet splits lines events without validating them; for polling.
func liveLinesQuiet(events []harness.Event) []string {
	var out []string
	for _, ev := range events {
		if ev.Name == "spawn.stdout.lines" {
			out = append(out, strings.Split(ev.Message, "\n")...)
		}
	}
	return out
}

// waitUntil polls cond until it holds or the wait elapses.
func waitUntil(t *testing.T, wait time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// lastEvent returns the final event, failing the test when there is none.
func lastEvent(t *testing.T, events []harness.Event) harness.Event {
	t.Helper()
	if len(events) == 0 {
		t.Fatalf("the sink received no events")
	}
	return events[len(events)-1]
}
