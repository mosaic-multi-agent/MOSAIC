package harness_test

// Tests for ParseOpenCodeV2Stream against the recorded OpenCode V2 streams in
// testdata/opencodev2.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/harness"
)

func loadV2Fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "opencodev2", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func parseV2Fixture(t *testing.T, name string) harness.OpenCodeV2Stream {
	t.Helper()
	got, err := harness.ParseOpenCodeV2Stream(loadV2Fixture(t, name))
	if err != nil {
		t.Fatalf("ParseOpenCodeV2Stream(%s): unexpected error: %v", name, err)
	}
	return got
}

// ---------------------------------------------------------------------------
// Final text: single-turn and multi-step streams
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_Fixture_MultiStepStreamYieldsLastStepText(t *testing.T) {
	got := parseV2Fixture(t, "r1-denyshell.jsonl")

	want := "Step 1: Could not run `echo SHELL-RAN`; no shell command tool is available.\n\n" +
		"Step 2: `Success. Updated the following files:\nA note.txt`\n\n" +
		"Step 3: `Read file README.md, lines 1-1\n1: # spike`"
	if !got.Complete {
		t.Error("want Complete for a stream ending in a stop")
	}
	if got.FinalText != want {
		t.Errorf("want FinalText %q, got %q", want, got.FinalText)
	}
	if got.Error != nil {
		t.Errorf("want no error event, got %+v", got.Error)
	}
	if got.SessionID != "ses_e16487fdaffeGYDfCHxMTcCD71" {
		t.Errorf("want the root session id, got %q", got.SessionID)
	}
}

func TestParseOpenCodeV2Stream_Fixture_SingleTurnProtocolTexts(t *testing.T) {
	cases := map[string]string{
		"r4-1.jsonl": "P1", "r4-2.jsonl": "P2", "r4-3.jsonl": "P3", "r4-4.jsonl": "P4",
	}
	for name, child := range cases {
		t.Run(name, func(t *testing.T) {
			got := parseV2Fixture(t, name)

			if !got.Complete {
				t.Fatal("want Complete")
			}
			var protocol struct {
				StatusCode string `json:"status_code"`
				Child      string `json:"child"`
			}
			if err := json.Unmarshal([]byte(got.FinalText), &protocol); err != nil {
				t.Fatalf("want FinalText to be exactly the agent's JSON, got %q: %v", got.FinalText, err)
			}
			if protocol.Child != child || protocol.StatusCode != "SUCCESS" {
				t.Errorf("want child %s SUCCESS, got %+v", child, protocol)
			}
		})
	}
}

func TestParseOpenCodeV2Stream_Fixture_SingleStepBlockedProtocolText(t *testing.T) {
	got := parseV2Fixture(t, "q1-research-write.jsonl")

	if !got.Complete {
		t.Fatal("want Complete")
	}
	var protocol struct {
		StatusCode string `json:"status_code"`
		ErrorCode  string `json:"error_code"`
	}
	if err := json.Unmarshal([]byte(got.FinalText), &protocol); err != nil {
		t.Fatalf("want FinalText to be the agent's JSON, got %q: %v", got.FinalText, err)
	}
	if protocol.StatusCode != "BLOCKED" || protocol.ErrorCode != "E100" {
		t.Errorf("want BLOCKED/E100, got %+v", protocol)
	}
}

// ---------------------------------------------------------------------------
// Final text: parallel subagents (stop followed by background_wait)
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_Fixture_ParallelSubagentsOnlyLastTurnCounts(t *testing.T) {
	got := parseV2Fixture(t, "n1-parallel.jsonl")

	want := "{\"status_code\":\"SUCCESS\",\"child\":\"ALPHA\",\"note\":\"Ran the ping command and read README.md.\"}\n" +
		"{\"status_code\":\"SUCCESS\",\"child\":\"BETA\",\"note\":\"Ran the requested ping and echo commands, then read README.md.\"}\n" +
		"ALL-DONE"
	if !got.Complete {
		t.Error("want Complete: the last stop has no later background_wait")
	}
	if got.FinalText != want {
		t.Errorf("want only the last turn's text %q, got %q", want, got.FinalText)
	}
}

func TestParseOpenCodeV2Stream_Fixture_ParallelStreamTruncatedAtFirstWaitIsIncomplete(t *testing.T) {
	// Cut the recorded stream right after its first background_wait: the
	// orchestrator's "both subagents are running" turn is intermediate.
	data := loadV2Fixture(t, "n1-parallel.jsonl")
	lines := splitLinesKeepingContent(data)
	cut := -1
	for i, l := range lines {
		var ev struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(l), &ev) == nil && ev.Type == "background_wait" {
			cut = i
			break
		}
	}
	if cut < 0 {
		t.Fatal("fixture has no background_wait line")
	}
	truncated := joinLines(lines[:cut+1])

	got, err := harness.ParseOpenCodeV2Stream(truncated)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SessionID == "" {
		t.Error("want the stream's session id to be read")
	}
	if got.Complete {
		t.Error("want incomplete: stream ends after stop + background_wait")
	}
	if got.FinalText != "" {
		t.Errorf("want empty FinalText when incomplete, got %q", got.FinalText)
	}
}

// ---------------------------------------------------------------------------
// Error events
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_Fixture_UnknownAgentErrorEvent(t *testing.T) {
	got := parseV2Fixture(t, "r2-shared.jsonl")

	if got.Error == nil {
		t.Fatal("want the error event")
	}
	if got.Error.Type != "unknown" || got.Error.Message != `Agent not found: "spike-pathdeny"` {
		t.Errorf("want unknown / Agent not found message verbatim, got %+v", got.Error)
	}
	if got.Complete || got.FinalText != "" {
		t.Errorf("want no final answer, got Complete=%v FinalText=%q", got.Complete, got.FinalText)
	}
}

func TestParseOpenCodeV2Stream_Fixture_UnknownModelErrorEvent(t *testing.T) {
	got := parseV2Fixture(t, "r5.out")

	if got.Error == nil {
		t.Fatal("want the error event")
	}
	if got.Error.Type != "provider.no-route" || got.Error.Message != "Model unavailable: openai/no-such-model" {
		t.Errorf("want provider.no-route / Model unavailable message verbatim, got %+v", got.Error)
	}
	if got.Complete {
		t.Error("want not Complete")
	}
}

// ---------------------------------------------------------------------------
// Permission-denied tool call does not fail the run
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_Fixture_PermissionDeniedToolErrorDoesNotFailTheRun(t *testing.T) {
	got := parseV2Fixture(t, "r5-pathdeny.jsonl")

	want := "{\"error\":{\"type\":\"permission.rejected\",\"message\":\"Permission denied: shell\"}  \nAFTER"
	if !got.Complete {
		t.Error("want Complete: a denied tool call is not a run failure")
	}
	if got.Error != nil {
		t.Errorf("want no stream error for a tool error state, got %+v", got.Error)
	}
	if got.FinalText != want {
		t.Errorf("want FinalText %q (the agent's own text), got %q", want, got.FinalText)
	}
}

// ---------------------------------------------------------------------------
// Every fixture: a complete stream never carries an error, and vice versa
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_Fixture_AllRecordedStreamsParseWithoutError(t *testing.T) {
	names := []string{
		"r1-denyshell.jsonl", "n1-parallel.jsonl", "r4-1.jsonl", "r4-2.jsonl", "r4-3.jsonl", "r4-4.jsonl",
		"r5-pathdeny.jsonl", "r5.out", "r2-shared.jsonl", "q1-research-write.jsonl",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got, err := harness.ParseOpenCodeV2Stream(loadV2Fixture(t, name))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Complete == (got.Error != nil) {
				t.Errorf("want exactly one of Complete/Error, got Complete=%v Error=%+v", got.Complete, got.Error)
			}
			if got.Complete && got.FinalText == "" {
				t.Error("want non-empty FinalText on a complete recorded stream")
			}
			if !got.Complete && got.FinalText != "" {
				t.Errorf("want empty FinalText on an incomplete stream, got %q", got.FinalText)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Empty and malformed input
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_EmptyAndWhitespaceInputIsEmptyResponse(t *testing.T) {
	for _, in := range []string{"", " ", "\n", "\r\n\t \n"} {
		_, err := harness.ParseOpenCodeV2Stream([]byte(in))

		if !errors.Is(err, harness.ErrEmptyResponse) {
			t.Errorf("input %q: want ErrEmptyResponse, got %v", in, err)
		}
	}
}

func TestParseOpenCodeV2Stream_NilInputIsEmptyResponse(t *testing.T) {
	_, err := harness.ParseOpenCodeV2Stream(nil)

	if !errors.Is(err, harness.ErrEmptyResponse) {
		t.Errorf("want ErrEmptyResponse, got %v", err)
	}
}

func TestParseOpenCodeV2Stream_NoDecodableLineIsMalformedJSON(t *testing.T) {
	inputs := []string{
		"this is not JSON at all",
		"Usage: opencode run [message..]\n\nOptions:\n  --agent  agent to use\n",
		"[1,2,3]",
		"\"just a string\"",
		"42\nnull\n",
		"{\"type\": \"text\", ",
	}
	for _, in := range inputs {
		_, err := harness.ParseOpenCodeV2Stream([]byte(in))

		if !errors.Is(err, harness.ErrMalformedJSON) {
			t.Errorf("input %q: want ErrMalformedJSON, got %v", in, err)
		}
	}
}
