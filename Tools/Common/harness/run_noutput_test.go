package harness_test

// Tests for Run's opt-in no-output watchdog (RunOptions.NoOutputTimeout).
//
// Coverage:
//   - a silent process is ended with an error wrapping ErrNoOutput and ErrTimeout
//   - the sink gets a spawn.no_output event; the Response is empty
//   - stdout data resets the silence, with and without an OnStdoutLine observer;
//     stderr does not
//   - the whole process tree ends, and Run returns promptly
//   - zero or negative duration disables the watchdog
//   - wall-clock timeout, cancellation and exit keep their own results when they
//     come first
//
// Tests marked "regression" protect behaviour that exists without the watchdog
// and do not by themselves show that the watchdog works.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mosaic-common/harness"
)

func runWatchdog(t *testing.T, noOutput, timeout time.Duration, sink harness.Sink, observer func([]byte)) runResult {
	t.Helper()
	return runBounded(t, context.Background(), harness.RunOptions{
		NoOutputTimeout: noOutput,
		Timeout:         timeout,
		Sink:            sink,
		OnStdoutLine:    observer,
	})
}

// ---------------------------------------------------------------------------
// Firing
// ---------------------------------------------------------------------------

func TestRun_NoOutput_SilentProcessEndsWithErrNoOutputAndErrTimeout(t *testing.T) {
	setHelperEnv(t, "hang")

	r := runWatchdog(t, time.Second, 60*time.Second, nil, nil)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", r.err)
	}
	if !errors.Is(r.err, harness.ErrTimeout) {
		t.Errorf("want the watchdog error to also satisfy ErrTimeout, got %v", r.err)
	}
	if errors.Is(r.err, harness.ErrCancelled) {
		t.Errorf("a watchdog end must not satisfy ErrCancelled, got %v", r.err)
	}
	if r.took > 6*time.Second {
		t.Errorf("want the watchdog to end the run near its 1s duration, took %s", r.took)
	}
}

func TestRun_NoOutput_ReturnsEmptyResponse(t *testing.T) {
	setHelperEnv(t, "hang")

	r := runWatchdog(t, time.Second, 60*time.Second, nil, nil)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", r.err)
	}
	if r.resp.ExitCode != 0 || len(r.resp.Stdout) != 0 || r.resp.Stderr != "" || r.resp.Text != "" {
		t.Errorf("want an empty Response, got %+v", r.resp)
	}
}

func TestRun_NoOutput_ErrorNamesTheSilence(t *testing.T) {
	setHelperEnv(t, "hang")

	r := runWatchdog(t, time.Second, 60*time.Second, nil, nil)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", r.err)
	}
	if !strings.Contains(r.err.Error(), harness.ErrNoOutput.Error()) || !strings.Contains(r.err.Error(), time.Second.String()) {
		t.Errorf("want the message to carry the sentinel text and the duration %s, got %q", time.Second, r.err.Error())
	}
}

func TestRun_NoOutput_LogsSpawnNoOutputEvent(t *testing.T) {
	setHelperEnv(t, "hang")
	sink := &recordingSink{}

	r := runWatchdog(t, time.Second, 60*time.Second, sink, nil)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", r.err)
	}
	ev, ok := findEvent(sink.snapshot(), "spawn.no_output")
	if !ok {
		t.Fatalf("want a spawn.no_output event, got %+v", sink.snapshot())
	}
	if want := "no stdout output for " + time.Second.String(); ev.Message != want {
		t.Errorf("want message %q, got %q", want, ev.Message)
	}
	var after string
	for _, f := range ev.Fields {
		if f.Key == "after" {
			after = f.Value
		}
	}
	if after != time.Second.String() {
		t.Errorf("want field after=%s, got %q", time.Second, after)
	}
}

func TestRun_NoOutput_FiresAfterOutputWentSilent(t *testing.T) {
	setHelperEnv(t, "stream-then-hang")

	r := runWatchdog(t, time.Second, 60*time.Second, nil, nil)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput once output stopped, got %v", r.err)
	}
}

func TestRun_NoOutput_EndsWholeProcessTree(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)

	r := runWatchdog(t, time.Second, 60*time.Second, nil, nil)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", r.err)
	}
	requireGone(t, waitForPid(t, pidFile))
}

// ---------------------------------------------------------------------------
// Silence is reset by stdout data only
// ---------------------------------------------------------------------------

func TestRun_NoOutput_StdoutDataResetsSilence(t *testing.T) {
	cases := map[string]func([]byte){
		"without observer": nil,
		"with observer":    func([]byte) {},
	}
	for name, observer := range cases {
		t.Run(name, func(t *testing.T) {
			setHelperEnv(t, "wd-drip-then-hang")
			// Lines at 0, 0.4, 0.8, 1.2 s, then silence. Silence is measured
			// from the last line, so the watchdog fires at about 2.2 s. A
			// watchdog measuring from start would fire at 1 s.
			setDrip(t, 4, 400*time.Millisecond)

			r := runWatchdog(t, time.Second, 60*time.Second, nil, observer)

			if !errors.Is(r.err, harness.ErrNoOutput) {
				t.Fatalf("want ErrNoOutput, got %v", r.err)
			}
			if r.took < 2*time.Second {
				t.Errorf("want the silence measured from the last output (about 2.2s), but Run ended after %s", r.took)
			}
		})
	}
}

func TestRun_NoOutput_StderrDoesNotResetSilence(t *testing.T) {
	setHelperEnv(t, "wd-stderr-drip")

	r := runWatchdog(t, time.Second, 60*time.Second, nil, nil)

	if !errors.Is(r.err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput despite steady stderr output, got %v", r.err)
	}
	if r.took > 6*time.Second {
		t.Errorf("want the watchdog to end the run near 1s, took %s", r.took)
	}
}

func TestRun_NoOutput_ContinuousOutputIsNotInterrupted(t *testing.T) {
	// regression + guard: output arrives every 400 ms for 2.4 s in total, each
	// gap well below the 1 s watchdog.
	setHelperEnv(t, "wd-drip")
	setDrip(t, 6, 400*time.Millisecond)

	r := runWatchdog(t, time.Second, 20*time.Second, nil, nil)

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if got := strings.Count(string(r.resp.Stdout), "tick-"); got != 6 {
		t.Errorf("want all 6 lines, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Disabled
// ---------------------------------------------------------------------------

func TestRun_NoOutput_ZeroOrNegativeDisablesWatchdog(t *testing.T) {
	// regression: a silence of 1.5 s must not end a run without a watchdog.
	for name, d := range map[string]time.Duration{"zero": 0, "negative": -time.Second} {
		t.Run(name, func(t *testing.T) {
			setHelperEnv(t, "wd-silent-then-line")
			setSilence(t, 1500*time.Millisecond)

			r := runWatchdog(t, d, 20*time.Second, nil, nil)

			if r.err != nil {
				t.Fatalf("unexpected error: %v", r.err)
			}
			if string(r.resp.Stdout) != "done\n" {
				t.Errorf("want stdout %q, got %q", "done\n", r.resp.Stdout)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Races with the other ways a run ends
// ---------------------------------------------------------------------------

func TestRun_NoOutput_SilentExitBeforeDeadlineIsNotAWatchdogEnd(t *testing.T) {
	setHelperEnv(t, "wd-silent-exit")
	setSilence(t, 700*time.Millisecond)

	r := runWatchdog(t, 3*time.Second, 20*time.Second, nil, nil)

	if r.err != nil {
		t.Fatalf("want a normal exit, got %v", r.err)
	}
	if r.resp.ExitCode != 0 {
		t.Errorf("want exit code 0, got %d", r.resp.ExitCode)
	}
}

func TestRun_NoOutput_OutputBeforeDeadlineKeepsRunAlive(t *testing.T) {
	// regression + guard: the line arrives 1 s into a 2.5 s watchdog.
	setHelperEnv(t, "wd-silent-then-line")
	setSilence(t, time.Second)

	r := runWatchdog(t, 2500*time.Millisecond, 20*time.Second, nil, nil)

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if string(r.resp.Stdout) != "done\n" {
		t.Errorf("want stdout %q, got %q", "done\n", r.resp.Stdout)
	}
}

func TestRun_NoOutput_WallClockTimeoutFirstIsNotAWatchdogEnd(t *testing.T) {
	setHelperEnv(t, "hang")

	r := runWatchdog(t, 30*time.Second, time.Second, nil, nil)

	if !errors.Is(r.err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", r.err)
	}
	if errors.Is(r.err, harness.ErrNoOutput) {
		t.Errorf("the wall-clock timeout came first; want no ErrNoOutput, got %v", r.err)
	}
}

func TestRun_NoOutput_CancellationFirstIsNotAWatchdogEnd(t *testing.T) {
	setHelperEnv(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()
	defer cancel()

	r := runBounded(t, ctx, harness.RunOptions{NoOutputTimeout: 30 * time.Second, Timeout: 60 * time.Second})

	if !errors.Is(r.err, harness.ErrCancelled) {
		t.Fatalf("want ErrCancelled, got %v", r.err)
	}
	if errors.Is(r.err, harness.ErrNoOutput) {
		t.Errorf("cancellation came first; want no ErrNoOutput, got %v", r.err)
	}
}
