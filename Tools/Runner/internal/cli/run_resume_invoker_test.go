package cli_test

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/testrun"
	"mosaic-run/internal/testrun/invoker"
)

// capturedInvokerArgs returns the arguments the Runner's test runner builds for
// inv, captured through the invoker's command seam.
func capturedInvokerArgs(t *testing.T, inv testrun.RunInvocation) []string {
	t.Helper()
	var captured []string
	subject := invoker.NewSubprocessRunInvoker(invoker.RunInvokerOptions{
		ExecutablePath: "mosaic-run-under-test",
		WorkingDir:     t.TempDir(),
		Invoke: func(_ context.Context, _ string, _ string, args []string) ([]byte, []byte, int, error) {
			captured = args
			return nil, nil, 0, nil
		},
	})
	// The invoker may fail after the command ran (no run folder is produced);
	// only the arguments it built matter here.
	_, _ = subject.Invoke(context.Background(), inv)
	if len(captured) == 0 {
		t.Fatal("the invoker did not run a command")
	}
	return captured
}

// The test runner builds a new-run argument set; the relaxed resume validation
// must not change what that set means.
func TestTestRunnerInvocationArgs_StillStartANewRun(t *testing.T) {
	args := capturedInvokerArgs(t, testrun.RunInvocation{
		WorkflowID:  "smoke-single",
		Mode:        "auto",
		Harness:     "fake",
		FixturePath: "fixtures/smoke-single",
		Task:        "Test: smoke-single / auto / fake",
	})

	code, sess, _, errOut := runSnapshotCLI(t, args)

	if code != cli.ExitSuccess || !sess.called {
		t.Fatalf("exit code = %d, started = %v for args %v; stderr: %q", code, sess.called, args, errOut)
	}
	if !sess.config.IsNewRun {
		t.Error("config.IsNewRun = false, want the test runner's invocation to start a new run")
	}
	if string(sess.config.WorkflowID) != "smoke-single" {
		t.Errorf("config.WorkflowID = %q, want smoke-single", sess.config.WorkflowID)
	}
	if strings.Contains(sess.atStart, "Recorded settings") {
		t.Errorf("a new run printed a recorded-settings summary:\n%s", sess.atStart)
	}
}
