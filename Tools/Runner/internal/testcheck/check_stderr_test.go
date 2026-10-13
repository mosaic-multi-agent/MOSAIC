package testcheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/testcheck"
)

// ============================================================
// Check tests: stderr_contains
// ============================================================

// singleDispatchLog writes a dispatch log holding one successful dispatch of
// "agent" and returns its path.
func singleDispatchLog(t *testing.T) string {
	t.Helper()
	return writeDispatchLog(t, t.TempDir(), []string{
		requestLine("agent#1", "do the work"),
		responseLine("agent#1", "SUCCESS"),
	})
}

// singleDispatchExpected returns an expectation for the log written by
// singleDispatchLog with the given stderr assertion.
func singleDispatchExpected(stderrContains string) *testcheck.ExpectedOutcome {
	return &testcheck.ExpectedOutcome{
		ExitCode: 0,
		Dispatches: []testcheck.ExpectedDispatch{
			{Agent: "agent", ExpectStatus: "SUCCESS"},
		},
		StderrContains: stderrContains,
	}
}

func TestCheck_StderrContainsPresent_Passes(t *testing.T) {
	logPath := singleDispatchLog(t)
	input := testcheck.CheckInput{
		ExitCode:    0,
		DispatchLog: logPath,
		Stderr:      "run started\nconsultant stop: PRECONSULT-STOP-MARKER\n",
	}

	result := testcheck.Check(input, singleDispatchExpected("PRECONSULT-STOP-MARKER"))

	if !result.Pass {
		t.Errorf("Pass = false, want true when stderr contains expected text; mismatch: %+v", result.Mismatch)
	}
}

func TestCheck_StderrContainsAbsent_ReportsMismatchStderrContains(t *testing.T) {
	logPath := singleDispatchLog(t)
	input := testcheck.CheckInput{ExitCode: 0, DispatchLog: logPath, Stderr: "run started\n"}

	result := testcheck.Check(input, singleDispatchExpected("PRECONSULT-STOP-MARKER"))

	if result.Pass {
		t.Fatal("Pass = true, want false when stderr lacks the expected text")
	}
	if result.Mismatch == nil {
		t.Fatal("Mismatch = nil, want non-nil")
	}
	if result.Mismatch.Kind != testcheck.MismatchStderrContains {
		t.Errorf("Mismatch.Kind = %v, want MismatchStderrContains", result.Mismatch.Kind)
	}
	if result.Mismatch.Index != -1 {
		t.Errorf("Mismatch.Index = %d, want -1", result.Mismatch.Index)
	}
	if result.Mismatch.ExpectedValue != "PRECONSULT-STOP-MARKER" {
		t.Errorf("Mismatch.ExpectedValue = %q, want the expected substring", result.Mismatch.ExpectedValue)
	}
	if result.Mismatch.ActualValue != "run started\n" {
		t.Errorf("Mismatch.ActualValue = %q, want the whole short stderr", result.Mismatch.ActualValue)
	}
	if !strings.Contains(result.Mismatch.Message, "PRECONSULT-STOP-MARKER") {
		t.Errorf("Mismatch.Message = %q, want it to name the expected text", result.Mismatch.Message)
	}
}

func TestCheck_StderrContainsAbsent_ExcerptIsLast300Bytes(t *testing.T) {
	logPath := singleDispatchLog(t)
	stderr := "HEAD-OF-STDERR" + strings.Repeat("x", 400) + "TAIL-OF-STDERR"
	input := testcheck.CheckInput{ExitCode: 0, DispatchLog: logPath, Stderr: stderr}

	result := testcheck.Check(input, singleDispatchExpected("NOT-THERE"))

	if result.Mismatch == nil || result.Mismatch.Kind != testcheck.MismatchStderrContains {
		t.Fatalf("want MismatchStderrContains, got %+v", result.Mismatch)
	}
	want := stderr[len(stderr)-300:]
	if result.Mismatch.ActualValue != want {
		t.Errorf("ActualValue len = %d, want the last 300 bytes of stderr (len %d)",
			len(result.Mismatch.ActualValue), len(want))
	}
	if strings.Contains(result.Mismatch.Message, "HEAD-OF-STDERR") {
		t.Error("Message contains the head of a long stderr, want only a bounded tail excerpt")
	}
	if !strings.Contains(result.Mismatch.Message, "TAIL-OF-STDERR") {
		t.Error("Message lacks the tail of stderr, want the bounded tail excerpt")
	}
}

func TestCheck_StderrContainsAbsentField_StderrNotChecked(t *testing.T) {
	logPath := singleDispatchLog(t)
	input := testcheck.CheckInput{ExitCode: 0, DispatchLog: logPath, Stderr: "anything at all\n"}

	result := testcheck.Check(input, singleDispatchExpected(""))

	if !result.Pass {
		t.Errorf("Pass = false, want true when stderr_contains is empty; mismatch: %+v", result.Mismatch)
	}
}

func TestCheck_StderrContainsSet_ExitCodeMismatchReportedFirst(t *testing.T) {
	logPath := singleDispatchLog(t)
	input := testcheck.CheckInput{ExitCode: 7, DispatchLog: logPath, Stderr: "no marker here"}

	result := testcheck.Check(input, singleDispatchExpected("PRECONSULT-STOP-MARKER"))

	if result.Pass || result.Mismatch == nil {
		t.Fatalf("want a failing result, got %+v", result)
	}
	if result.Mismatch.Kind != testcheck.MismatchExitCode {
		t.Errorf("Mismatch.Kind = %v, want MismatchExitCode (exit code is checked before stderr)", result.Mismatch.Kind)
	}
}

func TestCheck_StderrContainsSet_DispatchMismatchReportedBeforeStderr(t *testing.T) {
	logPath := singleDispatchLog(t)
	expected := singleDispatchExpected("PRECONSULT-STOP-MARKER")
	expected.Dispatches[0].Agent = "other-agent"
	input := testcheck.CheckInput{ExitCode: 0, DispatchLog: logPath, Stderr: "no marker here"}

	result := testcheck.Check(input, expected)

	if result.Pass || result.Mismatch == nil {
		t.Fatalf("want a failing result, got %+v", result)
	}
	if result.Mismatch.Kind != testcheck.MismatchAgent {
		t.Errorf("Mismatch.Kind = %v, want MismatchAgent (dispatch sequence is checked before stderr)", result.Mismatch.Kind)
	}
}

// ============================================================
// LoadExpected tests: stderr_contains
// ============================================================

func writeRawSidecar(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "raw.expected.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoadExpected_StderrContainsField_IsRead(t *testing.T) {
	path := writeRawSidecar(t, `{"exit_code":6,"dispatches":[],"stderr_contains":"PRECONSULT-STOP-MARKER"}`)

	got, err := testcheck.LoadExpected(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StderrContains != "PRECONSULT-STOP-MARKER" {
		t.Errorf("StderrContains = %q, want %q", got.StderrContains, "PRECONSULT-STOP-MARKER")
	}
	if got.ExitCode != 6 {
		t.Errorf("ExitCode = %d, want 6", got.ExitCode)
	}
}

func TestLoadExpected_FormatWithoutStderrField_StillLoads(t *testing.T) {
	path := writeRawSidecar(t,
		`{"exit_code":0,"dispatches":[{"agent":"mosaictest-scripted","expect_status":"SUCCESS"}]}`)

	got, err := testcheck.LoadExpected(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StderrContains != "" {
		t.Errorf("StderrContains = %q, want empty for a file without the field", got.StderrContains)
	}
	if len(got.Dispatches) != 1 || got.Dispatches[0].Agent != "mosaictest-scripted" {
		t.Errorf("Dispatches = %+v, want the one existing dispatch preserved", got.Dispatches)
	}
}
