package harness_test

// Tests for ParseClaudeCodeEnvelope with the --output-format stream-json
// form: a JSONL transcript whose last "result" line carries the final text.
// Fixtures live in testdata/. The json (array/object) form is covered in
// envelope_test.go and must keep working unchanged.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/harness"
)

const streamProtocolText = `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`

func readStreamFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestParseClaudeCodeEnvelope_Stream_ReturnsResultLineText(t *testing.T) {
	text, err := harness.ParseClaudeCodeEnvelope(readStreamFixture(t, "claude_stream_success.jsonl"))
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if text != streamProtocolText {
		t.Errorf("want result text %q, got %q", streamProtocolText, text)
	}
	if _, exErr := harness.ExtractProtocolJSON(text); exErr != nil {
		t.Errorf("want returned text extractable as protocol JSON, got %v", exErr)
	}
}

func TestParseClaudeCodeEnvelope_Stream_MultipleResultLines_LastWins(t *testing.T) {
	text, err := harness.ParseClaudeCodeEnvelope(readStreamFixture(t, "claude_stream_multiple_results.jsonl"))
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if !strings.Contains(text, `"status_message":"last"`) {
		t.Errorf("want text of the last result line, got %q", text)
	}
}

func TestParseClaudeCodeEnvelope_Stream_NoResultLine_ReturnsErrProtocolNotExtractable(t *testing.T) {
	_, err := harness.ParseClaudeCodeEnvelope(readStreamFixture(t, "claude_stream_no_result.jsonl"))
	if !errors.Is(err, harness.ErrProtocolNotExtractable) {
		t.Errorf("want ErrProtocolNotExtractable, got %v", err)
	}
}

func TestParseClaudeCodeEnvelope_Stream_ErrorSubtypeWithoutResultField_ReturnsErrProtocolNotExtractable(t *testing.T) {
	// Same class as the json object form with no result field.
	_, err := harness.ParseClaudeCodeEnvelope(readStreamFixture(t, "claude_stream_error_max_turns.jsonl"))
	if !errors.Is(err, harness.ErrProtocolNotExtractable) {
		t.Errorf("want ErrProtocolNotExtractable, got %v", err)
	}
}

func TestParseClaudeCodeEnvelope_Stream_IsErrorWithText_ReturnsTextVerbatim(t *testing.T) {
	// The json form returns the result text regardless of is_error; the
	// stream form must behave the same.
	text, err := harness.ParseClaudeCodeEnvelope(readStreamFixture(t, "claude_stream_is_error_with_text.jsonl"))
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if text != "Invalid API key - please run /login" {
		t.Errorf("want the result text verbatim, got %q", text)
	}
}

func TestParseClaudeCodeEnvelope_Stream_CRLFAndBlankLines_Tolerated(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"init"}` + "\r\n\r\n" +
		`{"type":"result","subtype":"success","is_error":false,"result":` + jsonQuote(streamProtocolText) + `}` + "\r\n\r\n")
	text, err := harness.ParseClaudeCodeEnvelope(data)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if text != streamProtocolText {
		t.Errorf("want %q, got %q", streamProtocolText, text)
	}
}

func TestParseClaudeCodeEnvelope_Stream_NoTrailingNewline_Accepted(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"init"}` + "\n" +
		`{"type":"result","subtype":"success","result":` + jsonQuote(streamProtocolText) + `}`)
	text, err := harness.ParseClaudeCodeEnvelope(data)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if text != streamProtocolText {
		t.Errorf("want %q, got %q", streamProtocolText, text)
	}
}

func TestParseClaudeCodeEnvelope_Stream_ResultNotLast_StillTaken(t *testing.T) {
	data := []byte(`{"type":"result","subtype":"success","result":` + jsonQuote(streamProtocolText) + `}` + "\n" +
		`{"type":"system","subtype":"hook_response"}` + "\n")
	text, err := harness.ParseClaudeCodeEnvelope(data)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if text != streamProtocolText {
		t.Errorf("want %q, got %q", streamProtocolText, text)
	}
}

func TestParseClaudeCodeEnvelope_Stream_EmptyAndWhitespace_ReturnErrEmptyResponse(t *testing.T) {
	for _, in := range []string{"", "\n\n", "  \r\n \n"} {
		if _, err := harness.ParseClaudeCodeEnvelope([]byte(in)); !errors.Is(err, harness.ErrEmptyResponse) {
			t.Errorf("input %q: want ErrEmptyResponse, got %v", in, err)
		}
	}
}

func TestParseClaudeCodeEnvelope_Stream_NonJSONLines_ReturnErrMalformedJSON(t *testing.T) {
	_, err := harness.ParseClaudeCodeEnvelope([]byte("this is not JSON\nneither is this\n"))
	if !errors.Is(err, harness.ErrMalformedJSON) {
		t.Errorf("want ErrMalformedJSON, got %v", err)
	}
}

func TestParseClaudeCodeEnvelope_Stream_TruncatedResultLine_ReturnsErrorNotText(t *testing.T) {
	// A transcript cut off mid result line has no usable result: it must fail
	// with one of the existing error classes, never return partial text.
	data := []byte(`{"type":"system","subtype":"init"}` + "\n" +
		`{"type":"result","subtype":"success","result":"{\"agent_inst`)
	text, err := harness.ParseClaudeCodeEnvelope(data)
	if err == nil {
		t.Fatalf("want an error, got text %q", text)
	}
	if !errors.Is(err, harness.ErrProtocolNotExtractable) && !errors.Is(err, harness.ErrMalformedJSON) {
		t.Errorf("want ErrProtocolNotExtractable or ErrMalformedJSON, got %v", err)
	}
}

func TestParseClaudeCodeEnvelope_Stream_ErrorMessageDoesNotLeakGoTypeNames(t *testing.T) {
	_, err := harness.ParseClaudeCodeEnvelope([]byte("garbage line one\ngarbage line two\n"))
	if err == nil {
		t.Fatal("want error, got nil")
	}
	for _, leak := range []string{"cliEnvelope", "json: cannot unmarshal"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error leaks %q: %q", leak, err.Error())
		}
	}
}
