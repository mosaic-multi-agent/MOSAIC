package claudecode_test

// Tests for what the Claude Code adapter leaves in the debug log while the
// process runs and when it ends: the subagent's stdout lines attributed to the
// invocation, the stdout byte count at the end, no whole-stdout entry, and the
// pre-kill stdout, stderr and end reason when the process is killed.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/claudecode"
	"mosaic-run/internal/testutil"
)

func init() {
	helperScenarios["lines-then-hang"] = func() {
		os.Stderr.WriteString("err-before-kill\n") //nolint:errcheck
		time.Sleep(300 * time.Millisecond)
		writeStreamLines("q1", "q2")
		time.Sleep(24 * time.Hour)
	}
}

func TestClaudeCodeAdapter_LiveLog_InvokeLogsAttributedLinesAndByteCountButNoWholeStdout(t *testing.T) {
	setHelperEnv(t, "stream-envelope")
	logger := &testutil.RecordingLogger{}
	adapter := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 5*time.Second, logger)

	if _, err := adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logger.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "test-agent#1")
	lines := testutil.LiveLines(t, entries, "test-agent#1")
	if len(lines) != 5 || !strings.Contains(lines[0], `"subtype":"init"`) || !strings.Contains(lines[4], `"type":"result"`) {
		t.Errorf("want the 5 transcript lines once each in order, got %q", lines)
	}
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "test-agent#1", "exit", len(strings.Join(lines, "\n"))+1)
}

func TestClaudeCodeAdapter_LiveLog_InvokeRawLogsLinesUnderTheConsultedAgentAndNoParsedText(t *testing.T) {
	setHelperEnv(t, "stream-plain-text")
	logger := &testutil.RecordingLogger{}
	adapter := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 5*time.Second, logger)

	if _, err := adapter.InvokeRaw(context.Background(), orchestratorAgentRefCC(t), []byte("consult")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := logger.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "orchestrator-agent")
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "orchestrator-agent", "exit", -1)
	if len(testutil.Named(entries, domain.EventHarnessInvokeOK)) != 1 {
		t.Errorf("want the invoke-ok entry kept, got events %v", logger.Events())
	}
}

func TestClaudeCodeAdapter_LiveLog_TimeoutKeepsPreKillStdoutStderrAndReason(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	logger := &testutil.RecordingLogger{}
	adapter := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 3*time.Second, logger)

	_, _ = adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))

	testutil.AssertKillTrail(t, logger.Entries(), "test-agent#1", "timeout", []string{"q1", "q2"}, "err-before-kill")
}

func TestClaudeCodeAdapter_LiveLog_CancellationKeepsPreKillStdoutStderrAndReason(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	logger := &testutil.RecordingLogger{}
	adapter := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 30*time.Second, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		testutil.WaitForLine(t, logger, "q2", 5*time.Second)
		cancel()
	}()

	_, _ = adapter.Invoke(ctx, ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))

	testutil.AssertKillTrail(t, logger.Entries(), "test-agent#1", "cancelled", []string{"q1", "q2"}, "err-before-kill")
}

func TestClaudeCodeAdapter_LiveLog_LinesAppearWhileTheProcessStillRuns(t *testing.T) {
	setHelperEnv(t, "lines-then-hang")
	logger := &testutil.RecordingLogger{}
	adapter := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 30*time.Second, logger)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = adapter.Invoke(ctx, ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
	}()

	testutil.WaitForLine(t, logger, "q1", 10*time.Second)
	select {
	case <-done:
		t.Fatalf("the invocation returned although the process was still running")
	default:
	}
	if len(testutil.Named(logger.Entries(), domain.EventHarnessProcessEnd)) != 0 {
		t.Errorf("want no end entry while the process runs")
	}
	cancel()
	<-done
}
