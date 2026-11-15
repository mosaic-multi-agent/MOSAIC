package harness_test

// Tests for ParseCodexEnvelope: scanning the `codex exec --json` event stream
// and deriving the outcome from the stream's own content. Every row of the
// stream outcome table is one case, driven by JSONL fixtures under
// testdata/codex/. Error kinds are checked by sentinel family, never by wording.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/harness"
)

func codexStream(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "codex", name))
	if err != nil {
		t.Fatalf("read stream fixture: %v", err)
	}
	return data
}

func TestParseCodexEnvelope_CompletedTurn_ReturnsLastAgentMessage(t *testing.T) {
	text, err := harness.ParseCodexEnvelope(codexStream(t, "stream_success.jsonl"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`
	if text != want {
		t.Errorf("want the last completed agent message %q, got %q", want, text)
	}
}

func TestParseCodexEnvelope_CompletedTurn_TextYieldsProtocolJSON(t *testing.T) {
	text, err := harness.ParseCodexEnvelope(codexStream(t, "stream_success.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	protocol, err := harness.ExtractProtocolJSON(text)

	if err != nil {
		t.Fatalf("ExtractProtocolJSON: %v", err)
	}
	if !strings.Contains(string(protocol), `"status_code":"SUCCESS"`) {
		t.Errorf("want the protocol JSON, got %s", protocol)
	}
}

func TestParseCodexEnvelope_SeveralTurns_LastTurnWins(t *testing.T) {
	text, err := harness.ParseCodexEnvelope(codexStream(t, "stream_two_turns.jsonl"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "second turn answer" {
		t.Errorf("want the message of the last completed turn, got %q", text)
	}
}

func TestParseCodexEnvelope_NoiseLinesAndUnknownEvents_AreIgnored(t *testing.T) {
	text, err := harness.ParseCodexEnvelope(codexStream(t, "stream_noise_and_unknown_events.jsonl"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "the answer" {
		t.Errorf("want %q, got %q", "the answer", text)
	}
}

func TestParseCodexEnvelope_ErrorEventFollowedByCompletedTurn_Succeeds(t *testing.T) {
	text, err := harness.ParseCodexEnvelope(codexStream(t, "stream_error_then_completed.jsonl"))

	if err != nil {
		t.Fatalf("a recovered error event must not fail the turn, got %v", err)
	}
	if text != "recovered answer" {
		t.Errorf("want %q, got %q", "recovered answer", text)
	}
}

func TestParseCodexEnvelope_TurnFailed_IsStreamErrorWithMessage(t *testing.T) {
	text, err := harness.ParseCodexEnvelope(codexStream(t, "stream_turn_failed.jsonl"))

	if !errors.Is(err, harness.ErrCodexStreamError) {
		t.Fatalf("want ErrCodexStreamError, got %v", err)
	}
	if !strings.Contains(err.Error(), "quota exhausted for this model") {
		t.Errorf("want the failure message in the error, got %v", err)
	}
	if text != "" {
		t.Errorf("want empty text on error, got %q", text)
	}
}

func TestParseCodexEnvelope_LastTerminalEventIsTurnFailed_IsStreamError(t *testing.T) {
	_, err := harness.ParseCodexEnvelope(codexStream(t, "stream_completed_then_failed.jsonl"))

	if !errors.Is(err, harness.ErrCodexStreamError) {
		t.Fatalf("want ErrCodexStreamError, got %v", err)
	}
	if !strings.Contains(err.Error(), "second turn blew up") {
		t.Errorf("want the failure message in the error, got %v", err)
	}
}

func TestParseCodexEnvelope_TopLevelErrorWithoutCompletion_IsStreamErrorWithMessage(t *testing.T) {
	_, err := harness.ParseCodexEnvelope(codexStream(t, "stream_top_level_error.jsonl"))

	if !errors.Is(err, harness.ErrCodexStreamError) {
		t.Fatalf("want ErrCodexStreamError, got %v", err)
	}
	if !strings.Contains(err.Error(), "stream disconnected before completion") {
		t.Errorf("want the error message in the error, got %v", err)
	}
}

func TestParseCodexEnvelope_NoTerminalEvent_IsStreamIncomplete(t *testing.T) {
	text, err := harness.ParseCodexEnvelope(codexStream(t, "stream_incomplete.jsonl"))

	if !errors.Is(err, harness.ErrCodexStreamIncomplete) {
		t.Fatalf("want ErrCodexStreamIncomplete, got %v", err)
	}
	if errors.Is(err, harness.ErrCodexStreamError) {
		t.Errorf("an incomplete stream is not a stream error, got %v", err)
	}
	if text != "" {
		t.Errorf("want empty text on error, got %q", text)
	}
}

func TestParseCodexEnvelope_CompletedTurnWithoutAgentMessage_IsEmptyResponse(t *testing.T) {
	_, err := harness.ParseCodexEnvelope(codexStream(t, "stream_completed_without_message.jsonl"))

	if !errors.Is(err, harness.ErrEmptyResponse) {
		t.Fatalf("want ErrEmptyResponse, got %v", err)
	}
}

func TestParseCodexEnvelope_EmptyOrNoJSONEvents_IsEmptyResponse(t *testing.T) {
	cases := map[string][]byte{
		"nil":        nil,
		"empty":      {},
		"whitespace": []byte("  \n\t\n"),
		"noise only": codexStream(t, "stream_noise_only.txt"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := harness.ParseCodexEnvelope(data)

			if !errors.Is(err, harness.ErrEmptyResponse) {
				t.Errorf("want ErrEmptyResponse, got %v", err)
			}
		})
	}
}

func TestParseCodexEnvelope_FailureShapes_AreDistinctKinds(t *testing.T) {
	_, failed := harness.ParseCodexEnvelope(codexStream(t, "stream_turn_failed.jsonl"))
	_, incomplete := harness.ParseCodexEnvelope(codexStream(t, "stream_incomplete.jsonl"))

	if errors.Is(failed, harness.ErrCodexStreamIncomplete) {
		t.Errorf("a failed turn must not be reported as incomplete: %v", failed)
	}
	if !errors.Is(incomplete, harness.ErrCodexStreamIncomplete) || errors.Is(incomplete, harness.ErrEmptyResponse) {
		t.Errorf("an incomplete stream must be reported as incomplete only: %v", incomplete)
	}
}
