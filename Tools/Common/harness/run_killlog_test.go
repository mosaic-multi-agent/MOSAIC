package harness_test

// Tests for what Run leaves in RunOptions.Sink when an invocation is killed.
//
// Coverage (invocation timeout, no-output timeout, context cancellation):
//   - every stdout byte received before the kill reaches the sink, including an
//     unterminated last line
//   - the stderr captured before the kill reaches the sink
//   - spawn.end names the end reason and carries the stdout byte count, and is
//     the last event
//   - returned errors and the empty Response are unchanged

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mosaic-common/harness"
)

type killCase struct {
	name    string
	reason  string
	timeout bool // expects a spawn.timeout event
	noOut   bool // expects a spawn.no_output event
	wantErr error
}

// runKilled runs the ll-kill-partial helper until the given kill path fires.
func runKilled(t *testing.T, reason string, sink *hookSink) runResult {
	t.Helper()
	setHelperEnv(t, "ll-kill-partial")
	ctx := context.Background()
	opts := harness.RunOptions{Sink: sink, Timeout: 60 * time.Second}
	switch reason {
	case "timeout":
		opts.Timeout = 2 * time.Second
	case "no_output":
		opts.NoOutputTimeout = 1500 * time.Millisecond
	case "cancelled":
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		t.Cleanup(cancel)
		opts.OnStdoutLine = func(line []byte) {
			if string(line) == "two" {
				cancel()
			}
		}
	}
	return runBounded(t, ctx, opts)
}

// assertKillEventOrder checks the documented order: lines, the kill event (when
// the path has one), stderr, end.
func assertKillEventOrder(t *testing.T, events []harness.Event, tc killCase) {
	t.Helper()
	want := []string{"spawn.stdout.lines"}
	switch {
	case tc.timeout:
		want = append(want, "spawn.timeout")
	case tc.noOut:
		want = append(want, "spawn.no_output")
	}
	want = append(want, "spawn.stderr", "spawn.end")
	var got []string
	for _, n := range eventNames(events) {
		if len(got) == 0 || got[len(got)-1] != n || n != "spawn.stdout.lines" {
			got = append(got, n)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want event order %v (lines events collapsed), got %v", want, got)
	}
}

func TestRun_KillLog_PreKillOutputStderrReasonAndByteCountReachSink(t *testing.T) {
	cases := []killCase{
		{"invocation timeout", "timeout", true, false, harness.ErrTimeout},
		{"no-output timeout", "no_output", false, true, harness.ErrNoOutput},
		{"context cancellation", "cancelled", false, false, harness.ErrCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &hookSink{}

			r := runKilled(t, tc.reason, sink)

			if !errors.Is(r.err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, r.err)
			}
			if len(r.resp.Stdout) != 0 {
				t.Errorf("want an empty Response on a kill path, got stdout %q", r.resp.Stdout)
			}
			events := sink.snapshot()
			got := lineTexts(liveLines(t, events))
			if !reflect.DeepEqual(got, []string{"one", "two", "part"}) {
				t.Errorf("want pre-kill lines [one two part] incl. the unterminated one, got %q", got)
			}
			se, ok := findEvent(events, "spawn.stderr")
			if !ok || !strings.Contains(se.Message, "err-before-kill") {
				t.Errorf("want spawn.stderr with the pre-kill stderr, got %+v (found %v)", se, ok)
			}
			_, gotTimeout := findEvent(events, "spawn.timeout")
			_, gotNoOut := findEvent(events, "spawn.no_output")
			if gotTimeout != tc.timeout || gotNoOut != tc.noOut {
				t.Errorf("want spawn.timeout=%v spawn.no_output=%v, got %v %v", tc.timeout, tc.noOut, gotTimeout, gotNoOut)
			}
			end := lastEvent(t, events)
			if end.Name != "spawn.end" {
				t.Fatalf("want spawn.end as last event, got order %v", eventNames(events))
			}
			assertKillEventOrder(t, events, tc)
			if mustField(t, end, "reason") != tc.reason || mustField(t, end, "stdout_bytes") != "12" {
				t.Errorf("want reason=%s stdout_bytes=12, got %v", tc.reason, end.Fields)
			}
			if _, has := fieldOf(end, "exit_code"); has {
				t.Errorf("want no exit_code on a kill path, got %v", end.Fields)
			}
		})
	}
}

func TestRun_KillLog_NoWholeStdoutEventOnKillPaths(t *testing.T) {
	sink := &hookSink{}

	runKilled(t, "timeout", sink)

	if _, ok := findEvent(sink.snapshot(), "spawn.stdout"); ok {
		t.Errorf("kill paths must not emit the whole-stdout spawn.stdout event")
	}
}

func TestRun_KillLog_CancellationBeforeAnyOutputStillLogsEnd(t *testing.T) {
	setHelperEnv(t, "hang")
	sink := &recordingSink{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()
	defer cancel()

	r := runBounded(t, ctx, harness.RunOptions{Sink: sink, Timeout: 60 * time.Second})

	if !errors.Is(r.err, harness.ErrCancelled) {
		t.Fatalf("want ErrCancelled, got %v", r.err)
	}
	events := sink.snapshot()
	end := lastEvent(t, events)
	if end.Name != "spawn.end" || mustField(t, end, "reason") != "cancelled" || mustField(t, end, "stdout_bytes") != "0" {
		t.Errorf("want spawn.end reason=cancelled stdout_bytes=0, got order %v last %+v", eventNames(events), end)
	}
}

func TestRun_KillLog_SinkLinesCompleteWhenObserverSet(t *testing.T) {
	setHelperEnv(t, "ll-kill-partial")
	sink := &recordingSink{}
	c := &lineCollector{}

	r := runBounded(t, context.Background(), harness.RunOptions{
		Sink: sink, OnStdoutLine: c.observe, Timeout: 2 * time.Second,
	})

	if !errors.Is(r.err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", r.err)
	}
	got := lineTexts(liveLines(t, sink.snapshot()))
	if !reflect.DeepEqual(got, []string{"one", "two", "part"}) {
		t.Errorf("want sink lines [one two part] with an observer set, got %q", got)
	}
	seen := c.lines()
	if len(seen) < 2 || seen[0] != "one" || seen[1] != "two" {
		t.Errorf("want the observer to have seen one and two before the kill, got %q", seen)
	}
}
