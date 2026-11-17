package harness_test

// Synthetic-stream tests for ParseOpenCodeV2Stream: the final-text rule's edge
// cases that the recorded fixtures do not cover.
//
// The rule under test: a turn ends at a step_finish with reason "stop"; the
// final turn is the last one with no background_wait after it; the final text
// is the text parts of the last text-bearing step of that turn, joined with a
// newline. Anything else is incomplete with an empty FinalText.

import (
	"encoding/json"
	"strings"
	"testing"

	"mosaic-common/harness"
)

// ---------------------------------------------------------------------------
// Stream builders
// ---------------------------------------------------------------------------

func splitLinesKeepingContent(data []byte) []string {
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func joinLines(lines []string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

func v2Stream(lines ...string) []byte { return joinLines(lines) }

const v2StepStart = `{"type":"step_start","sessionID":"ses_syn","part":{"type":"step-start"}}`

func v2Text(s string) string {
	b, _ := json.Marshal(s)
	return `{"type":"text","sessionID":"ses_syn","part":{"type":"text","text":` + string(b) + `}}`
}

func v2Finish(reason string) string {
	return `{"type":"step_finish","sessionID":"ses_syn","part":{"type":"step-finish","reason":"` + reason +
		`","cost":0.0123,"tokens":{"input":10,"output":5,"reasoning":2,"cache":{"read":3,"write":1}}}}`
}

func v2Wait() string {
	return `{"type":"background_wait","jobs":[{"id":"ses_child","kind":"subagent","title":"work"}]}`
}

func v2Error(typ, msg string) string {
	return `{"type":"error","sessionID":"ses_syn","error":{"type":"` + typ + `","message":` + jsonQuote(msg) + `}}`
}

func v2Tool(tool, status, output string) string {
	return `{"type":"tool_use","sessionID":"ses_syn","part":{"type":"tool","id":"call_1","tool":"` + tool +
		`","state":{"status":"` + status + `","input":{},"output":` + jsonQuote(output) + `}}}`
}

func parseV2(t *testing.T, data []byte) harness.OpenCodeV2Stream {
	t.Helper()
	got, err := harness.ParseOpenCodeV2Stream(data)
	if err != nil {
		t.Fatalf("ParseOpenCodeV2Stream: unexpected error: %v", err)
	}
	return got
}

func assertV2Final(t *testing.T, got harness.OpenCodeV2Stream, wantComplete bool, wantText string) {
	t.Helper()
	if got.SessionID != "ses_syn" {
		t.Errorf("want the stream's session id ses_syn to be read, got %q", got.SessionID)
	}
	if got.Complete != wantComplete {
		t.Errorf("want Complete=%v, got %v", wantComplete, got.Complete)
	}
	if got.FinalText != wantText {
		t.Errorf("want FinalText %q, got %q", wantText, got.FinalText)
	}
}

// ---------------------------------------------------------------------------
// Final text
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_SingleTurnText(t *testing.T) {
	got := parseV2(t, v2Stream(v2StepStart, v2Text("the answer"), v2Finish("stop")))

	assertV2Final(t, got, true, "the answer")
}

func TestParseOpenCodeV2Stream_SeveralTextPartsInFinalStepAreJoinedWithNewline(t *testing.T) {
	got := parseV2(t, v2Stream(v2StepStart, v2Text("first part"), v2Text("second part"), v2Text("third"), v2Finish("stop")))

	assertV2Final(t, got, true, "first part\nsecond part\nthird")
}

func TestParseOpenCodeV2Stream_EarlierStepTextIsNotPartOfFinalText(t *testing.T) {
	got := parseV2(t, v2Stream(
		v2StepStart, v2Text("let me look"), v2Tool("read", "completed", "file body"), v2Finish("tool-calls"),
		v2StepStart, v2Text("final answer"), v2Finish("stop"),
	))

	assertV2Final(t, got, true, "final answer")
}

func TestParseOpenCodeV2Stream_FinalStepWithoutTextFallsBackToLastTextBearingStep(t *testing.T) {
	got := parseV2(t, v2Stream(
		v2StepStart, v2Text("the earlier text"), v2Finish("tool-calls"),
		v2StepStart, v2Finish("stop"),
	))

	assertV2Final(t, got, true, "the earlier text")
}

func TestParseOpenCodeV2Stream_StopWithoutAnyTextIsCompleteWithEmptyText(t *testing.T) {
	got := parseV2(t, v2Stream(v2StepStart, v2Tool("read", "completed", "x"), v2Finish("stop")))

	assertV2Final(t, got, true, "")
}

func TestParseOpenCodeV2Stream_TextAfterTheLastStopIsIgnored(t *testing.T) {
	// A stream cut mid-step: the open step's text was never finished.
	got := parseV2(t, v2Stream(v2StepStart, v2Text("the answer"), v2Finish("stop"), v2StepStart, v2Text("dangling")))

	assertV2Final(t, got, true, "the answer")
}

// ---------------------------------------------------------------------------
// background_wait
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_StopThenBackgroundWaitAtEndIsIncomplete(t *testing.T) {
	got := parseV2(t, v2Stream(v2StepStart, v2Text("waiting for children"), v2Finish("stop"), v2Wait()))

	assertV2Final(t, got, false, "")
}

func TestParseOpenCodeV2Stream_OnlyTheTurnAfterTheLastWaitCounts(t *testing.T) {
	got := parseV2(t, v2Stream(
		v2StepStart, v2Text("intermediate one"), v2Finish("stop"), v2Wait(),
		v2StepStart, v2Text("intermediate two"), v2Finish("stop"), v2Wait(),
		v2StepStart, v2Text("the real answer"), v2Finish("stop"),
	))

	assertV2Final(t, got, true, "the real answer")
}

func TestParseOpenCodeV2Stream_FinalTurnTextDoesNotIncludeIntermediateTurnText(t *testing.T) {
	// The final turn has no text of its own step... its last text-bearing step
	// must come from the final turn only, never from before the wait.
	got := parseV2(t, v2Stream(
		v2StepStart, v2Text("intermediate"), v2Finish("stop"), v2Wait(),
		v2StepStart, v2Text("closing words"), v2Finish("stop"),
	))

	if strings.Contains(got.FinalText, "intermediate") {
		t.Errorf("want no intermediate-turn text in %q", got.FinalText)
	}
	assertV2Final(t, got, true, "closing words")
}

func TestParseOpenCodeV2Stream_WaitThenStepWithoutStopIsIncomplete(t *testing.T) {
	got := parseV2(t, v2Stream(
		v2StepStart, v2Text("waiting"), v2Finish("stop"), v2Wait(),
		v2StepStart, v2Text("woken"), v2Finish("tool-calls"),
	))

	assertV2Final(t, got, false, "")
}

// ---------------------------------------------------------------------------
// Incomplete streams
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_LastFinishReasonOtherThanStopIsIncomplete(t *testing.T) {
	for _, reason := range []string{"tool-calls", "length", "content-filter", "error", "unknown"} {
		t.Run(reason, func(t *testing.T) {
			got := parseV2(t, v2Stream(v2StepStart, v2Text("partial answer"), v2Finish(reason)))

			assertV2Final(t, got, false, "")
		})
	}
}

func TestParseOpenCodeV2Stream_StopFollowedByLaterNonStopFinishIsIncomplete(t *testing.T) {
	got := parseV2(t, v2Stream(
		v2StepStart, v2Text("answer"), v2Finish("stop"),
		v2StepStart, v2Text("more"), v2Finish("tool-calls"),
		v2StepStart, v2Text("cut off"), v2Finish("length"),
	))

	assertV2Final(t, got, false, "")
}

func TestParseOpenCodeV2Stream_NoStepFinishIsIncomplete(t *testing.T) {
	got := parseV2(t, v2Stream(v2StepStart, v2Text("never finished")))

	assertV2Final(t, got, false, "")
}

func TestParseOpenCodeV2Stream_StepStartOnlyIsIncompleteNotAnError(t *testing.T) {
	got, err := harness.ParseOpenCodeV2Stream(v2Stream(v2StepStart))

	if err != nil {
		t.Fatalf("want nil error for an incomplete stream, got %v", err)
	}
	assertV2Final(t, got, false, "")
}

// ---------------------------------------------------------------------------
// Tool events
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_SubagentToolResultIsNeverTheAnswer(t *testing.T) {
	running := "The subagent is working in the background (sessionID: ses_x). You will receive a <subagent> message."
	got := parseV2(t, v2Stream(
		v2StepStart, v2Tool("subagent", "completed", running), v2Finish("tool-calls"),
		v2StepStart, v2Text("dispatched, waiting"), v2Finish("stop"),
	))

	assertV2Final(t, got, true, "dispatched, waiting")
}

func TestParseOpenCodeV2Stream_SubagentResultWithTaskResultMarkupIsNotTreatedAsTheAnswer(t *testing.T) {
	// The V1 task / <task_result> rule does not apply to V2.
	got := parseV2(t, v2Stream(
		v2StepStart, v2Tool("subagent", "completed", "<task_result>{\"status_code\":\"SUCCESS\"}</task_result>"), v2Finish("tool-calls"),
		v2StepStart, v2Text("orchestrator words"), v2Finish("stop"),
	))

	assertV2Final(t, got, true, "orchestrator words")
}

func TestParseOpenCodeV2Stream_SubagentResultAloneYieldsNoFinalText(t *testing.T) {
	got := parseV2(t, v2Stream(v2StepStart, v2Tool("subagent", "completed", "running"), v2Finish("stop")))

	assertV2Final(t, got, true, "")
}

func TestParseOpenCodeV2Stream_ToolErrorStateDoesNotRecordAStreamError(t *testing.T) {
	got := parseV2(t, v2Stream(
		v2StepStart, v2Tool("shell", "error", "Permission denied: shell"), v2Finish("tool-calls"),
		v2StepStart, v2Text("done anyway"), v2Finish("stop"),
	))

	assertV2Final(t, got, true, "done anyway")
	if got.Error != nil {
		t.Errorf("want no stream error, got %+v", got.Error)
	}
}

// ---------------------------------------------------------------------------
// Other events
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_RetryAndUnknownEventTypesAreIgnored(t *testing.T) {
	got := parseV2(t, v2Stream(
		v2StepStart,
		`{"type":"retry","sessionID":"ses_syn","attempt":2,"message":"rate limited"}`,
		`{"type":"some_future_event","sessionID":"ses_syn","part":{"text":"not an answer"}}`,
		v2Text("answer"),
		`{"type":"another_unknown"}`,
		v2Finish("stop"),
	))

	assertV2Final(t, got, true, "answer")
	if got.Error != nil {
		t.Errorf("want retry/unknown events not to set an error, got %+v", got.Error)
	}
}

func TestParseOpenCodeV2Stream_LastErrorEventIsKept(t *testing.T) {
	got := parseV2(t, v2Stream(v2StepStart, v2Error("unknown", "first failure"), v2Error("provider.no-route", "second failure")))

	if got.Error == nil || got.Error.Type != "provider.no-route" || got.Error.Message != "second failure" {
		t.Errorf("want the last error event, got %+v", got.Error)
	}
}

func TestParseOpenCodeV2Stream_ErrorMessageIsKeptVerbatim(t *testing.T) {
	msg := "  Agent not found: \"a b\"\nsecond line  "
	got := parseV2(t, v2Stream(v2Error("unknown", msg)))

	if got.Error == nil || got.Error.Message != msg {
		t.Errorf("want message %q verbatim, got %+v", msg, got.Error)
	}
}

func TestParseOpenCodeV2Stream_SessionIDIsTheFirstNonEmptyTopLevelID(t *testing.T) {
	got := parseV2(t, v2Stream(
		`{"type":"background_wait","jobs":[]}`,
		`{"type":"step_start","sessionID":"","part":{}}`,
		`{"type":"step_start","sessionID":"ses_first","part":{}}`,
		`{"type":"step_start","sessionID":"ses_second","part":{}}`,
	))

	if got.SessionID != "ses_first" {
		t.Errorf("want ses_first, got %q", got.SessionID)
	}
}

func TestParseOpenCodeV2Stream_NoSessionIDGivesEmpty(t *testing.T) {
	got := parseV2(t, v2Stream(`{"type":"step_start","part":{}}`))

	if got.SessionID != "" {
		t.Errorf("want empty SessionID, got %q", got.SessionID)
	}
}

// ---------------------------------------------------------------------------
// Line handling
// ---------------------------------------------------------------------------

func TestParseOpenCodeV2Stream_CRLFLineEndingsAreAccepted(t *testing.T) {
	data := strings.Join([]string{v2StepStart, v2Text("crlf answer"), v2Finish("stop")}, "\r\n") + "\r\n"

	assertV2Final(t, parseV2(t, []byte(data)), true, "crlf answer")
}

func TestParseOpenCodeV2Stream_NoTrailingNewlineIsAccepted(t *testing.T) {
	data := strings.Join([]string{v2StepStart, v2Text("no newline"), v2Finish("stop")}, "\n")

	assertV2Final(t, parseV2(t, []byte(data)), true, "no newline")
}

func TestParseOpenCodeV2Stream_NonJSONAndNonObjectLinesAreSkipped(t *testing.T) {
	got := parseV2(t, v2Stream(
		"warning: something on stdout",
		v2StepStart,
		"",
		"[1,2,3]",
		`"a bare string"`,
		v2Text("answer"),
		"{broken json",
		v2Finish("stop"),
	))

	assertV2Final(t, got, true, "answer")
}

func TestParseOpenCodeV2Stream_BlankLinesBetweenEventsAreIgnored(t *testing.T) {
	data := []byte(v2StepStart + "\n\n\n" + v2Text("answer") + "\n   \n" + v2Finish("stop") + "\n")

	assertV2Final(t, parseV2(t, data), true, "answer")
}

func TestParseOpenCodeV2Stream_TextWithEmbeddedNewlinesAndUnicodeIsPreserved(t *testing.T) {
	text := "line one\n\nline “two” é\n\ttabbed"
	got := parseV2(t, v2Stream(v2StepStart, v2Text(text), v2Finish("stop")))

	assertV2Final(t, got, true, text)
}
