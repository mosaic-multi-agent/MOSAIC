package codex_test

// Tests that a Codex invocation writes the same kinds of debug log events as
// the OpenCode adapter: invoke start, stdout, stderr, parse failed, invoke
// error and invoke ok.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/codex"
)

func loggedAdapter(t *testing.T, logger domain.DebugLogger) *codex.CodexAdapter {
	t.Helper()
	return codex.NewCodexAdapterWithLogger(helperExe(t), adapterTimeout, logger)
}

func requireLogged(t *testing.T, lg *recordingLogger, events ...string) {
	t.Helper()
	for _, e := range events {
		if !lg.logged(e) {
			t.Errorf("want %s logged, got events %v", e, lg.all())
		}
	}
}

func requireNotLogged(t *testing.T, lg *recordingLogger, events ...string) {
	t.Helper()
	for _, e := range events {
		if lg.logged(e) {
			t.Errorf("did not expect %s, got events %v", e, lg.all())
		}
	}
}

func TestCodexAdapter_Invoke_Success_LogsStartStdoutAndOK(t *testing.T) {
	setHelperEnv(t, "codex-success")
	lg := &recordingLogger{}

	if _, err := loggedAdapter(t, lg).Invoke(context.Background(), agentRef(t), request("test-agent#1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLogged(t, lg, domain.EventHarnessInvokeStart, domain.EventHarnessStdout, domain.EventHarnessInvokeOK)
	requireNotLogged(t, lg, domain.EventHarnessInvokeError, domain.EventHarnessParseFailed)
}

func TestCodexAdapter_InvokeRaw_Success_LogsStartStdoutAndOK(t *testing.T) {
	setHelperEnv(t, "codex-raw-success")
	lg := &recordingLogger{}

	if _, err := loggedAdapter(t, lg).InvokeRaw(context.Background(), agentRef(t), rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLogged(t, lg, domain.EventHarnessInvokeStart, domain.EventHarnessStdout, domain.EventHarnessInvokeOK)
	requireNotLogged(t, lg, domain.EventHarnessInvokeError)
}

func TestCodexAdapter_Invoke_NonZeroExit_LogsStderrAndInvokeError(t *testing.T) {
	setHelperEnv(t, "codex-exit1")
	lg := &recordingLogger{}

	_, _ = loggedAdapter(t, lg).Invoke(context.Background(), agentRef(t), request("test-agent#1"))

	requireLogged(t, lg, domain.EventHarnessInvokeStart, domain.EventHarnessStderr, domain.EventHarnessInvokeError)
	requireNotLogged(t, lg, domain.EventHarnessInvokeOK)
}

func TestCodexAdapter_Invoke_StreamFailures_LogParseFailedAndInvokeError(t *testing.T) {
	for _, scenario := range []string{"codex-turn-failed", "codex-error-event", "codex-incomplete", "codex-unparseable"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			lg := &recordingLogger{}

			_, err := loggedAdapter(t, lg).Invoke(context.Background(), agentRef(t), request("test-agent#1"))

			if err == nil {
				t.Fatal("want an error")
			}
			requireLogged(t, lg, domain.EventHarnessParseFailed, domain.EventHarnessInvokeError)
			requireNotLogged(t, lg, domain.EventHarnessInvokeOK)
		})
	}
}

func TestCodexAdapter_Invoke_MissingExecutable_LogsInvokeError(t *testing.T) {
	setHelperEnv(t, "codex-success")
	lg := &recordingLogger{}
	adapter := codex.NewCodexAdapterWithLogger("/nonexistent/codex-binary", adapterTimeout, lg)

	_, _ = adapter.Invoke(context.Background(), agentRef(t), request("test-agent#1"))

	requireLogged(t, lg, domain.EventHarnessInvokeStart, domain.EventHarnessInvokeError)
	requireNotLogged(t, lg, domain.EventHarnessInvokeOK)
}

func TestCodexAdapter_Invoke_UndecodableAgent_LogsInvokeError(t *testing.T) {
	setHelperEnv(t, "codex-success")
	lg := &recordingLogger{}

	_, _ = loggedAdapter(t, lg).Invoke(context.Background(), agentWithContent(t, "name = \n[broken"), request("test-agent#1"))

	requireLogged(t, lg, domain.EventHarnessInvokeError)
	requireNotLogged(t, lg, domain.EventHarnessInvokeOK)
}

func TestCodexAdapter_InvokeRaw_Failures_LogInvokeError(t *testing.T) {
	for _, scenario := range []string{"codex-exit1", "codex-turn-failed", "codex-incomplete", "codex-empty"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			lg := &recordingLogger{}

			_, err := loggedAdapter(t, lg).InvokeRaw(context.Background(), agentRef(t), rawPayload)

			if err == nil {
				t.Fatal("want an error")
			}
			requireLogged(t, lg, domain.EventHarnessInvokeStart, domain.EventHarnessInvokeError)
			requireNotLogged(t, lg, domain.EventHarnessInvokeOK)
		})
	}
}

func TestCodexAdapter_NilLogger_IsAccepted(t *testing.T) {
	setHelperEnv(t, "codex-success")

	if _, err := loggedAdapter(t, nil).Invoke(context.Background(), agentRef(t), request("test-agent#1")); err != nil {
		t.Fatalf("a nil logger must behave as no logging, got %v", err)
	}
}
