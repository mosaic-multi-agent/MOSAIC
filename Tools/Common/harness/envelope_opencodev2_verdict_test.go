package harness_test

// Tests for OpenCodeV2Verdict: the outcome of one finished V2 invocation,
// decided from the exit code, the stream's error event and the stream itself.
// Each failure cause has its own sentinel and carries V2's message verbatim.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"mosaic-common/harness"
)

// v2Run builds the (Response, runErr) pair Run produces for a finished process.
func v2Run(exit int, stdout, stderr string) (harness.Response, error) {
	resp := harness.Response{ExitCode: exit, Stdout: []byte(stdout), Stderr: stderr}
	if exit != 0 {
		return resp, fmt.Errorf("%w: exit status %d", harness.ErrNonZeroExit, exit)
	}
	return resp, nil
}

var allV2Sentinels = []error{
	harness.ErrOpenCodeV2UnknownAgent, harness.ErrOpenCodeV2UnknownModel, harness.ErrOpenCodeV2UsageError,
	harness.ErrOpenCodeV2RunFailed, harness.ErrOpenCodeV2Interrupted, harness.ErrOpenCodeV2Terminated,
	harness.ErrOpenCodeV2StreamIncomplete,
}

// assertOnlySentinel fails unless err matches want and no other V2 sentinel.
func assertOnlySentinel(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
	for _, other := range allV2Sentinels {
		if other != want && errors.Is(err, other) {
			t.Errorf("want only %v, but error also matches %v", want, other)
		}
	}
}

func verdict(exit int, stdout, stderr string) (string, error) {
	resp, runErr := v2Run(exit, stdout, stderr)
	return harness.OpenCodeV2Verdict(resp, runErr)
}

// ---------------------------------------------------------------------------
// Success
// ---------------------------------------------------------------------------

func TestOpenCodeV2Verdict_ExitZeroCompleteStreamReturnsFinalText(t *testing.T) {
	text, err := verdict(0, string(v2Stream(v2StepStart, v2Text("the answer"), v2Finish("stop"))), "")

	if err != nil || text != "the answer" {
		t.Errorf("want (the answer, nil), got (%q, %v)", text, err)
	}
}

func TestOpenCodeV2Verdict_ExitZeroWithPermissionDeniedToolErrorSucceeds(t *testing.T) {
	text, err := verdict(0, string(loadV2Fixture(t, "r5-pathdeny.jsonl")), "")

	if err != nil || !strings.HasSuffix(text, "AFTER") {
		t.Errorf("want success with the agent's text, got (%q, %v)", text, err)
	}
}

func TestOpenCodeV2Verdict_ExitZeroParallelFixtureReturnsLastTurnText(t *testing.T) {
	text, err := verdict(0, string(loadV2Fixture(t, "n1-parallel.jsonl")), "")

	if err != nil || !strings.HasSuffix(text, "ALL-DONE") || strings.Contains(text, "BETA is still running") {
		t.Errorf("want the last turn's text only, got (%q, %v)", text, err)
	}
}

// ---------------------------------------------------------------------------
// Errors that bypass the stream: Run failed before an exit
// ---------------------------------------------------------------------------

func TestOpenCodeV2Verdict_RunErrorWithoutNonZeroExitIsReturnedUnchanged(t *testing.T) {
	for _, runErr := range []error{harness.ErrTimeout, harness.ErrCancelled, harness.ErrExecutableNotFound, errors.New("launch failed")} {
		_, err := harness.OpenCodeV2Verdict(harness.Response{}, runErr)

		if err != runErr {
			t.Errorf("want %v returned unchanged, got %v", runErr, err)
		}
	}
}

func TestOpenCodeV2Verdict_WrappedTimeoutKeepsItsChain(t *testing.T) {
	runErr := fmt.Errorf("harness: no output: %w", harness.ErrTimeout)

	_, err := harness.OpenCodeV2Verdict(harness.Response{}, runErr)

	if !errors.Is(err, harness.ErrTimeout) {
		t.Errorf("want ErrTimeout preserved, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Interrupt and terminate
// ---------------------------------------------------------------------------

func TestOpenCodeV2Verdict_Exit130IsInterrupted(t *testing.T) {
	_, err := verdict(130, "", "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2Interrupted)
	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Errorf("want error to wrap ErrNonZeroExit, got %v", err)
	}
	if !strings.Contains(err.Error(), "(exit status 130)") {
		t.Errorf("want exit status in the message, got %q", err.Error())
	}
}

func TestOpenCodeV2Verdict_Exit143IsTerminated(t *testing.T) {
	_, err := verdict(143, "", "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2Terminated)
	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Errorf("want error to wrap ErrNonZeroExit, got %v", err)
	}
}

func TestOpenCodeV2Verdict_InterruptWinsOverAnErrorEventInTheStream(t *testing.T) {
	_, err := verdict(130, string(v2Stream(v2Error("unknown", "Agent not found: \"x\""))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2Interrupted)
}

func TestOpenCodeV2Verdict_TerminateWinsOverAnErrorEventInTheStream(t *testing.T) {
	_, err := verdict(143, string(v2Stream(v2Error("provider.no-route", "Model unavailable: x"))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2Terminated)
}

// ---------------------------------------------------------------------------
// Error events: unknown agent, unknown model, generic failure
// ---------------------------------------------------------------------------

func TestOpenCodeV2Verdict_UnknownAgentErrorEventMessagePreservedExactly(t *testing.T) {
	_, err := verdict(1, string(loadV2Fixture(t, "r2-shared.jsonl")), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownAgent)
	want := "harness: opencode-v2 unknown agent: Agent not found: \"spike-pathdeny\" (exit status 1)"
	if err.Error() != want {
		t.Errorf("want message %q, got %q", want, err.Error())
	}
	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Errorf("want error to wrap ErrNonZeroExit, got %v", err)
	}
}

func TestOpenCodeV2Verdict_UnknownModelErrorEventMessagePreservedExactly(t *testing.T) {
	_, err := verdict(1, string(loadV2Fixture(t, "r5.out")), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownModel)
	want := "harness: opencode-v2 unknown model: Model unavailable: openai/no-such-model (exit status 1)"
	if err.Error() != want {
		t.Errorf("want message %q, got %q", want, err.Error())
	}
}

func TestOpenCodeV2Verdict_OtherErrorEventIsRunFailedWithMessage(t *testing.T) {
	_, err := verdict(1, string(v2Stream(v2StepStart, v2Error("api.timeout", "upstream timed out"))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2RunFailed)
	want := "harness: opencode-v2 run failed: upstream timed out (exit status 1)"
	if err.Error() != want {
		t.Errorf("want message %q, got %q", want, err.Error())
	}
}

func TestOpenCodeV2Verdict_ErrorEventAtExitZeroStillFails(t *testing.T) {
	_, err := verdict(0, string(v2Stream(v2Error("unknown", "Agent not found: \"x\""))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownAgent)
	if errors.Is(err, harness.ErrNonZeroExit) {
		t.Errorf("want no ErrNonZeroExit for exit 0, got %v", err)
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("want no exit status suffix for exit 0, got %q", err.Error())
	}
}

func TestOpenCodeV2Verdict_ModelRouteTypeIsMatchedByTypeNotMessage(t *testing.T) {
	_, err := verdict(1, string(v2Stream(v2Error("provider.no-route", "anything at all"))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownModel)
}

func TestOpenCodeV2Verdict_AgentNotFoundIsMatchedByMessagePrefix(t *testing.T) {
	_, err := verdict(1, string(v2Stream(v2Error("whatever", "Agent not found: \"y\""))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownAgent)
}

func TestOpenCodeV2Verdict_ErrorEventWinsOverUsageAndGenericExit(t *testing.T) {
	_, err := verdict(2, string(v2Stream(v2Error("unknown", "Agent not found: \"z\""))), "some stderr")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UnknownAgent)
}

// ---------------------------------------------------------------------------
// Usage error: help text instead of a stream
// ---------------------------------------------------------------------------

const v2HelpText = "\nopencode run [message..]\n\nrun opencode with a message\n\nOptions:\n  --agent  agent to use\n"

func TestOpenCodeV2Verdict_ExitOneWithHelpTextIsUsageError(t *testing.T) {
	_, err := verdict(1, v2HelpText, "Unknown argument: bogus-flag\n")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UsageError)
	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Errorf("want error to wrap ErrNonZeroExit, got %v", err)
	}
	want := "harness: opencode-v2 rejected the command line: Unknown argument: bogus-flag (exit status 1)"
	if err.Error() != want {
		t.Errorf("want message %q, got %q", want, err.Error())
	}
}

func TestOpenCodeV2Verdict_UsageErrorDetailSkipsBlankStderrLines(t *testing.T) {
	_, err := verdict(1, v2HelpText, "\n  \nUnknown argument: x\nmore\n")

	if !strings.Contains(err.Error(), "Unknown argument: x") || strings.Contains(err.Error(), "more") {
		t.Errorf("want only the first non-blank stderr line, got %q", err.Error())
	}
}

func TestOpenCodeV2Verdict_UsageErrorDetailFallsBackToFirstStdoutLine(t *testing.T) {
	_, err := verdict(1, v2HelpText, "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UsageError)
	if !strings.Contains(err.Error(), "opencode run [message..]") || strings.Contains(err.Error(), "Options:") {
		t.Errorf("want the first non-blank stdout line as detail, got %q", err.Error())
	}
}

func TestOpenCodeV2Verdict_ExitOneWithEmptyStdoutIsUsageError(t *testing.T) {
	_, err := verdict(1, "", "boom on stderr")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UsageError)
	if !strings.Contains(err.Error(), "boom on stderr") {
		t.Errorf("want the stderr line in the message, got %q", err.Error())
	}
}

func TestOpenCodeV2Verdict_UsageErrorDetailIsCappedAt200Runes(t *testing.T) {
	long := strings.Repeat("é", 500)

	_, err := verdict(1, "", long)

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2UsageError)
	if strings.Contains(err.Error(), strings.Repeat("é", 201)) {
		t.Errorf("want detail capped at 200 runes, got %d runes", len([]rune(err.Error())))
	}
	if !strings.Contains(err.Error(), strings.Repeat("é", 200)) {
		t.Errorf("want the first 200 runes kept, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Generic failure exit
// ---------------------------------------------------------------------------

func TestOpenCodeV2Verdict_NonZeroExitWithStreamButNoErrorEventIsRunFailed(t *testing.T) {
	_, err := verdict(1, string(v2Stream(v2StepStart, v2Text("partial"))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2RunFailed)
	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Errorf("want error to wrap ErrNonZeroExit, got %v", err)
	}
	if !strings.Contains(err.Error(), "(exit status 1)") {
		t.Errorf("want exit status in the message, got %q", err.Error())
	}
}

func TestOpenCodeV2Verdict_NonZeroExitEvenWithCompleteStreamFails(t *testing.T) {
	_, err := verdict(3, string(v2Stream(v2StepStart, v2Text("answer"), v2Finish("stop"))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2RunFailed)
}

// ---------------------------------------------------------------------------
// Exit zero, bad streams
// ---------------------------------------------------------------------------

func TestOpenCodeV2Verdict_ExitZeroEmptyStdoutIsEmptyResponse(t *testing.T) {
	_, err := verdict(0, "", "")

	if !errors.Is(err, harness.ErrEmptyResponse) {
		t.Errorf("want ErrEmptyResponse, got %v", err)
	}
}

func TestOpenCodeV2Verdict_ExitZeroGarbageIsMalformedJSON(t *testing.T) {
	_, err := verdict(0, "this is not a stream", "")

	if !errors.Is(err, harness.ErrMalformedJSON) {
		t.Errorf("want ErrMalformedJSON, got %v", err)
	}
}

func TestOpenCodeV2Verdict_ExitZeroBlankFinalTextIsEmptyResponse(t *testing.T) {
	for _, text := range []string{"", "  \n\t "} {
		_, err := verdict(0, string(v2Stream(v2StepStart, v2Text(text), v2Finish("stop"))), "")

		if !errors.Is(err, harness.ErrEmptyResponse) {
			t.Errorf("text %q: want ErrEmptyResponse, got %v", text, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Exit zero, incomplete stream: the reason is named
// ---------------------------------------------------------------------------

func TestOpenCodeV2Verdict_IncompleteStreamNamesBackgroundWait(t *testing.T) {
	_, err := verdict(0, string(v2Stream(v2StepStart, v2Text("waiting"), v2Finish("stop"), v2Wait())), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2StreamIncomplete)
	want := "harness: opencode-v2 stream ended without a final answer: waiting for background subagents"
	if err.Error() != want {
		t.Errorf("want message %q, got %q", want, err.Error())
	}
}

func TestOpenCodeV2Verdict_IncompleteStreamNamesNoStepFinished(t *testing.T) {
	_, err := verdict(0, string(v2Stream(v2StepStart, v2Text("never finished"))), "")

	assertOnlySentinel(t, err, harness.ErrOpenCodeV2StreamIncomplete)
	want := "harness: opencode-v2 stream ended without a final answer: no step finished"
	if err.Error() != want {
		t.Errorf("want message %q, got %q", want, err.Error())
	}
}

func TestOpenCodeV2Verdict_IncompleteStreamNamesLastFinishReason(t *testing.T) {
	for _, reason := range []string{"length", "tool-calls", "content-filter", "error", "unknown"} {
		_, err := verdict(0, string(v2Stream(v2StepStart, v2Text("cut"), v2Finish(reason))), "")

		assertOnlySentinel(t, err, harness.ErrOpenCodeV2StreamIncomplete)
		want := "harness: opencode-v2 stream ended without a final answer: last step finished with reason " + reason
		if err.Error() != want {
			t.Errorf("reason %s: want message %q, got %q", reason, want, err.Error())
		}
	}
}

func TestOpenCodeV2Verdict_BackgroundWaitReasonWinsOverLastFinishReason(t *testing.T) {
	_, err := verdict(0, string(v2Stream(v2StepStart, v2Finish("length"), v2Finish("stop"), v2Wait())), "")

	if !strings.Contains(err.Error(), "waiting for background subagents") {
		t.Errorf("want the background wait named first, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Sentinel hygiene
// ---------------------------------------------------------------------------

func TestOpenCodeV2Sentinels_AreDistinctFromEachOtherAndFromV1(t *testing.T) {
	v1 := []error{harness.ErrOpenCodeStreamError, harness.ErrOpenCodeStreamIncomplete}
	for i, a := range allV2Sentinels {
		for j, b := range allV2Sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("%v must not match %v", a, b)
			}
		}
		for _, old := range v1 {
			if errors.Is(a, old) || errors.Is(old, a) {
				t.Errorf("%v and V1 sentinel %v must be unrelated", a, old)
			}
		}
	}
}
