package testutil

// Observers that misbehave, and the checks built on them: a feed that panics
// or blocks must not change what an invocation returns or stall it; a
// recording debug logger lets tests compare log events with and without an
// observer.

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

// NeverBlocksWait bounds how long an invocation with a blocked feed may take.
const NeverBlocksWait = 20 * time.Second

// FailingObserver is an observer whose feed panics or blocks on Activity, and
// panics on Ended. It counts the calls it received so a test can tell the
// adapter really fed it.
type FailingObserver struct {
	panics  bool
	release chan struct{}
	calls   atomic.Int32
}

// NewPanickingObserver returns an observer whose feed panics on every call.
func NewPanickingObserver() *FailingObserver { return &FailingObserver{panics: true} }

// NewBlockingObserver returns an observer whose Activity blocks until Release.
func NewBlockingObserver() *FailingObserver {
	return &FailingObserver{release: make(chan struct{})}
}

// Release unblocks every blocked Activity call. Safe to call once.
func (o *FailingObserver) Release() { close(o.release) }

// Calls is the number of Activity calls received.
func (o *FailingObserver) Calls() int { return int(o.calls.Load()) }

// InvocationStarted implements domain.ActivityObserver.
func (o *FailingObserver) InvocationStarted(domain.ActivityInvocation) domain.ActivityFeed {
	return &failingFeed{o: o}
}

type failingFeed struct{ o *FailingObserver }

func (f *failingFeed) Activity(domain.ActivityItem) {
	f.o.calls.Add(1)
	if f.o.panics {
		panic("activity feed failure")
	}
	<-f.o.release
}

func (f *failingFeed) Ended() {
	if f.o.panics {
		panic("activity feed failure on end")
	}
}

// AssertFeedFailuresHarmless runs one invocation per misbehaving observer and
// requires the same outcome string as a run without observer, returned within
// NeverBlocksWait. run builds an adapter around obs (nil means none), performs
// the invocation and renders its result and error as a string.
func AssertFeedFailuresHarmless(t *testing.T, run func(obs domain.ActivityObserver) string) {
	t.Helper()
	want := run(nil)

	t.Run("panicking feed", func(t *testing.T) {
		obs := NewPanickingObserver()
		got := runBounded(t, func() string { return run(obs) })
		if got != want {
			t.Errorf("outcome changed by a panicking feed: want %q, got %q", want, got)
		}
		if obs.Calls() == 0 {
			t.Error("the panicking feed was never called, so the adapter did not feed the observer")
		}
	})
	t.Run("blocking feed", func(t *testing.T) {
		obs := NewBlockingObserver()
		defer obs.Release()
		got := runBounded(t, func() string { return run(obs) })
		if got != want {
			t.Errorf("outcome changed by a blocking feed: want %q, got %q", want, got)
		}
		if obs.Calls() == 0 {
			t.Error("the blocking feed was never called, so the adapter did not feed the observer")
		}
	})
}

// runBounded runs f and fails the test when it does not return in time.
func runBounded(t *testing.T, f func() string) string {
	t.Helper()
	done := make(chan string, 1)
	go func() { done <- f() }()
	select {
	case got := <-done:
		return got
	case <-time.After(NeverBlocksWait):
		t.Fatal("the invocation did not return while the activity feed was misbehaving")
		return ""
	}
}

// Logger is a recording domain.DebugLogger.
type Logger struct {
	mu      sync.Mutex
	entries []string
}

// Log implements domain.DebugLogger. It records the event name and the sorted
// field keys; messages and values vary with timing and paths.
func (l *Logger) Log(event string, _ string, fields ...domain.DebugField) {
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	sort.Strings(keys)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, event+"["+strings.Join(keys, ",")+"]")
}

// Entries returns the recorded events in order.
func (l *Logger) Entries() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

// AssertLogsIdentical requires the debug-log events of a run with an observer
// and of a run with a nil observer to equal those of a run without any.
func AssertLogsIdentical(t *testing.T, run func(lg domain.DebugLogger, obs domain.ActivityObserver, withObserver bool)) {
	t.Helper()
	collect := func(obs domain.ActivityObserver, with bool) []string {
		lg := &Logger{}
		run(lg, obs, with)
		return lg.Entries()
	}
	base := collect(nil, false)
	if len(base) == 0 {
		t.Fatal("the base run logged nothing; the comparison would prove nothing")
	}
	for name, got := range map[string][]string{
		"observer":     collect(&Observer{}, true),
		"nil observer": collect(nil, true),
	} {
		if strings.Join(got, "\n") != strings.Join(base, "\n") {
			t.Errorf("debug-log events differ with %s:\nbase: %v\n got: %v", name, base, got)
		}
	}
}
