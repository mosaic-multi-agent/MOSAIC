package ghcpcli_test

// Tests that the GHCP CLI adapter logs the subagent's stdout lines attributed
// to the invocation (the instance id for a dispatch, the agent identifier for a
// consultation), the stdout byte count, and no whole-stdout or parsed-text
// entry.

import (
	"context"
	"testing"
	"time"

	"mosaic-run/internal/harness/ghcpcli"
	"mosaic-run/internal/testutil"
)

func TestGHCPCLIAdapter_LiveLog_InvokeLogsAttributedLinesAndByteCountButNoWholeStdout(t *testing.T) {
	setHelperEnv(t, "ghcpcli-success")
	lg := &testutil.RecordingLogger{}
	adapter := ghcpcli.NewGHCPCLIAdapterWithLogger(helperExe(t), 5*time.Second, lg)

	if _, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := lg.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "test-agent#1")
	if lines := testutil.LiveLines(t, entries, "test-agent#1"); len(lines) == 0 {
		t.Errorf("want the stdout lines in the log")
	}
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "test-agent#1", "exit", -1)
}

func TestGHCPCLIAdapter_LiveLog_InvokeRawLogsLinesUnderTheConsultedAgentAndNoParsedText(t *testing.T) {
	setHelperEnv(t, "ghcpcli-success")
	lg := &testutil.RecordingLogger{}
	adapter := ghcpcli.NewGHCPCLIAdapterWithLogger(helperExe(t), 5*time.Second, lg)

	if _, err := adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := lg.Entries()
	testutil.AssertEveryLiveEntryAttributed(t, entries, "orchestrator-agent")
	testutil.AssertNoWholeStdoutEntry(t, entries)
	testutil.AssertProcessEnd(t, entries, "orchestrator-agent", "exit", -1)
}
