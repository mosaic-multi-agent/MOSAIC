package codex_test

// Tests for what the Codex adapter leaves in the debug log while the process
// runs and when it ends: the stdout lines attributed to the invocation (the
// instance id for a dispatch, the agent identifier for a consultation), the
// stdout byte count, no whole-stdout or parsed-text entry, and the pre-kill
// stdout, stderr and end reason on a timeout.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/testutil"
)

func init() {
	helperScenarios["lines-then-hang"] = func() {
		os.Stderr.WriteString("err-before-kill\n") //nolint:errcheck
		time.Sleep(300 * time.Millisecond)
		testutil.EmitLines("q1", "q2")
		time.Sleep(24 * time.Hour)
	}
}

func TestCodexAdapter_LiveLog_InvokeLogsAttributedLinesAndByteCountButNoWholeStdout(t *testing.T) {
	setHelperEnv(t, "codex-success")
	lg := &testutil.RecordingLogger{}

	if _, err := loggedAdapter(t, lg).Invoke(context.Background(), agentRef(t), request("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := lg.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "test-agent#1")
	lines := testutil.LiveLines(t, entries, "test-agent#1")
	if len(lines) != 4 || !strings.Contains(lines[0], "thread.started") || !strings.Contains(lines[3], "turn.completed") {
		t.Errorf("want the 4 event lines once each in order, got %q", lines)
	}
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "test-agent#1", "exit", len(strings.Join(lines, "\n"))+1)
}

func TestCodexAdapter_LiveLog_InvokeRawLogsLinesUnderTheConsultedAgentAndNoParsedText(t *testing.T) {
	setHelperEnv(t, "codex-raw-success")
	lg := &testutil.RecordingLogger{}

	if _, err := loggedAdapter(t, lg).InvokeRaw(context.Background(), agentRef(t), rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := lg.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "test-agent")
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "test-agent", "exit", -1)
}

func TestCodexAdapter_LiveLog_TimeoutKeepsPreKillStdoutStderrAndReason(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	lg := &testutil.RecordingLogger{}

	_, _ = loggedAdapterWithTimeout(t, lg, 3*time.Second).Invoke(context.Background(), agentRef(t), request("test-agent#1"))

	testutil.AssertKillTrail(t, lg.Entries(), "test-agent#1", "timeout", []string{"q1", "q2"}, "err-before-kill")
}

func TestCodexAdapter_LiveLog_RawConsultationCancellationKeepsPreKillStdoutStderrAndReason(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	lg := &testutil.RecordingLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		testutil.WaitForLine(t, lg, "q2", 5*time.Second)
		cancel()
	}()

	_, _ = loggedAdapterWithTimeout(t, lg, 30*time.Second).InvokeRaw(ctx, agentRef(t), rawPayload)

	testutil.AssertKillTrail(t, lg.Entries(), "test-agent", "cancelled", []string{"q1", "q2"}, "err-before-kill")
}
