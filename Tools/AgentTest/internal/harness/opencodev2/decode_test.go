package opencodev2_test

// Tests for envelope decoding: mapping a finished V2 invocation onto the
// subject's result. Real streams come from tool-local copies of recorded V2
// runs; failure shapes with no recording are built from minimal lines. The
// V2 failure cause is judged by the first non-blank line of Stderr, which is
// where the neutral readers look.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func v2Text(text string) string {
	return `{"type":"text","part":{"type":"text","text":` + jsonQuote(text) + `}}`
}

func v2StepStart() string { return `{"type":"step_start","part":{"type":"step-start"}}` }

func v2StepFinish(reason string) string {
	return `{"type":"step_finish","part":{"type":"step-finish","reason":` + jsonQuote(reason) + `}}`
}

func v2Error(typ, message string) string {
	return `{"type":"error","error":{"type":` + jsonQuote(typ) + `,"message":` + jsonQuote(message) + `}}`
}

func v2Stream(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

// nonZeroExit mimics the error Run returns for a process that exited non-zero.
func nonZeroExit() error { return fmt.Errorf("exit status 1: %w", commonharness.ErrNonZeroExit) }

func decode(t *testing.T, res commonharness.Response, runErr error) domain.SubjectResult {
	t.Helper()
	result, err := opencodev2.DecodeEnvelope(res, runErr)
	if err != nil {
		t.Fatalf("DecodeEnvelope returned error %v, want nil", err)
	}
	return result
}

func firstNonBlankLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// assertSpawnFailedWithCause checks the disposition and that the first
// non-blank Stderr line carries the sentinel's text and every detail given.
func assertSpawnFailedWithCause(t *testing.T, result domain.SubjectResult, cause error, details ...string) {
	t.Helper()
	if result.Disposition != domain.DispositionSpawnFailed {
		t.Fatalf("Disposition = %q, want %q", result.Disposition, domain.DispositionSpawnFailed)
	}
	first := firstNonBlankLine(result.Stderr)
	if !strings.Contains(first, cause.Error()) {
		t.Errorf("first Stderr line = %q, want it to carry %q", first, cause.Error())
	}
	for _, d := range details {
		if !strings.Contains(first, d) {
			t.Errorf("first Stderr line = %q, want it to carry %q", first, d)
		}
	}
}

// --- completed runs, from recorded streams ---

func TestDecodeEnvelope_RecordedStreamWithProtocolJSON_CompletedWithProtocolMessage(t *testing.T) {
	stdout := readStreamFixture(t, "q1-research-write.jsonl")
	res := commonharness.Response{Stdout: stdout, Stderr: "warn", ExitCode: 0, Duration: 3 * time.Second}

	result := decode(t, res, nil)

	if result.Disposition != domain.DispositionCompleted {
		t.Fatalf("Disposition = %q, want %q", result.Disposition, domain.DispositionCompleted)
	}
	var proto map[string]any
	if err := json.Unmarshal([]byte(result.ProtocolMessage), &proto); err != nil {
		t.Fatalf("ProtocolMessage %q is not the extracted protocol JSON: %v", result.ProtocolMessage, err)
	}
	if proto["status_code"] != "BLOCKED" {
		t.Errorf("ProtocolMessage status_code = %v, want BLOCKED", proto["status_code"])
	}
	if result.RawOutput != string(stdout) || result.Stderr != "warn" || result.Duration != 3*time.Second || result.ExitCode != 0 {
		t.Errorf("result = %+v, want RawOutput, Stderr, Duration and ExitCode carried from the response", result)
	}
}

func TestDecodeEnvelope_RecordedSubagentRun_CompletedWithItsProtocolMessage(t *testing.T) {
	res := commonharness.Response{Stdout: readStreamFixture(t, "r4-1.jsonl")}

	result := decode(t, res, nil)

	if result.Disposition != domain.DispositionCompleted {
		t.Fatalf("Disposition = %q, want %q", result.Disposition, domain.DispositionCompleted)
	}
	if !strings.Contains(result.ProtocolMessage, `"child":"P1"`) {
		t.Errorf("ProtocolMessage = %q, want the final text's protocol JSON", result.ProtocolMessage)
	}
}

func TestDecodeEnvelope_RecordedRunWithToolErrorState_StillCompleted(t *testing.T) {
	res := commonharness.Response{Stdout: readStreamFixture(t, "r5-pathdeny.jsonl")}

	result := decode(t, res, nil)

	if result.Disposition != domain.DispositionCompleted {
		t.Errorf("Disposition = %q, want %q: a denied tool call does not fail the run", result.Disposition, domain.DispositionCompleted)
	}
	if strings.TrimSpace(result.ProtocolMessage) == "" {
		t.Errorf("ProtocolMessage is empty, want the final assistant text")
	}
}

func TestDecodeEnvelope_RecordedRunWithBackgroundWaits_CompletedAtTheFinalTurn(t *testing.T) {
	res := commonharness.Response{Stdout: readStreamFixture(t, "n1-parallel.jsonl")}

	result := decode(t, res, nil)

	if result.Disposition != domain.DispositionCompleted {
		t.Errorf("Disposition = %q, want %q", result.Disposition, domain.DispositionCompleted)
	}
	if strings.TrimSpace(result.ProtocolMessage) == "" {
		t.Errorf("ProtocolMessage is empty, want the final turn's text")
	}
}

func TestDecodeEnvelope_FinalTextWithoutProtocolJSON_CarriedRaw(t *testing.T) {
	res := commonharness.Response{Stdout: v2Stream(v2StepStart(), v2Text("all finished, nothing to report"), v2StepFinish("stop"))}

	result := decode(t, res, nil)

	if result.Disposition != domain.DispositionCompleted {
		t.Fatalf("Disposition = %q, want %q", result.Disposition, domain.DispositionCompleted)
	}
	if result.ProtocolMessage != "all finished, nothing to report" {
		t.Errorf("ProtocolMessage = %q, want the raw final text", result.ProtocolMessage)
	}
}

// --- failures, with the cause preserved on the first Stderr line ---

func TestDecodeEnvelope_UnknownAgent_SpawnFailedNamingTheCause(t *testing.T) {
	stdout := readStreamFixture(t, "r2-shared.jsonl")
	res := commonharness.Response{Stdout: stdout, Stderr: "process noise", ExitCode: 1, Duration: time.Second}

	result := decode(t, res, nonZeroExit())

	assertSpawnFailedWithCause(t, result, commonharness.ErrOpenCodeV2UnknownAgent, `Agent not found: "spike-pathdeny"`)
	if result.ExitCode != 1 || result.RawOutput != string(stdout) || result.Duration != time.Second {
		t.Errorf("result = %+v, want ExitCode, RawOutput and Duration carried from the response", result)
	}
	if !strings.Contains(result.Stderr, "process noise") {
		t.Errorf("Stderr = %q, want the process stderr kept after the cause", result.Stderr)
	}
	if strings.HasPrefix(strings.TrimSpace(result.Stderr), "process noise") {
		t.Errorf("Stderr = %q, want the cause before the process stderr", result.Stderr)
	}
}

func TestDecodeEnvelope_UnknownModel_SpawnFailedNamingTheCause(t *testing.T) {
	res := commonharness.Response{Stdout: v2Stream(v2Error("provider.no-route", "No route for model acme/none")), ExitCode: 1}

	result := decode(t, res, nonZeroExit())

	assertSpawnFailedWithCause(t, result, commonharness.ErrOpenCodeV2UnknownModel, "No route for model acme/none")
}

func TestDecodeEnvelope_UnknownFlag_SpawnFailedAsUsageError(t *testing.T) {
	res := commonharness.Response{Stdout: nil, Stderr: "unknown option --bogus\nusage: opencode run", ExitCode: 1}

	result := decode(t, res, nonZeroExit())

	assertSpawnFailedWithCause(t, result, commonharness.ErrOpenCodeV2UsageError, "unknown option --bogus")
}

func TestDecodeEnvelope_ErrorEventOnZeroExit_SpawnFailedAsRunFailed(t *testing.T) {
	res := commonharness.Response{Stdout: v2Stream(v2StepStart(), v2Error("unknown", "provider exploded")), ExitCode: 0}

	result := decode(t, res, nil)

	assertSpawnFailedWithCause(t, result, commonharness.ErrOpenCodeV2RunFailed, "provider exploded")
}

func TestDecodeEnvelope_NonZeroExitWithoutErrorEvent_SpawnFailedAsRunFailed(t *testing.T) {
	res := commonharness.Response{Stdout: v2Stream(v2StepStart(), v2Text("partial")), ExitCode: 2}

	result := decode(t, res, nonZeroExit())

	assertSpawnFailedWithCause(t, result, commonharness.ErrOpenCodeV2RunFailed)
	if result.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", result.ExitCode)
	}
}

func TestDecodeEnvelope_InterruptAndTerminateExits_NameTheirCauses(t *testing.T) {
	cases := []struct {
		code  int
		cause error
	}{
		{130, commonharness.ErrOpenCodeV2Interrupted},
		{143, commonharness.ErrOpenCodeV2Terminated},
	}
	for _, c := range cases {
		t.Run(c.cause.Error(), func(t *testing.T) {
			result := decode(t, commonharness.Response{ExitCode: c.code}, nonZeroExit())

			assertSpawnFailedWithCause(t, result, c.cause)
		})
	}
}

func TestDecodeEnvelope_StreamEndingWithoutFinalAnswer_SpawnFailedAsIncomplete(t *testing.T) {
	res := commonharness.Response{Stdout: v2Stream(v2StepStart(), v2Text("working"), v2StepFinish("tool-calls"))}

	result := decode(t, res, nil)

	assertSpawnFailedWithCause(t, result, commonharness.ErrOpenCodeV2StreamIncomplete, "tool-calls")
}

func TestDecodeEnvelope_EmptyOutputOnZeroExit_SpawnFailedAsEmptyResponse(t *testing.T) {
	result := decode(t, commonharness.Response{}, nil)

	assertSpawnFailedWithCause(t, result, commonharness.ErrEmptyResponse)
}

func TestDecodeEnvelope_BlankFinalText_SpawnFailedAsEmptyResponse(t *testing.T) {
	res := commonharness.Response{Stdout: v2Stream(v2StepStart(), v2Text("   "), v2StepFinish("stop"))}

	result := decode(t, res, nil)

	assertSpawnFailedWithCause(t, result, commonharness.ErrEmptyResponse)
}

// --- process-level errors ---

func TestDecodeEnvelope_Timeout_TimedOutWithDurationOnly(t *testing.T) {
	res := commonharness.Response{Duration: 31 * time.Minute, Stdout: []byte("ignored")}

	result := decode(t, res, commonharness.ErrTimeout)

	if result.Disposition != domain.DispositionTimedOut {
		t.Fatalf("Disposition = %q, want %q", result.Disposition, domain.DispositionTimedOut)
	}
	if result.Duration != 31*time.Minute {
		t.Errorf("Duration = %v, want 31m", result.Duration)
	}
}

func TestDecodeEnvelope_NoOutputWatchdog_TimedOut(t *testing.T) {
	runErr := fmt.Errorf("%w: %w", commonharness.ErrNoOutput, commonharness.ErrTimeout)

	result := decode(t, commonharness.Response{}, runErr)

	if result.Disposition != domain.DispositionTimedOut {
		t.Errorf("Disposition = %q, want %q", result.Disposition, domain.DispositionTimedOut)
	}
}

func TestDecodeEnvelope_ExecutableNotFoundAndOtherRunErrors_SpawnFailedWithErrorText(t *testing.T) {
	cases := []error{commonharness.ErrExecutableNotFound, errors.New("some unrelated failure")}
	for _, runErr := range cases {
		t.Run(runErr.Error(), func(t *testing.T) {
			result := decode(t, commonharness.Response{}, runErr)

			if result.Disposition != domain.DispositionSpawnFailed {
				t.Fatalf("Disposition = %q, want %q", result.Disposition, domain.DispositionSpawnFailed)
			}
			if !strings.Contains(firstNonBlankLine(result.Stderr), runErr.Error()) {
				t.Errorf("Stderr = %q, want the run error's text on its first line", result.Stderr)
			}
		})
	}
}

func TestDecodeEnvelope_NeverProducesEarlyExitOrTurnLimit(t *testing.T) {
	streams := [][]byte{
		readStreamFixture(t, "q1-research-write.jsonl"),
		readStreamFixture(t, "r2-shared.jsonl"),
		nil,
	}
	for _, s := range streams {
		result := decode(t, commonharness.Response{Stdout: s}, nil)

		if result.Disposition == "" || result.Disposition == domain.DispositionEarlyExit || result.Disposition == domain.DispositionTurnLimit {
			t.Errorf("Disposition = %q, want completed, timed_out or spawn_failed", result.Disposition)
		}
	}
}
