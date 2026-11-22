package opencodev2_test

// Tests for what the OpenCode V2 adapter leaves in the debug log: the stdout
// lines attributed to the invocation, the stdout byte count, no whole-stdout or
// parsed-text entry, and, when the process is killed or fails, the pre-kill
// stdout, stderr and end reason while the failure itself is still reported
// exactly once with the adapter's own entry.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/opencodev2"
	"mosaic-run/internal/testutil"
)

func init() {
	helperScenarios["lines-then-hang"] = func() {
		os.Stderr.WriteString("err-before-kill\n") //nolint:errcheck
		time.Sleep(300 * time.Millisecond)
		emit("q1", "q2")
		time.Sleep(24 * time.Hour)
	}
}

func loggedAdapter(t *testing.T, timeout time.Duration) (*opencodev2.OpenCodeV2Adapter, *testutil.RecordingLogger) {
	t.Helper()
	lg := &testutil.RecordingLogger{}
	return opencodev2.NewOpenCodeV2AdapterWithLogger(helperExe(t), timeout, lg), lg
}

func TestInvoke_LiveLog_LogsAttributedLinesAndByteCountButNoWholeStdout(t *testing.T) {
	setHelperEnv(t, "success")
	adapter, lg := loggedAdapter(t, 10*time.Second)

	if _, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := lg.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "test-agent#1")
	lines := testutil.LiveLines(t, entries, "test-agent#1")
	if len(lines) != 3 || !strings.Contains(lines[0], "step_start") || !strings.Contains(lines[2], "step_finish") {
		t.Errorf("want the 3 stream lines once each in order, got %q", lines)
	}
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "test-agent#1", "exit", len(strings.Join(lines, "\n"))+1)
}

func TestInvokeRaw_LiveLog_LogsLinesUnderTheConsultedAgentAndNoParsedText(t *testing.T) {
	setHelperEnv(t, "raw-text")
	adapter, lg := loggedAdapter(t, 10*time.Second)

	if _, err := adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := lg.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "orchestrator-agent")
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "orchestrator-agent", "exit", -1)
	if len(testutil.Named(entries, domain.EventHarnessInvokeOK)) != 1 {
		t.Errorf("want the invoke-ok entry kept, got events %v", lg.Events())
	}
}

func TestInvoke_LiveLog_TimeoutKeepsKillTimeOutputAndReportsTheFailureOnce(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	adapter, lg := loggedAdapter(t, 3*time.Second)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if err == nil {
		t.Fatal("want a timeout failure")
	}
	entries := lg.Entries()
	testutil.AssertKillTrail(t, entries, "test-agent#1", "timeout", []string{"q1", "q2"}, "err-before-kill")
	if n := len(testutil.Named(entries, domain.EventHarnessInvokeError)); n != 1 {
		t.Errorf("want the failure reported once, got %d invoke-error entries in %v", n, lg.Events())
	}
}

func TestInvoke_LiveLog_NoOutputWatchdogKeepsKillTimeOutputAndReportsTheFailureOnce(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	adapter, lg := loggedAdapter(t, 30*time.Second)
	adapter.WithNoOutputTimeout(2 * time.Second)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if err == nil {
		t.Fatal("want a watchdog failure")
	}
	entries := lg.Entries()
	testutil.AssertKillTrail(t, entries, "test-agent#1", "no_output", []string{"q1", "q2"}, "err-before-kill")
	if n := len(testutil.Named(entries, domain.EventHarnessInvokeError)); n != 1 {
		t.Errorf("want the failure reported once, got %d invoke-error entries in %v", n, lg.Events())
	}
}

func TestInvoke_LiveLog_CancellationKeepsKillTimeOutput(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	adapter, lg := loggedAdapter(t, 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		testutil.WaitForLine(t, lg, "q2", 5*time.Second)
		cancel()
	}()

	_, _ = adapter.Invoke(ctx, ordinaryAgentRef(), minimalRequest("test-agent#1"))

	testutil.AssertKillTrail(t, lg.Entries(), "test-agent#1", "cancelled", []string{"q1", "q2"}, "err-before-kill")
}

func TestInvoke_LiveLog_NonZeroExitKeepsLinesAndReportsTheFailureOnce(t *testing.T) {
	setHelperEnv(t, "error-other")
	adapter, lg := loggedAdapter(t, 10*time.Second)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if err == nil {
		t.Fatal("want a failure")
	}
	entries := lg.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "test-agent#1")
	testutil.AssertProcessEnd(t, entries, "test-agent#1", "nonzero_exit", -1)
	if n := len(testutil.Named(entries, domain.EventHarnessInvokeError)); n != 1 {
		t.Errorf("want the failure reported once, got %d invoke-error entries in %v", n, lg.Events())
	}
}
