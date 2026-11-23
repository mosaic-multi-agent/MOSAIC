package harness_test

// Tests what happens when a caller-supplied Sink panics while Run delivers
// stdout lines to it on the harness's background goroutine.
//
// A panic that ends the process cannot be observed in-process, so each test
// re-executes this test binary as a child (the init below intercepts it before
// TestMain). The child runs harness.Run with a sink that panics on the first
// stdout line, either with the shared fatal reporter installed or without.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-common/fatal"
	"mosaic-common/harness"
)

const (
	sinkPanicChildEnv   = "GO_SINK_PANIC_CHILD" // "wired" or "unwired"
	sinkPanicFailureEnv = "GO_SINK_PANIC_FAILURE_DIR"
	sinkPanicMessage    = "sink exploded"
	sinkPanicWait       = 60 * time.Second
)

// panicSink panics on the first stdout line it receives.
type panicSink struct{}

func (panicSink) Log(ev harness.Event) {
	if ev.Name == "spawn.stdout.lines" {
		panic(sinkPanicMessage)
	}
}

func init() {
	mode := os.Getenv(sinkPanicChildEnv)
	if mode == "" || os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		return
	}
	runSinkPanicChild(mode)
}

// runSinkPanicChild is the child process: it never returns.
func runSinkPanicChild(mode string) {
	if mode == "wired" {
		fatal.Install(fatal.Config{
			Tool:       "mosaic-deploy",
			Version:    "0.0.0-test",
			Mode:       fatal.ModeCLI,
			FailureDir: func() (string, error) { return os.Getenv(sinkPanicFailureEnv), nil },
		})
	}
	os.Setenv("GO_WANT_HELPER_PROCESS", "1")    //nolint:errcheck
	os.Setenv("GO_HELPER_CMD", "embedded-json") //nolint:errcheck
	_, _ = harness.Run(context.Background(), harness.Command{Path: os.Args[0]}, nil,
		harness.RunOptions{Timeout: 30 * time.Second, Sink: panicSink{}})
	os.Stderr.WriteString("child: Run returned\n") //nolint:errcheck
	os.Exit(0)
}

// runSinkPanicChildProcess runs the child and returns its stderr and exit code.
func runSinkPanicChildProcess(t *testing.T, mode, failureDir string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), sinkPanicWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), sinkPanicChildEnv+"="+mode, sinkPanicFailureEnv+"="+failureDir)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("child did not end after the sink panic (hung)")
	}
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("start child: %v", err)
	}
	return stderr.String(), code
}

func TestRun_SinkPanic_WithReporterInstalled_IsReportedWithMessageAndStack(t *testing.T) {
	dir := t.TempDir()

	stderr, code := runSinkPanicChildProcess(t, "wired", dir)

	if code != 1 {
		t.Errorf("want exit code 1 from the reporter, got %d; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "panic: "+sinkPanicMessage) {
		t.Errorf("want the panic message in the report, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "goroutine ") {
		t.Errorf("want a stack trace in the report, got:\n%s", stderr)
	}
}

func TestRun_SinkPanic_WithReporterInstalled_WritesFailureFile(t *testing.T) {
	dir := t.TempDir()

	runSinkPanicChildProcess(t, "wired", dir)

	files, err := filepath.Glob(filepath.Join(dir, "mosaic-*-failure-*.txt"))
	if err != nil || len(files) != 1 {
		t.Fatalf("want exactly one failure file, got %v (err %v)", files, err)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read failure file: %v", err)
	}
	text := string(b)
	if !strings.Contains(text, "Kind: panic") || !strings.Contains(text, sinkPanicMessage) {
		t.Errorf("want a panic-kind failure file naming the panic, got:\n%s", text)
	}
}

func TestRun_SinkPanic_WithoutReporter_EndsWithGoStandardPanicOutput(t *testing.T) {
	dir := t.TempDir()

	stderr, code := runSinkPanicChildProcess(t, "unwired", dir)

	if code != 2 {
		t.Errorf("want Go's standard panic exit code 2, got %d; stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "panic: "+sinkPanicMessage) || !strings.Contains(stderr, "goroutine ") {
		t.Errorf("want Go's standard panic message and stack on stderr, got:\n%s", stderr)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "mosaic-*-failure-*.txt")); len(files) != 0 {
		t.Errorf("want no failure file without a reporter, got %v", files)
	}
}
