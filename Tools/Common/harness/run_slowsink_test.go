package harness_test

// Tests that a slow or blocking sink cannot hold up Run's stdout pipe or the
// no-output watch, and that lines queued behind it are still delivered.
//
// The sink's Log blocks until the test releases it. While it is blocked the
// OnStdoutLine observer (which never waits on the sink) shows the pipe is still
// being drained, and Run is only allowed to return after the release.

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"mosaic-common/harness"
)

const slowSinkWait = 15 * time.Second

func startRun(t *testing.T, ctx context.Context, opts harness.RunOptions) <-chan runResult {
	t.Helper()
	ch := make(chan runResult, 1)
	go func() {
		resp, err := harness.Run(ctx, harness.Command{Path: helperExe(t)}, nil, opts)
		ch <- runResult{resp: resp, err: err}
	}()
	return ch
}

func awaitResult(t *testing.T, ch <-chan runResult) runResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(slowSinkWait):
		t.Fatalf("Run did not return after the sink was released")
		return runResult{}
	}
}

func TestRun_SlowSink_DoesNotStallStdoutPipe(t *testing.T) {
	const total = 3000
	setHelperEnv(t, "stream-many")
	t.Setenv("GO_HELPER_LINE_COUNT", strconv.Itoa(total))
	t.Setenv("GO_HELPER_LINE_PAD", "100") // ~330 KB, well above pipe buffers
	sink := newSlowSink()
	var seen atomic.Int64
	done := startRun(t, context.Background(), harness.RunOptions{
		Timeout: 60 * time.Second, Sink: sink,
		OnStdoutLine: func([]byte) { seen.Add(1) },
	})

	waitUntil(t, slowSinkWait, "all lines to reach the observer while the sink blocks", func() bool {
		return seen.Load() == total
	})
	close(sink.release)
	r := awaitResult(t, done)

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if got := liveLines(t, sink.snapshot()); len(got) != total {
		t.Errorf("want all %d lines in the sink after release, got %d", total, len(got))
	}
}

func TestRun_SlowSink_DoesNotFireNoOutputWatchdog(t *testing.T) {
	setHelperEnv(t, "wd-drip")
	setDrip(t, 25, 100*time.Millisecond) // ~2.5 s of output, gaps far below the watchdog
	sink := newSlowSink()
	var seen atomic.Int64
	done := startRun(t, context.Background(), harness.RunOptions{
		Timeout: 60 * time.Second, NoOutputTimeout: 1 * time.Second, Sink: sink,
		OnStdoutLine: func([]byte) { seen.Add(1) },
	})

	waitUntil(t, slowSinkWait, "all ticks to reach the observer", func() bool { return seen.Load() == 25 })
	// Let the lag outlast the watchdog duration before releasing the sink.
	time.Sleep(1500 * time.Millisecond)
	close(sink.release)
	r := awaitResult(t, done)

	if errors.Is(r.err, harness.ErrNoOutput) || r.err != nil {
		t.Errorf("want a clean exit while the sink lags, got %v", r.err)
	}
	if got := liveLines(t, sink.snapshot()); len(got) != 25 {
		t.Errorf("want 25 logged lines, got %d", len(got))
	}
}

func TestRun_SlowSink_WatchdogStillFiresWhileSinkLags(t *testing.T) {
	setHelperEnv(t, "ll-lines-then-hang")
	pidFile := newPidFile(t)
	sink := newSlowSink()
	done := startRun(t, context.Background(), harness.RunOptions{
		Timeout: 60 * time.Second, NoOutputTimeout: 1 * time.Second, Sink: sink,
	})
	sink.waitEntered(t)

	// The watchdog must end the process while the sink is still blocked.
	requireGone(t, waitForPid(t, pidFile))
	close(sink.release)
	r := awaitResult(t, done)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Errorf("want ErrNoOutput from the watchdog despite the blocked sink, got %v", r.err)
	}
}

func TestRun_SlowSink_InvocationTimeoutKillNotDelayedBySink(t *testing.T) {
	setHelperEnv(t, "ll-lines-then-hang")
	pidFile := newPidFile(t)
	sink := newSlowSink()
	var seen atomic.Int64
	done := startRun(t, context.Background(), harness.RunOptions{
		Timeout: 1500 * time.Millisecond, Sink: sink,
		OnStdoutLine: func([]byte) { seen.Add(1) },
	})
	waitUntil(t, slowSinkWait, "lines to be observed", func() bool { return seen.Load() == 3 })
	sink.waitEntered(t)

	// The kill must happen on schedule even though the sink has not returned.
	requireGone(t, waitForPid(t, pidFile))
	close(sink.release)
	r := awaitResult(t, done)

	if !errors.Is(r.err, harness.ErrTimeout) {
		t.Errorf("want ErrTimeout, got %v", r.err)
	}
}

func TestRun_SlowSink_QueuedLinesStillLoggedAfterCancellation(t *testing.T) {
	setHelperEnv(t, "ll-lines-then-hang")
	sink := newSlowSink()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var seen atomic.Int64
	done := startRun(t, ctx, harness.RunOptions{
		Timeout: 60 * time.Second, Sink: sink,
		OnStdoutLine: func([]byte) {
			if seen.Add(1) == 3 {
				cancel()
			}
		},
	})

	waitUntil(t, slowSinkWait, "all lines observed", func() bool { return seen.Load() == 3 })
	time.Sleep(300 * time.Millisecond) // the kill happens while the sink still blocks
	close(sink.release)
	r := awaitResult(t, done)

	if !errors.Is(r.err, harness.ErrCancelled) {
		t.Fatalf("want ErrCancelled, got %v", r.err)
	}
	events := sink.snapshot()
	got := lineTexts(liveLines(t, events))
	if len(got) != 3 || got[0] != "q1" || got[1] != "q2" || got[2] != "q3" {
		t.Errorf("want queued lines [q1 q2 q3] delivered after the kill, got %q", got)
	}
	end := lastEvent(t, events)
	if end.Name != "spawn.end" || mustField(t, end, "reason") != "cancelled" || mustField(t, end, "stdout_bytes") != "9" {
		t.Errorf("want last event spawn.end reason=cancelled stdout_bytes=9, got %+v", end)
	}
}
