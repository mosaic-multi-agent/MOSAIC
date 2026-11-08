// Package testutil holds test support shared by the harness adapter tests:
// a recording domain.ActivityObserver, the gate that proves activity arrives
// while a fake-CLI process is still running, and observers that misbehave.
// Imported only from _test.go files.
package testutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

// GateWait bounds how long a paced fake-CLI scenario waits for its gate.
const GateWait = 5 * time.Second

// Observer is a recording domain.ActivityObserver.
type Observer struct {
	mu       sync.Mutex
	starts   []domain.ActivityInvocation
	items    []domain.ActivityItem
	events   []string // "start", "item", "end" in call order
	statuses []domain.ActivityStatus
	ends     int
	// OnItem, when set before use, is called after each item is recorded.
	OnItem func(domain.ActivityItem)
}

// InvocationStarted implements domain.ActivityObserver.
func (o *Observer) InvocationStarted(inv domain.ActivityInvocation) domain.ActivityFeed {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.starts = append(o.starts, inv)
	o.events = append(o.events, "start")
	return &recordingFeed{o: o}
}

type recordingFeed struct{ o *Observer }

func (f *recordingFeed) Activity(item domain.ActivityItem) {
	f.o.mu.Lock()
	f.o.items = append(f.o.items, item)
	f.o.events = append(f.o.events, "item")
	cb := f.o.OnItem
	f.o.mu.Unlock()
	if cb != nil {
		cb(item)
	}
}

// Status records the status without adding a lifecycle event, so statuses do
// not disturb the start/item/end ordering checks.
func (f *recordingFeed) Status(st domain.ActivityStatus) {
	f.o.mu.Lock()
	defer f.o.mu.Unlock()
	f.o.statuses = append(f.o.statuses, st)
}

// Statuses returns the statuses recorded so far, in call order.
func (o *Observer) Statuses() []domain.ActivityStatus {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]domain.ActivityStatus(nil), o.statuses...)
}

func (f *recordingFeed) Ended() {
	f.o.mu.Lock()
	defer f.o.mu.Unlock()
	f.o.ends++
	f.o.events = append(f.o.events, "end")
}

// Snapshot returns copies of everything recorded so far.
func (o *Observer) Snapshot() (starts []domain.ActivityInvocation, items []domain.ActivityItem, events []string, ends int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]domain.ActivityInvocation(nil), o.starts...),
		append([]domain.ActivityItem(nil), o.items...),
		append([]string(nil), o.events...), o.ends
}

// NewGatedObserver sets GO_HELPER_GATE_FILE for the helper process and returns
// an observer that opens the gate on the first tool call it receives.
func NewGatedObserver(t *testing.T) (*Observer, string) {
	t.Helper()
	gate := filepath.Join(t.TempDir(), "gate")
	t.Setenv("GO_HELPER_GATE_FILE", gate)
	var once sync.Once
	obs := &Observer{}
	obs.OnItem = func(item domain.ActivityItem) {
		if item.Kind == domain.ActivityToolCall {
			once.Do(func() { os.WriteFile(gate, []byte("open"), 0o644) }) //nolint:errcheck
		}
	}
	return obs, gate
}

// PresetOpenGate makes a paced scenario run to the end without waiting.
func PresetOpenGate(t *testing.T) {
	t.Helper()
	gate := filepath.Join(t.TempDir(), "gate")
	if err := os.WriteFile(gate, []byte("open"), 0o644); err != nil {
		t.Fatalf("write gate: %v", err)
	}
	t.Setenv("GO_HELPER_GATE_FILE", gate)
}

// UnopenedGate points a paced scenario at a gate that nobody opens, so the
// process stays alive after its first lines until it is stopped.
func UnopenedGate(t *testing.T) {
	t.Helper()
	t.Setenv("GO_HELPER_GATE_FILE", filepath.Join(t.TempDir(), "never-opened"))
}

// AssertGateWasOpenedWhileRunning fails when the scenario had to give up
// waiting, i.e. no tool call reached the observer before the process exited.
func AssertGateWasOpenedWhileRunning(t *testing.T, gate string) {
	t.Helper()
	if _, err := os.Stat(gate + ".timeout"); err == nil {
		t.Fatal("no tool-call activity reached the observer while the process was running")
	}
}

// PacedScenario returns a fake-CLI scenario that writes first, waits for the
// gate file, then writes rest. If the gate stays closed it leaves a ".timeout"
// marker next to the gate file and continues.
func PacedScenario(first, rest []string) func() {
	return func() {
		EmitLines(first...)
		gate := os.Getenv("GO_HELPER_GATE_FILE")
		deadline := time.Now().Add(GateWait)
		for {
			if _, err := os.Stat(gate); err == nil {
				break
			}
			if time.Now().After(deadline) {
				os.WriteFile(gate+".timeout", []byte("x"), 0o644) //nolint:errcheck
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		EmitLines(rest...)
	}
}

// EmitLines writes each line plus a newline to stdout.
func EmitLines(lines ...string) {
	for _, l := range lines {
		os.Stdout.WriteString(l + "\n") //nolint:errcheck
	}
}

// AssertLifecycle checks exactly one start first, exactly one end last, and
// only items in between.
func AssertLifecycle(t *testing.T, events []string, ends int) {
	t.Helper()
	if len(events) < 2 || events[0] != "start" || events[len(events)-1] != "end" {
		t.Fatalf("want start ... end, got %v", events)
	}
	if ends != 1 {
		t.Errorf("Ended must be called exactly once, got %d", ends)
	}
	for _, e := range events[1 : len(events)-1] {
		if e != "item" {
			t.Errorf("unexpected %q between start and end: %v", e, events)
		}
	}
}

// FindItem returns the first item of kind and name.
func FindItem(items []domain.ActivityItem, kind domain.ActivityKind, name string) (domain.ActivityItem, bool) {
	for _, it := range items {
		if it.Kind == kind && it.Name == name {
			return it, true
		}
	}
	return domain.ActivityItem{}, false
}
