package testutil

// A recording debug logger and assertions on the live-output entries of one
// harness invocation (harness.stdout.lines, harness.stderr,
// harness.process.end), shared by the adapter tests.

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

// legacyWholeStdoutEvent is the retired event that once held the whole stdout
// (or the parsed final text) after a normal exit. Nothing logs it any more.
const legacyWholeStdoutEvent = "harness.stdout"

// LogEntry is one recorded debug-log call.
type LogEntry struct {
	Event   string
	Message string
	Fields  []domain.DebugField
}

// Field returns the value of the named field.
func (e LogEntry) Field(key string) (string, bool) {
	for _, f := range e.Fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return "", false
}

// RecordingLogger is a thread-safe domain.DebugLogger that keeps every call
// with its fields.
type RecordingLogger struct {
	mu      sync.Mutex
	entries []LogEntry
}

// Log implements domain.DebugLogger.
func (r *RecordingLogger) Log(event string, message string, fields ...domain.DebugField) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, LogEntry{Event: event, Message: message, Fields: append([]domain.DebugField(nil), fields...)})
}

// Entries returns a copy of the recorded entries in call order.
func (r *RecordingLogger) Entries() []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]LogEntry(nil), r.entries...)
}

// Events returns the recorded event names in call order.
func (r *RecordingLogger) Events() []string {
	var out []string
	for _, e := range r.Entries() {
		out = append(out, e.Event)
	}
	return out
}

// Named returns the entries of one event, in call order.
func Named(entries []LogEntry, event string) []LogEntry {
	var out []LogEntry
	for _, e := range entries {
		if e.Event == event {
			out = append(out, e)
		}
	}
	return out
}

// LiveLines returns the stdout lines carried by the harness.stdout.lines
// entries of agent, in order, without assuming how many lines one entry holds.
// It fails the test when such an entry lacks its first/lines fields, when they
// disagree with the message, or when the ordinals are not contiguous from 1.
func LiveLines(t *testing.T, entries []LogEntry, agent string) []string {
	t.Helper()
	var out []string
	for _, e := range Named(entries, domain.EventHarnessStdoutLines) {
		if got, _ := e.Field("agent"); got != agent {
			continue
		}
		first, err1 := strconv.Atoi(fieldOrFail(t, e, "first"))
		count, err2 := strconv.Atoi(fieldOrFail(t, e, "lines"))
		parts := strings.Split(e.Message, "\n")
		if err1 != nil || err2 != nil || count != len(parts) || first != len(out)+1 {
			t.Fatalf("inconsistent %s entry: first=%v lines=%v for %d message lines after %d lines",
				e.Event, err1, err2, len(parts), len(out))
		}
		out = append(out, parts...)
	}
	return out
}

func fieldOrFail(t *testing.T, e LogEntry, key string) string {
	t.Helper()
	v, ok := e.Field(key)
	if !ok {
		t.Fatalf("entry %s has no field %q (fields %v)", e.Event, key, e.Fields)
	}
	return v
}

// AssertEveryLiveEntryAttributed fails the test unless there is at least one
// harness.stdout.lines entry and every one carries agent=<agent>.
func AssertEveryLiveEntryAttributed(t *testing.T, entries []LogEntry, agent string) {
	t.Helper()
	live := Named(entries, domain.EventHarnessStdoutLines)
	if len(live) == 0 {
		t.Fatalf("want %s entries, got events %v", domain.EventHarnessStdoutLines, eventNames(entries))
	}
	for _, e := range live {
		if got, _ := e.Field("agent"); got != agent {
			t.Errorf("want every %s entry to carry agent=%q, got fields %v", e.Event, agent, e.Fields)
		}
	}
}

// AssertNoWholeStdoutEntry fails the test when the retired whole-stdout entry
// (harness.stdout) was logged.
func AssertNoWholeStdoutEntry(t *testing.T, entries []LogEntry) {
	t.Helper()
	if got := Named(entries, legacyWholeStdoutEvent); len(got) != 0 {
		t.Errorf("want no %s entry, got %d (events %v)", legacyWholeStdoutEvent, len(got), eventNames(entries))
	}
}

// ProcessEnd returns the single harness.process.end entry of agent, failing
// the test when there is not exactly one.
func ProcessEnd(t *testing.T, entries []LogEntry, agent string) LogEntry {
	t.Helper()
	var found []LogEntry
	for _, e := range Named(entries, domain.EventHarnessProcessEnd) {
		if got, _ := e.Field("agent"); got == agent {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %s entry for agent %q, got %d (events %v)", domain.EventHarnessProcessEnd, agent, len(found), eventNames(entries))
	}
	return found[0]
}

// AssertProcessEnd checks the end entry of agent: its reason, and that
// stdout_bytes is the given count (a negative count only requires the field).
func AssertProcessEnd(t *testing.T, entries []LogEntry, agent, reason string, stdoutBytes int) {
	t.Helper()
	end := ProcessEnd(t, entries, agent)
	if got := fieldOrFail(t, end, "reason"); got != reason {
		t.Errorf("want reason=%s, got %q", reason, got)
	}
	got := fieldOrFail(t, end, "stdout_bytes")
	n, err := strconv.Atoi(got)
	if err != nil || n < 0 || (stdoutBytes >= 0 && n != stdoutBytes) {
		t.Errorf("want stdout_bytes=%d, got %q", stdoutBytes, got)
	}
}

// AssertKillTrail checks what a killed invocation leaves for agent: its
// pre-kill stdout lines (wantLines, in order), a harness.stderr entry holding
// wantStderr, and harness.process.end with the given reason, the end entry
// coming last.
func AssertKillTrail(t *testing.T, entries []LogEntry, agent, reason string, wantLines []string, wantStderr string) {
	t.Helper()
	AssertEveryLiveEntryAttributed(t, entries, agent)
	lines := LiveLines(t, entries, agent)
	if strings.Join(lines, "\n") != strings.Join(wantLines, "\n") {
		t.Errorf("want pre-kill stdout lines %q, got %q", wantLines, lines)
	}
	var stderrSeen bool
	for _, e := range Named(entries, domain.EventHarnessStderr) {
		if strings.Contains(e.Message, wantStderr) {
			stderrSeen = true
		}
	}
	if !stderrSeen {
		t.Errorf("want a %s entry containing %q, got events %v", domain.EventHarnessStderr, wantStderr, eventNames(entries))
	}
	AssertProcessEnd(t, entries, agent, reason, -1)
	var endIdx, lastLive int
	for i, e := range entries {
		switch e.Event {
		case domain.EventHarnessProcessEnd:
			endIdx = i
		case domain.EventHarnessStdoutLines, domain.EventHarnessStderr:
			lastLive = i
		}
	}
	if lastLive > endIdx {
		t.Errorf("want the end entry after the output entries, got order %v", eventNames(entries))
	}
}

// WaitForLine blocks until a live stdout line equal to text was logged and
// reports whether it was; on timeout it records a test error (safe to call
// from a helper goroutine, so a caller can still cancel and finish).
func WaitForLine(t *testing.T, r *RecordingLogger, text string, wait time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		for _, e := range Named(r.Entries(), domain.EventHarnessStdoutLines) {
			for _, l := range strings.Split(e.Message, "\n") {
				if l == text {
					return true
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("timed out waiting for stdout line %q in the log", text)
	return false
}

func eventNames(entries []LogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Event
	}
	return out
}
