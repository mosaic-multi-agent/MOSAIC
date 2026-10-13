package harness_test

// Tests for the GHCP CLI stream events that fail an invocation although the
// result event may say otherwise (subagent.deselected, session.error), and for
// GHCPCLIVersions.
//
// Coverage:
//
//   - subagent.deselected fails the run whatever the reply (valid protocol
//     JSON or prose) and whatever the result event says.
//   - session.error fails the run with its type/code/status and message in the
//     error text, whatever the result exit code, and also without a result.
//   - The verdict does not depend on whether the events precede or follow result.
//   - Precedence: deselected > session.error > the existing verdict; the first
//     result line stays authoritative for the text and the result verdict.
//   - The session.error detail forms and the tolerance for badly typed members.
//   - Streams without the two events behave exactly as before.
//   - GHCPCLIVersions extracts distinct versions from package paths.
//
// Fixtures live in testdata/ghcpcli/ (trimmed, neutral captures of copilot
// 1.0.91 / 1.0.92 streams); one-off variants are built inline.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/harness"
)

const quotaEventData = `"errorType":"quota","errorCode":"quota_exceeded","statusCode":402,` +
	`"message":"You have exceeded your monthly quota (Request ID: 1)"`

// loadGHCPFixture reads a trimmed stream from testdata/ghcpcli.
func loadGHCPFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "ghcpcli", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func deselectedLine() string { return ghcpEventLine("subagent.deselected", "", false) }

func sessionErrorLine(dataJSON string) string {
	return ghcpEventLine("session.error", dataJSON, false)
}

// assertNotRawTextOrStreamError pins that an error is not classified as a
// raw-text harness error and is not the stream-error sentinel.
func assertNotRawTextOrStreamError(t *testing.T, err error) {
	t.Helper()
	for _, s := range []error{harness.ErrProtocolNotExtractable, harness.ErrMalformedJSON,
		harness.ErrEmptyResponse, harness.ErrGHCPCLIStreamError, harness.ErrGHCPCLIStreamIncomplete} {
		if errors.Is(err, s) {
			t.Errorf("error must not wrap %v, got %v", s, err)
		}
	}
}

// ---------------------------------------------------------------------------
// subagent.deselected
// ---------------------------------------------------------------------------

func TestParseGHCPCLIEnvelope_DeselectedStreamEndingInProtocolReply_FailsWithDeselection(t *testing.T) {
	text, err := harness.ParseGHCPCLIEnvelope(loadGHCPFixture(t, "deselected_protocol_reply_1_0_91.jsonl"), nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLIAgentDeselected) {
		t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
	}
	if text != "" {
		t.Errorf("want empty text on failure, got %q", text)
	}
	assertNotRawTextOrStreamError(t, err)
}

func TestParseGHCPCLIEnvelope_DeselectedProseStream_FailsWithDeselection(t *testing.T) {
	_, err := harness.ParseGHCPCLIEnvelope(loadGHCPFixture(t, "deselected_prose_reply_1_0_91.jsonl"), nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLIAgentDeselected) {
		t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
	}
}

func TestParseGHCPCLIEnvelope_DeselectedAfterSuccessfulResult_StillFails(t *testing.T) {
	stream := ghcpStream(ghcpAssistantMessageLine("hi"), ghcpResultLine(0), deselectedLine())

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLIAgentDeselected) {
		t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
	}
}

func TestParseGHCPCLIEnvelope_DeselectedWithFailedResult_ReportsDeselectionNotStreamError(t *testing.T) {
	stream := ghcpStream(deselectedLine(), ghcpAssistantMessageLine("hi"), ghcpResultLine(1))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 1)

	if !errors.Is(err, harness.ErrGHCPCLIAgentDeselected) {
		t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
	}
	if errors.Is(err, harness.ErrGHCPCLIStreamError) {
		t.Errorf("deselection must win over the stream error, got %v", err)
	}
}

func TestParseGHCPCLIEnvelope_DeselectedWithoutResult_ReportsDeselectionNotIncomplete(t *testing.T) {
	stream := ghcpStream(deselectedLine(), ghcpAssistantMessageLine("partial"))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLIAgentDeselected) {
		t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
	}
	if errors.Is(err, harness.ErrGHCPCLIStreamIncomplete) {
		t.Errorf("deselection must win over an incomplete stream, got %v", err)
	}
}

func TestParseGHCPCLIEnvelope_DeselectedAndSessionError_DeselectionWins(t *testing.T) {
	stream := ghcpStream(sessionErrorLine(quotaEventData), deselectedLine(), ghcpResultLine(1))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 1)

	if !errors.Is(err, harness.ErrGHCPCLIAgentDeselected) {
		t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
	}
	if errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Errorf("deselection must win over session.error, got %v", err)
	}
}

func TestParseGHCPCLIEnvelope_DeselectedWithoutDataMember_StillRecognised(t *testing.T) {
	stream := ghcpStream(`{"type":"subagent.deselected"}`, ghcpAssistantMessageLine("hi"), ghcpResultLine(0))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLIAgentDeselected) {
		t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// session.error
// ---------------------------------------------------------------------------

func TestParseGHCPCLIEnvelope_QuotaStream_FailsWithTypeCodeStatusAndMessage(t *testing.T) {
	_, err := harness.ParseGHCPCLIEnvelope(loadGHCPFixture(t, "quota_session_error_1_0_92.jsonl"), nil, 1)

	if !errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
	}
	for _, want := range []string{"quota", "quota_exceeded", "402", "You have exceeded your monthly quota"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("want error text to contain %q, got %q", want, err.Error())
		}
	}
	assertNotRawTextOrStreamError(t, err)
}

func TestParseGHCPCLIEnvelope_SessionErrorWithResultExitCodeZero_Fails(t *testing.T) {
	stream := ghcpStream(ghcpAssistantMessageLine("hi"), sessionErrorLine(quotaEventData), ghcpResultLine(0))

	text, err := harness.ParseGHCPCLIEnvelope(stream, nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
	}
	if text != "" {
		t.Errorf("want empty text on failure, got %q", text)
	}
}

func TestParseGHCPCLIEnvelope_SessionErrorAfterSuccessfulResult_StillFails(t *testing.T) {
	stream := ghcpStream(ghcpAssistantMessageLine("hi"), ghcpResultLine(0), sessionErrorLine(quotaEventData))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
	}
}

func TestParseGHCPCLIEnvelope_SessionErrorAfterFailedResult_ReportsSessionErrorDetail(t *testing.T) {
	stream := ghcpStream(ghcpResultLine(1), sessionErrorLine(quotaEventData))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 1)

	if !errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
	}
	if !strings.Contains(err.Error(), "quota_exceeded") {
		t.Errorf("want error text to carry the error code, got %q", err.Error())
	}
}

func TestParseGHCPCLIEnvelope_SessionErrorWithoutResult_CarriesTypeCodeAndMessage(t *testing.T) {
	stream := ghcpStream(ghcpAssistantMessageLine("partial"), sessionErrorLine(quotaEventData))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 0)

	if !errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
	}
	for _, want := range []string{"quota", "quota_exceeded", "monthly quota"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("want error text to contain %q, got %q", want, err.Error())
		}
	}
}

func TestParseGHCPCLIEnvelope_TwoSessionErrors_ReportsTheFirst(t *testing.T) {
	stream := ghcpStream(
		sessionErrorLine(`"errorType":"first-type","message":"first message"`),
		sessionErrorLine(`"errorType":"second-type","message":"second message"`),
		ghcpResultLine(1))

	_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 1)

	if !errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
	}
	if !strings.Contains(err.Error(), "first message") || strings.Contains(err.Error(), "second message") {
		t.Errorf("want the first session.error only, got %q", err.Error())
	}
}

func TestParseGHCPCLIEnvelope_SessionError_ErrorTextCarriesStderrAndProcessExitCode(t *testing.T) {
	stream := ghcpStream(sessionErrorLine(quotaEventData), ghcpResultLine(1))

	_, err := harness.ParseGHCPCLIEnvelope(stream, []byte("  some stderr text \n"), 7)

	if !errors.Is(err, harness.ErrGHCPCLISessionError) {
		t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
	}
	if !strings.Contains(err.Error(), "some stderr text") || !strings.Contains(err.Error(), "7") {
		t.Errorf("want stderr and process exit code in the text, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// session.error detail forms and decoding tolerance
// ---------------------------------------------------------------------------

func TestParseGHCPCLIEnvelope_SessionErrorDetailForms(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string // substring of the error text, directly before " (stderr:"
	}{
		{"parts and message", sessionErrorLine(quotaEventData),
			"type quota, code quota_exceeded, status 402: You have exceeded your monthly quota (Request ID: 1)"},
		{"parts only", sessionErrorLine(`"errorType":"quota","errorCode":"quota_exceeded"`),
			": type quota, code quota_exceeded (stderr:"},
		{"status only", sessionErrorLine(`"statusCode":429`), ": status 429 (stderr:"},
		{"message only", sessionErrorLine(`"message":"boom"`), ": boom (stderr:"},
		{"empty data object", sessionErrorLine(""), ": no details (stderr:"},
		{"data member absent", `{"type":"session.error"}`, ": no details (stderr:"},
		{"data not an object", `{"type":"session.error","data":"oops"}`, ": no details (stderr:"},
		{"data null", `{"type":"session.error","data":null}`, ": no details (stderr:"},
		{"status code as string is omitted", sessionErrorLine(`"errorType":"quota","statusCode":"402","message":"m"`),
			": type quota: m (stderr:"},
		{"empty strings are omitted", sessionErrorLine(`"errorType":"","errorCode":"c","message":""`),
			": code c (stderr:"},
		{"badly typed members are omitted", sessionErrorLine(`"errorType":5,"errorCode":{"a":1},"message":["x"]`),
			": no details (stderr:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := ghcpStream(tc.line, ghcpResultLine(1))

			_, err := harness.ParseGHCPCLIEnvelope(stream, nil, 1)

			if !errors.Is(err, harness.ErrGHCPCLISessionError) {
				t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want error text to contain %q, got %q", tc.want, err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Order, second result, unchanged behaviour
// ---------------------------------------------------------------------------

func TestParseGHCPCLIEnvelope_SecondResultLine_ChangesNeitherVerdictNorText(t *testing.T) {
	ok := ghcpStream(ghcpAssistantMessageLine("A"), ghcpResultLine(0),
		ghcpAssistantMessageLine("B"), ghcpResultLine(1))
	failed := ghcpStream(ghcpAssistantMessageLine("A"), ghcpResultLine(1), ghcpResultLine(0))

	text, err := harness.ParseGHCPCLIEnvelope(ok, nil, 0)
	_, failedErr := harness.ParseGHCPCLIEnvelope(failed, nil, 1)

	if err != nil || text != "A" {
		t.Errorf("want text %q and no error when a later result fails, got %q, %v", "A", text, err)
	}
	if !errors.Is(failedErr, harness.ErrGHCPCLIStreamError) {
		t.Errorf("want ErrGHCPCLIStreamError when the first result fails, got %v", failedErr)
	}
}

func TestParseGHCPCLIEnvelope_NormalStream_ReturnsAssistantTextUnchanged(t *testing.T) {
	text, err := harness.ParseGHCPCLIEnvelope(loadGHCPFixture(t, "normal_1_0_92.jsonl"), nil, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello from the agent." {
		t.Errorf("want %q, got %q", "Hello from the agent.", text)
	}
}

func TestParseGHCPCLIEnvelope_OtherEventsWithUnusualDataTypes_GiveTodaysResult(t *testing.T) {
	odd := []string{
		`{"type":"model.call_failure","data":{"message":5,"statusCode":"402","errorType":{"x":1}}}`,
		`{"type":"some.other","data":"text"}`,
		`{"type":"some.other","data":[1,2]}`,
		`{"type":"some.other","data":null}`,
	}
	withResult := ghcpStream(append(append([]string{ghcpAssistantMessageLine("ok")}, odd...), ghcpResultLine(0))...)
	noResult := ghcpStream(odd...)

	text, err := harness.ParseGHCPCLIEnvelope(withResult, nil, 0)
	_, incompleteErr := harness.ParseGHCPCLIEnvelope(noResult, nil, 0)

	if err != nil || text != "ok" {
		t.Errorf("want %q and no error, got %q, %v", "ok", text, err)
	}
	if !errors.Is(incompleteErr, harness.ErrGHCPCLIStreamIncomplete) {
		t.Errorf("want ErrGHCPCLIStreamIncomplete (the lines decode), got %v", incompleteErr)
	}
}

func TestParseGHCPCLIEnvelope_NonJSONLineMentioningEventType_IsSkipped(t *testing.T) {
	stream := ghcpStream(`not json {"type":"session.error"} subagent.deselected`,
		ghcpAssistantMessageLine("ok"), ghcpResultLine(0))

	text, err := harness.ParseGHCPCLIEnvelope(stream, nil, 0)

	if err != nil || text != "ok" {
		t.Errorf("want %q and no error, got %q, %v", "ok", text, err)
	}
}

func TestGHCPCLIEventFixtures_ContainNoUserNamesLocalPathsOrRequestIDs(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "ghcpcli", "*.jsonl"))
	if err != nil || len(names) == 0 {
		t.Fatalf("want fixtures under testdata/ghcpcli, got %v, %v", names, err)
	}
	for _, name := range names {
		data, _ := os.ReadFile(name)
		for _, banned := range []string{"tgurt", `C:\\AI`, `\\Users\\tgurt`, "2834:B0F04", "7088:DF8EF"} {
			if strings.Contains(string(data), banned) {
				t.Errorf("%s contains %q", filepath.Base(name), banned)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// GHCPCLIVersions
// ---------------------------------------------------------------------------

func TestGHCPCLIVersions(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		want   []string
	}{
		{"normal fixture", string(loadGHCPFixture(t, "normal_1_0_92.jsonl")), []string{"1.0.92"}},
		{"deselected fixture", string(loadGHCPFixture(t, "deselected_protocol_reply_1_0_91.jsonl")), []string{"1.0.91"}},
		{"no version path", `{"type":"assistant.message","data":{"content":"hi"}}`, nil},
		{"distinct versions in first-seen order",
			`"x\\pkg\\win32-x64\\1.0.92\\a" "y\\pkg\\win32-x64\\1.0.91\\b" "z\\pkg\\win32-x64\\1.0.92\\c"`,
			[]string{"1.0.92", "1.0.91"}},
		{"path ends at the version", `{"p":"C:\\x\\pkg\\win32-x64\\1.0.92"}`, []string{"1.0.92"}},
		{"forward slashes", `/home/u/.copilot/pkg/linux-x64/1.0.92/builtin/s`, []string{"1.0.92"}},
		{"single backslashes and prerelease", `C:\x\pkg\darwin-arm64\1.1.0-beta.2\s`, []string{"1.1.0-beta.2"}},
		{"pkg at start of input", `pkg/linux-x64/2.0.0/x`, []string{"2.0.0"}},
		{"not a platform segment", `/a/pkg/dist/2024/x`, nil},
		{"pkg is not a whole segment", `/a/xpkg/linux-x64/1.0.0/x`, nil},
		{"version without a dot", `/a/pkg/linux-x64/7/x`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := harness.GHCPCLIVersions([]byte(tc.stdout))

			if len(got) != len(tc.want) || (len(got) > 0 && !reflect.DeepEqual(got, tc.want)) {
				t.Errorf("want %v, got %v", tc.want, got)
			}
		})
	}
}
