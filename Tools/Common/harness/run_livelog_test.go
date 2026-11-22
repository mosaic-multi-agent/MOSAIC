package harness_test

// Tests for Run's live stdout logging to RunOptions.Sink.
//
// Coverage:
//   - lines reach the sink while the process is still running, with and without
//     an OnStdoutLine observer
//   - every line is individually identifiable and in arrival order whatever the
//     framing into events
//   - a normal or non-zero exit leaves each line in the sink once, ends with
//     spawn.end carrying the stdout byte count, and keeps the spawn.stderr event
//     but no whole-stdout event
//   - Response, the activity callback and returned errors are unaffected

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"mosaic-common/harness"
)

func runLogged(t *testing.T, sink harness.Sink, observer func([]byte)) (harness.Response, error) {
	t.Helper()
	return harness.Run(context.Background(), harness.Command{Path: helperExe(t)}, nil, harness.RunOptions{
		Timeout:      20 * time.Second,
		Sink:         sink,
		OnStdoutLine: observer,
	})
}

func TestRun_LiveLog_LinesReachSinkBeforeProcessExits(t *testing.T) {
	observers := map[string]func([]byte){
		"without OnStdoutLine": nil,
		"with OnStdoutLine":    func([]byte) {},
	}
	for name, observer := range observers {
		t.Run(name, func(t *testing.T) {
			setHelperEnv(t, "stream-gated")
			openGate := newGate(t)
			sink := &hookSink{}
			sink.onLog = func(ev harness.Event) {
				if ev.Name == "spawn.stdout.lines" && strings.Contains(ev.Message, "first") {
					openGate()
				}
			}

			resp, err := runLogged(t, sink, observer)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(resp.Stdout) != "first\nsecond\n" {
				t.Fatalf("process was not released by the live-logged first line, stdout %q", resp.Stdout)
			}
			got := lineTexts(liveLines(t, sink.snapshot()))
			if !reflect.DeepEqual(got, []string{"first", "second"}) {
				t.Errorf("want lines [first second] in order, got %q", got)
			}
		})
	}
}

func TestRun_LiveLog_ManyLinesEachOnceInArrivalOrder(t *testing.T) {
	setHelperEnv(t, "stream-many")
	t.Setenv("GO_HELPER_LINE_COUNT", "500")
	t.Setenv("GO_HELPER_LINE_PAD", "10")
	sink := &recordingSink{}

	_, err := runLogged(t, sink, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := liveLines(t, sink.snapshot())
	if len(got) != 500 {
		t.Fatalf("want 500 logged lines, got %d", len(got))
	}
	for i, l := range got {
		if !strings.HasPrefix(l.text, "line-"+strconv.Itoa(i)+"p") || l.ordinal != i+1 {
			t.Fatalf("line %d: want ordinal %d with text line-%d..., got %+v", i, i+1, i, l)
		}
	}
}

func TestRun_LiveLog_NormalExitHasNoWholeStdoutEventAndEndsWithSpawnEnd(t *testing.T) {
	setHelperEnv(t, "stream-three")
	sink := &recordingSink{}

	resp, err := runLogged(t, sink, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := sink.snapshot()
	if got := lineTexts(liveLines(t, events)); !reflect.DeepEqual(got, []string{"l1", "l2", "l3"}) {
		t.Errorf("want each line once in order, got %q", got)
	}
	if whole, ok := findEvent(events, "spawn.stdout"); ok {
		t.Errorf("want no whole-stdout spawn.stdout event after exit, got %+v", whole)
	}
	if se, ok := findEvent(events, "spawn.stderr"); !ok || strings.TrimSpace(se.Message) != "warn" {
		t.Errorf("want the unchanged spawn.stderr event with the stderr, got %+v (found %v)", se, ok)
	}
	end := lastEvent(t, events)
	if end.Name != "spawn.end" {
		t.Fatalf("want spawn.end as the last event, got order %v", eventNames(events))
	}
	if mustField(t, end, "reason") != "exit" || mustField(t, end, "stdout_bytes") != "9" || mustField(t, end, "exit_code") != "0" {
		t.Errorf("want reason=exit stdout_bytes=9 exit_code=0, got %v", end.Fields)
	}
	if string(resp.Stdout) != "l1\nl2\nl3\n" {
		t.Errorf("Response.Stdout changed: %q", resp.Stdout)
	}
}

func TestRun_LiveLog_EventOrderOnNormalExit(t *testing.T) {
	setHelperEnv(t, "stream-three")
	sink := &recordingSink{}

	if _, err := runLogged(t, sink, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var seq []string
	for _, n := range eventNames(sink.snapshot()) {
		if len(seq) == 0 || seq[len(seq)-1] != n {
			seq = append(seq, n)
		}
	}
	want := []string{"spawn.stdout.lines", "spawn.stderr", "spawn.end"}
	if !reflect.DeepEqual(seq, want) {
		t.Errorf("want event order %v, got %v", want, seq)
	}
}

func TestRun_LiveLog_NonZeroExitLogsLinesErrorAndEnd(t *testing.T) {
	setHelperEnv(t, "stream-exit1")
	sink := &recordingSink{}

	_, err := runLogged(t, sink, nil)

	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Fatalf("want ErrNonZeroExit, got %v", err)
	}
	events := sink.snapshot()
	if got := lineTexts(liveLines(t, events)); !reflect.DeepEqual(got, []string{"partial"}) {
		t.Errorf("want [partial], got %q", got)
	}
	if _, ok := findEvent(events, "spawn.error"); !ok {
		t.Errorf("want the unchanged spawn.error event")
	}
	end := lastEvent(t, events)
	if end.Name != "spawn.end" || mustField(t, end, "reason") != "nonzero_exit" ||
		mustField(t, end, "stdout_bytes") != "8" || mustField(t, end, "exit_code") != "1" {
		t.Errorf("want last event spawn.end reason=nonzero_exit stdout_bytes=8 exit_code=1, got %+v", end)
	}
}

func TestRun_LiveLog_UnterminatedLastLineIsLoggedOnNormalExit(t *testing.T) {
	setHelperEnv(t, "stream-partial")
	sink := &recordingSink{}

	_, err := runLogged(t, sink, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := sink.snapshot()
	if got := lineTexts(liveLines(t, events)); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Errorf("want [alpha beta], got %q", got)
	}
	end := lastEvent(t, events)
	if end.Name != "spawn.end" || mustField(t, end, "stdout_bytes") != "10" {
		t.Errorf("want spawn.end with stdout_bytes=10, got %+v", end)
	}
}

func TestRun_LiveLog_CRStrippedEmptyLinesSkippedBytesCountAll(t *testing.T) {
	setHelperEnv(t, "stream-crlf")
	sink := &recordingSink{}

	_, err := runLogged(t, sink, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := sink.snapshot()
	got := liveLines(t, events)
	if !reflect.DeepEqual(lineTexts(got), []string{"a", "b"}) || got[1].ordinal != 2 {
		t.Errorf("want lines [a b] with ordinals 1,2, got %+v", got)
	}
	if end := lastEvent(t, events); mustField(t, end, "stdout_bytes") != "9" {
		t.Errorf("want stdout_bytes=9 (every byte, skipped lines included), got %v", end.Fields)
	}
}

func TestRun_LiveLog_SilentProcessStillEndsWithSpawnEnd(t *testing.T) {
	setHelperEnv(t, "empty")
	sink := &recordingSink{}

	if _, err := runLogged(t, sink, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := sink.snapshot()
	if len(liveLines(t, events)) != 0 {
		t.Errorf("want no stdout lines for a silent process")
	}
	end := lastEvent(t, events)
	if end.Name != "spawn.end" || mustField(t, end, "stdout_bytes") != "0" || mustField(t, end, "reason") != "exit" {
		t.Errorf("want spawn.end reason=exit stdout_bytes=0, got %+v", end)
	}
}

func TestRun_LiveLog_ObserverStillGetsLinesAndResponseIsComplete(t *testing.T) {
	setHelperEnv(t, "stream-three")
	sink := &recordingSink{}
	c := &lineCollector{}

	resp, err := runLogged(t, sink, c.observe)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertLines(t, c.lines(), []string{"l1", "l2", "l3"})
	if string(resp.Stdout) != "l1\nl2\nl3\n" {
		t.Errorf("Response.Stdout changed: %q", resp.Stdout)
	}
	if got := lineTexts(liveLines(t, sink.snapshot())); !reflect.DeepEqual(got, []string{"l1", "l2", "l3"}) {
		t.Errorf("want sink lines [l1 l2 l3], got %q", got)
	}
}

func TestRun_LiveLog_NilSinkIsAccepted(t *testing.T) {
	setHelperEnv(t, "stream-three")

	resp, err := runLogged(t, nil, nil)

	if err != nil || string(resp.Stdout) != "l1\nl2\nl3\n" {
		t.Errorf("want normal result with a nil sink, got %q, %v", resp.Stdout, err)
	}
}
