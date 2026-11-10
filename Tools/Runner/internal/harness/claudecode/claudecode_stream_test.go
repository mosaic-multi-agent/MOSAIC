package claudecode_test

// Tests for the Claude Code adapter against the stream-json transcript:
// response extraction from the last result line, error handling, the raw
// text path and debug-log events. Uses the fake-CLI helper process.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/claudecode"
	"mosaic-run/internal/harness/cliexec"
)

func invokeStream(t *testing.T, scenario string, logger domain.DebugLogger) (domain.ProtocolResponse, error) {
	t.Helper()
	setHelperEnv(t, scenario)
	adapter := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 5*time.Second, logger)
	return adapter.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
}

func invokeRawStream(t *testing.T, scenario string, logger domain.DebugLogger) ([]byte, error) {
	t.Helper()
	setHelperEnv(t, scenario)
	adapter := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 5*time.Second, logger)
	return adapter.InvokeRaw(context.Background(), orchestratorAgentRefCC(t), []byte("consult"))
}

func TestClaudeCodeAdapter_Stream_Invoke_ExtractsProtocolFromResultLine(t *testing.T) {
	resp, err := invokeStream(t, "success", nil)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if resp.StatusCode != domain.StatusSUCCESS || resp.AgentInstanceID != "test-agent#1" {
		t.Errorf("want SUCCESS from test-agent#1, got %q from %q", resp.StatusCode, resp.AgentInstanceID)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_LastResultLineWins(t *testing.T) {
	resp, err := invokeStream(t, "stream-multi-result", nil)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if resp.StatusCode != domain.StatusSUCCESS {
		t.Errorf("want SUCCESS, got %q", resp.StatusCode)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_FencedResultText_Extracted(t *testing.T) {
	resp, err := invokeStream(t, "stream-fenced", nil)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if resp.AgentInstanceID != "test-agent#1" {
		t.Errorf("want AgentInstanceID test-agent#1, got %q", resp.AgentInstanceID)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_ProtocolOnlyInAssistantMessage_NotUsed(t *testing.T) {
	_, err := invokeStream(t, "stream-protocol-in-assistant-only", nil)
	if !errors.Is(err, cliexec.ErrMalformedOutput) {
		t.Errorf("want ErrMalformedOutput (result line has no protocol), got %v", err)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_NoResultLine_ReturnsErrMalformedOutput(t *testing.T) {
	_, err := invokeStream(t, "bad-envelope", nil)
	if !errors.Is(err, cliexec.ErrMalformedOutput) {
		t.Errorf("want ErrMalformedOutput, got %v", err)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_ErrorSubtypeWithoutText_ReturnsErrMalformedOutput(t *testing.T) {
	_, err := invokeStream(t, "stream-error-max-turns", nil)
	if !errors.Is(err, cliexec.ErrMalformedOutput) {
		t.Errorf("want ErrMalformedOutput, got %v", err)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_IsErrorResultText_ReturnsErrMalformedOutput(t *testing.T) {
	_, err := invokeStream(t, "stream-is-error", nil)
	if !errors.Is(err, cliexec.ErrMalformedOutput) {
		t.Errorf("want ErrMalformedOutput, got %v", err)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_EmptyOutput_ReturnsErrEmptyResponse(t *testing.T) {
	_, err := invokeStream(t, "empty", nil)
	if !errors.Is(err, cliexec.ErrEmptyResponse) {
		t.Errorf("want ErrEmptyResponse, got %v", err)
	}
}

func TestClaudeCodeAdapter_Stream_Invoke_GarbageOutput_ReturnsErrMalformedJSON(t *testing.T) {
	_, err := invokeStream(t, "bad-json", nil)
	if !errors.Is(err, cliexec.ErrMalformedJSON) {
		t.Errorf("want ErrMalformedJSON, got %v", err)
	}
}

func TestClaudeCodeAdapter_Stream_Logging_NormalTranscript_NoRecoveredNoFailed(t *testing.T) {
	logger := &recordingLogger{}
	if _, err := invokeStream(t, "stream-envelope", logger); err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	for _, ev := range []string{domain.EventHarnessParseRecovered, domain.EventHarnessParseFailed} {
		if logger.eventLogged(ev) {
			t.Errorf("want no %s for a normal transcript, got events: %v", ev, logger.allEvents())
		}
	}
}

func TestClaudeCodeAdapter_Stream_Logging_RawStdoutIsFullTranscript(t *testing.T) {
	logger := &recordingLogger{}
	if _, err := invokeStream(t, "stream-envelope", logger); err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	msg, ok := logger.messageFor(domain.EventHarnessStdout)
	if !ok {
		t.Fatalf("want a %s event, got %v", domain.EventHarnessStdout, logger.allEvents())
	}
	if !strings.Contains(msg, `"subtype":"init"`) || !strings.Contains(msg, `"type":"result"`) {
		t.Errorf("want logged stdout to contain the whole transcript, got %q", msg)
	}
}

func TestClaudeCodeAdapter_Stream_Logging_NoResultLine_LogsParseFailed(t *testing.T) {
	logger := &recordingLogger{}
	if _, err := invokeStream(t, "bad-envelope", logger); err == nil {
		t.Fatal("want error, got nil")
	}
	if got := logger.countEvent(domain.EventHarnessParseFailed); got != 1 {
		t.Errorf("want one %s, got %d: %v", domain.EventHarnessParseFailed, got, logger.allEvents())
	}
}

func TestClaudeCodeAdapter_Stream_Logging_BareProtocolStillLogsParseRecovered(t *testing.T) {
	logger := &recordingLogger{}
	if _, err := invokeStream(t, "bare-json", logger); err != nil {
		t.Fatalf("want recovery without error, got %v", err)
	}
	if !logger.eventLogged(domain.EventHarnessParseRecovered) {
		t.Errorf("want %s for non-transcript stdout, got %v", domain.EventHarnessParseRecovered, logger.allEvents())
	}
}

func TestClaudeCodeAdapter_Stream_Logging_NoiseWithEmbeddedProtocolStillLogsParseRecovered(t *testing.T) {
	logger := &recordingLogger{}
	if _, err := invokeStream(t, "embedded-json", logger); err != nil {
		t.Fatalf("want recovery without error, got %v", err)
	}
	if !logger.eventLogged(domain.EventHarnessParseRecovered) {
		t.Errorf("want %s, got %v", domain.EventHarnessParseRecovered, logger.allEvents())
	}
}

func TestClaudeCodeAdapter_Stream_InvokeRaw_UsesStreamJSONArgs(t *testing.T) {
	argsFile := setHelperEnv(t, "stream-plain-text")
	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 5*time.Second)
	if _, err := adapter.InvokeRaw(context.Background(), orchestratorAgentRefCC(t), []byte("consult")); err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	args := readArgs(t, argsFile)
	if !containsSequence(args, "--output-format", "stream-json") || !containsArg(args, "--verbose") {
		t.Errorf("want --output-format stream-json and --verbose, got %v", args)
	}
}

func TestClaudeCodeAdapter_Stream_InvokeRaw_ReturnsResultLineTextOnly(t *testing.T) {
	got, err := invokeRawStream(t, "stream-plain-text", nil)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if string(got) != "plain consultation reply, no protocol JSON" {
		t.Errorf("want only the result line text, got %q", got)
	}
}

func TestClaudeCodeAdapter_Stream_InvokeRaw_LastResultLineWins(t *testing.T) {
	got, err := invokeRawStream(t, "stream-multi-result", nil)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if string(got) != streamProtocolOK {
		t.Errorf("want the last result line text %q, got %q", streamProtocolOK, got)
	}
}

func TestClaudeCodeAdapter_Stream_InvokeRaw_IsErrorText_ReturnedAsText(t *testing.T) {
	got, err := invokeRawStream(t, "stream-is-error", nil)
	if err != nil {
		t.Fatalf("want no error (text path does not interpret the text), got %v", err)
	}
	if string(got) != "Invalid API key - please run /login" {
		t.Errorf("want result text verbatim, got %q", got)
	}
}

func TestClaudeCodeAdapter_Stream_InvokeRaw_NoResultLine_ReturnsErrorAndLogsInvokeError(t *testing.T) {
	logger := &recordingLogger{}
	if _, err := invokeRawStream(t, "bad-envelope", logger); err == nil {
		t.Fatal("want error for a transcript without result line, got nil")
	}
	if !logger.eventLogged(domain.EventHarnessInvokeError) {
		t.Errorf("want %s, got %v", domain.EventHarnessInvokeError, logger.allEvents())
	}
}

func TestClaudeCodeAdapter_Stream_InvokeRaw_LogsInvokeOK(t *testing.T) {
	logger := &recordingLogger{}
	if _, err := invokeRawStream(t, "stream-plain-text", logger); err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if !logger.eventLogged(domain.EventHarnessInvokeOK) {
		t.Errorf("want %s, got %v", domain.EventHarnessInvokeOK, logger.allEvents())
	}
}
