package harness_test

// Tests for Run's opt-in incremental stdout observation (RunOptions.OnStdoutLine).
//
// Coverage:
//   - lines reach the observer while the process is still running
//   - the complete stdout is still returned and logged as spawn.stdout
//   - line framing: partial last line, CRLF, empty lines, very long lines
//   - delivery: ordered, sequential, retained slices stay intact
//   - a slow or panicking observer cannot stall the process or change the result
//   - results with and without an observer are identical
//   - timeout and cancellation still return ErrTimeout / ErrCancelled
//
// "While the process is still running" is proved with a gate: the helper writes
// its first line, then waits for a file the observer creates on receiving it.

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mosaic-common/harness"
)

// lineCollector records the lines an observer receives.
type lineCollector struct {
	mu     sync.Mutex
	raw    [][]byte
	onLine func(line string)
}

func (c *lineCollector) observe(line []byte) {
	c.mu.Lock()
	c.raw = append(c.raw, line)
	cb := c.onLine
	c.mu.Unlock()
	if cb != nil {
		cb(string(line))
	}
}

func (c *lineCollector) lines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.raw))
	for i, l := range c.raw {
		out[i] = string(l)
	}
	return out
}

func (s *recordingSink) snapshot() []harness.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]harness.Event(nil), s.events...)
}

func findEvent(events []harness.Event, name string) (harness.Event, bool) {
	for _, ev := range events {
		if ev.Name == name {
			return ev, true
		}
	}
	return harness.Event{}, false
}

func runObserved(t *testing.T, timeout time.Duration, observer func([]byte), sink harness.Sink) (harness.Response, error) {
	t.Helper()
	return harness.Run(context.Background(), harness.Command{Path: helperExe(t)}, nil, harness.RunOptions{
		Timeout:      timeout,
		Sink:         sink,
		OnStdoutLine: observer,
	})
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want observed lines %q, got %q", want, got)
	}
}

// ---------------------------------------------------------------------------
// Incremental delivery
// ---------------------------------------------------------------------------

func TestRun_OnStdoutLine_DeliversLineBeforeProcessExits(t *testing.T) {
	setHelperEnv(t, "stream-gated")
	openGate := newGate(t)
	c := &lineCollector{onLine: func(line string) {
		if line == "first" {
			openGate()
		}
	}}

	resp, err := runObserved(t, 20*time.Second, c.observe, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Stdout) != "first\nsecond\n" {
		t.Errorf("want the process to be released only by the observed first line, got stdout %q", resp.Stdout)
	}
	assertLines(t, c.lines(), []string{"first", "second"})
}

func TestRun_OnStdoutLine_FullStdoutStillReturnedAndLogged(t *testing.T) {
	setHelperEnv(t, "stream-three")
	sink := &recordingSink{}
	c := &lineCollector{}

	resp, err := runObserved(t, 5*time.Second, c.observe, sink)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Stdout) != "l1\nl2\nl3\n" {
		t.Errorf("want full stdout in Response, got %q", resp.Stdout)
	}
	ev, ok := findEvent(sink.snapshot(), "spawn.stdout")
	if !ok {
		t.Fatalf("want a spawn.stdout event")
	}
	if ev.Message != "l1\nl2\nl3\n" {
		t.Errorf("want spawn.stdout to carry the full stdout, got %q", ev.Message)
	}
	assertLines(t, c.lines(), []string{"l1", "l2", "l3"})
}

// ---------------------------------------------------------------------------
// Line framing
// ---------------------------------------------------------------------------

func TestRun_OnStdoutLine_PartialLastLineIsDelivered(t *testing.T) {
	setHelperEnv(t, "stream-partial")
	c := &lineCollector{}

	resp, err := runObserved(t, 5*time.Second, c.observe, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertLines(t, c.lines(), []string{"alpha", "beta"})
	if string(resp.Stdout) != "alpha\nbeta" {
		t.Errorf("want stdout returned verbatim, got %q", resp.Stdout)
	}
}

func TestRun_OnStdoutLine_StripsTerminatorsAndSkipsEmptyLines(t *testing.T) {
	setHelperEnv(t, "stream-crlf")
	c := &lineCollector{}

	resp, err := runObserved(t, 5*time.Second, c.observe, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertLines(t, c.lines(), []string{"a", "b"})
	if string(resp.Stdout) != "a\r\n\r\n\nb\r\n" {
		t.Errorf("want stdout returned verbatim including terminators, got %q", resp.Stdout)
	}
}

func TestRun_OnStdoutLine_MultiMegabyteLineIsOneLine(t *testing.T) {
	setHelperEnv(t, "stream-long")
	const size = 3 << 20
	t.Setenv("GO_HELPER_LONG_BYTES", strconv.Itoa(size))
	c := &lineCollector{}

	_, err := runObserved(t, 20*time.Second, c.observe, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := c.lines()
	if len(got) != 2 {
		t.Fatalf("want 2 lines (the long line and the tail), got %d", len(got))
	}
	if len(got[0]) != size || strings.Trim(got[0], "x") != "" {
		t.Errorf("want one intact %d-byte line, got length %d", size, len(got[0]))
	}
	if got[1] != "tail" {
		t.Errorf("want the line after the long one to be %q, got %q", "tail", got[1])
	}
}

// ---------------------------------------------------------------------------
// Delivery contract
// ---------------------------------------------------------------------------

func TestRun_OnStdoutLine_DeliversInOrderAndRetainedSlicesStayIntact(t *testing.T) {
	setHelperEnv(t, "stream-many")
	t.Setenv("GO_HELPER_LINE_COUNT", "20")
	c := &lineCollector{}

	_, err := runObserved(t, 10*time.Second, c.observe, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := make([]string, 20)
	for i := range want {
		want[i] = "line-" + strconv.Itoa(i)
	}
	// c.raw keeps the slices handed over; they are compared only now, after
	// every later line was delivered.
	assertLines(t, c.lines(), want)
}

func TestRun_OnStdoutLine_CallsNeverOverlap(t *testing.T) {
	setHelperEnv(t, "stream-many")
	t.Setenv("GO_HELPER_LINE_COUNT", "20")
	var inFlight, overlaps, calls int32
	observer := func([]byte) {
		atomic.AddInt32(&calls, 1)
		if atomic.AddInt32(&inFlight, 1) > 1 {
			atomic.AddInt32(&overlaps, 1)
		}
		time.Sleep(3 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
	}

	_, err := runObserved(t, 10*time.Second, observer, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) == 0 {
		t.Fatalf("want the observer to be called")
	}
	if n := atomic.LoadInt32(&overlaps); n != 0 {
		t.Errorf("want sequential calls, observed %d overlapping calls", n)
	}
}

// ---------------------------------------------------------------------------
// Observer misbehaviour
// ---------------------------------------------------------------------------

func TestRun_OnStdoutLine_BlockedObserverDoesNotStallProcessOrChangeResult(t *testing.T) {
	setHelperEnv(t, "stream-many")
	// ~2 MB of output, far beyond any pipe buffer: a pipe reader that waited
	// for the observer would stall the helper and run into the timeout.
	t.Setenv("GO_HELPER_LINE_COUNT", "2000")
	t.Setenv("GO_HELPER_LINE_PAD", "1000")
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls int32
	observer := func([]byte) {
		atomic.AddInt32(&calls, 1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	}
	type result struct {
		resp harness.Response
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := runObserved(t, 15*time.Second, observer, nil)
		done <- result{resp, err}
	}()

	var res result
	select {
	case res = <-done:
	case <-time.After(10 * time.Second):
		close(release)
		t.Fatalf("Run did not return while the observer was blocked")
	}
	select {
	case <-entered:
	default:
		close(release)
		t.Fatalf("want the observer to have been called")
	}
	if res.err != nil {
		t.Errorf("want no error despite the blocked observer, got %v", res.err)
	}
	if got := strings.Count(string(res.resp.Stdout), "\n"); got != 2000 {
		t.Errorf("want all 2000 lines in Response.Stdout, got %d", got)
	}

	// No call may start after Run returned; the blocked call finishing is fine.
	close(release)
	time.Sleep(300 * time.Millisecond)
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("want exactly the one blocked call and no call after Run returned, got %d calls", n)
	}
}

func TestRun_OnStdoutLine_PanickingObserverGetsNoFurtherLinesAndResultIsUnaffected(t *testing.T) {
	setHelperEnv(t, "stream-three")
	var calls int32
	observer := func([]byte) {
		atomic.AddInt32(&calls, 1)
		panic("observer failure")
	}

	resp, err := runObserved(t, 5*time.Second, observer, nil)

	if err != nil {
		t.Fatalf("want no error despite the panic, got %v", err)
	}
	if string(resp.Stdout) != "l1\nl2\nl3\n" {
		t.Errorf("want full stdout, got %q", resp.Stdout)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("want the observer called once before it was dropped, got %d calls", n)
	}
}

// ---------------------------------------------------------------------------
// Parity with the no-observer path
// ---------------------------------------------------------------------------

func TestRun_OnStdoutLine_ResultAndSinkEventsMatchNoObserver(t *testing.T) {
	setHelperEnv(t, "stream-three")
	plainSink, observedSink := &recordingSink{}, &recordingSink{}
	c := &lineCollector{}

	plain, plainErr := runObserved(t, 5*time.Second, nil, plainSink)
	observed, observedErr := runObserved(t, 5*time.Second, c.observe, observedSink)

	if plainErr != nil || observedErr != nil {
		t.Fatalf("unexpected errors: %v / %v", plainErr, observedErr)
	}
	assertLines(t, c.lines(), []string{"l1", "l2", "l3"})
	if string(plain.Stdout) != string(observed.Stdout) || plain.Stderr != observed.Stderr || plain.ExitCode != observed.ExitCode {
		t.Errorf("want identical responses, got %+v vs %+v", plain, observed)
	}
	if !reflect.DeepEqual(plainSink.snapshot(), observedSink.snapshot()) {
		t.Errorf("want identical sink events, got %+v vs %+v", plainSink.snapshot(), observedSink.snapshot())
	}
}

func TestRun_OnStdoutLine_NonZeroExitKeepsErrorAndFullOutput(t *testing.T) {
	setHelperEnv(t, "stream-exit1")
	c := &lineCollector{}

	resp, err := runObserved(t, 5*time.Second, c.observe, nil)

	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Fatalf("want ErrNonZeroExit, got %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("want the error to carry stderr, got %v", err)
	}
	if resp.ExitCode != 1 || string(resp.Stdout) != "partial\n" {
		t.Errorf("want ExitCode 1 and full stdout, got %d / %q", resp.ExitCode, resp.Stdout)
	}
	assertLines(t, c.lines(), []string{"partial"})
}

// ---------------------------------------------------------------------------
// Timeout and cancellation
// ---------------------------------------------------------------------------

func TestRun_OnStdoutLine_TimeoutReturnsErrTimeout(t *testing.T) {
	setHelperEnv(t, "stream-then-hang")
	sink := &recordingSink{}
	c := &lineCollector{}

	resp, err := runObserved(t, time.Second, c.observe, sink)

	if !errors.Is(err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if len(resp.Stdout) != 0 || resp.ExitCode != 0 {
		t.Errorf("want an empty Response on timeout, got %+v", resp)
	}
	if _, ok := findEvent(sink.snapshot(), "spawn.timeout"); !ok {
		t.Errorf("want a spawn.timeout event")
	}
	assertLines(t, c.lines(), []string{"first"})
}

func TestRun_OnStdoutLine_CancellationReturnsErrCancelled(t *testing.T) {
	setHelperEnv(t, "stream-then-hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &lineCollector{onLine: func(string) { cancel() }}

	resp, err := harness.Run(ctx, harness.Command{Path: helperExe(t)}, nil, harness.RunOptions{
		Timeout:      10 * time.Second,
		OnStdoutLine: c.observe,
	})

	if !errors.Is(err, harness.ErrCancelled) {
		t.Fatalf("want ErrCancelled once the observed line triggered cancellation, got %v", err)
	}
	if len(resp.Stdout) != 0 {
		t.Errorf("want an empty Response on cancellation, got %+v", resp)
	}
}
