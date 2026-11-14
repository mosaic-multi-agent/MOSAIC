package dispatchlog_test

// Tests for reading a dispatch log back: the decoded entries of a log the
// Logger wrote, tolerance of undecodable lines, and the path helpers that tell
// a run's own log from a startup fallback file.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
)

const readerRunID = "20261106T135514Z-7c2e"

func writeLogFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "log.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return path
}

func TestReadEntries_LoggerOutput_DecodesEveryEntryTypeInFileOrder(t *testing.T) {
	dir := t.TempDir()
	l := dispatchlog.New(dir)
	l.SetRunID(readerRunID)
	l.SetToolVersion("1.2.3")
	l.LogRequest(domain.ProtocolRequest{AgentInstanceID: "worker#1", TaskDescription: "t"})
	l.LogResponse(domain.ProtocolResponse{AgentInstanceID: "worker#1", StatusCode: domain.StatusSUCCESS, StatusMessage: "ok"})
	l.LogRequest(domain.ProtocolRequest{AgentInstanceID: "worker#2", TaskDescription: "t"})
	l.LogResponse(domain.ProtocolResponse{
		AgentInstanceID: "worker#2", StatusCode: domain.StatusBLOCKED, StatusMessage: "no",
		ErrorCode: domain.ErrorDEPENDENCY_MISSING, ErrorReason: "r",
	})
	l.LogHarnessError("worker#3", domain.HarnessFailure{Kind: domain.HarnessFailureTimeout}, "timed out")
	l.LogError("orchestrator#routing#1", "plain failure")
	l.Close()

	entries, err := dispatchlog.ReadEntries(l.Path())

	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	want := []dispatchlog.Entry{
		{Type: "version"},
		{Type: "request", AgentInstanceID: "worker#1"},
		{Type: "response", AgentInstanceID: "worker#1", StatusCode: domain.StatusSUCCESS},
		{Type: "request", AgentInstanceID: "worker#2"},
		{Type: "response", AgentInstanceID: "worker#2", StatusCode: domain.StatusBLOCKED, ErrorCode: domain.ErrorDEPENDENCY_MISSING},
		{Type: "error", AgentInstanceID: "worker#3", HarnessError: true},
		{Type: "error", AgentInstanceID: "orchestrator#routing#1"},
	}
	if len(entries) != len(want) {
		t.Fatalf("want %d entries, got %d: %+v", len(want), len(entries), entries)
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Errorf("entry %d: want %+v, got %+v", i, want[i], entries[i])
		}
	}
}

func TestReadEntries_CorrelationEntry_IsDecodedWithoutAgentID(t *testing.T) {
	dir := t.TempDir()
	l := dispatchlog.New(dir)
	l.LogError("worker#1", "early failure") // opens the startup fallback file
	l.SetRunID(readerRunID)                 // records the correlation entry
	l.Close()

	entries, err := dispatchlog.ReadEntries(l.Path())

	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(entries) != 2 || entries[1].Type != "correlation" || entries[1].AgentInstanceID != "" {
		t.Errorf("want an error entry then a correlation entry without agent id, got %+v", entries)
	}
}

func TestReadEntries_UndecodableAndBlankLines_AreSkipped(t *testing.T) {
	path := writeLogFile(t, `{"type":"response","agent_instance_id":"w#1","response":{"agent_instance_id":"w#1","status_code":"SUCCESS"}}`+"\n"+
		"\n"+
		"this is not json\n"+
		`{"type":"response","agent_instance_id":"w#2","response":{"agent_instance_id":"w#2","status_code":"BLOCKED","error_code":"E501"}}`+"\n"+
		`{"type":"respo`+"\n")

	entries, err := dispatchlog.ReadEntries(path)

	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(entries) != 2 || entries[0].AgentInstanceID != "w#1" || entries[1].AgentInstanceID != "w#2" {
		t.Fatalf("want the two decodable entries in order, got %+v", entries)
	}
	if entries[1].StatusCode != domain.StatusBLOCKED || entries[1].ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Errorf("want BLOCKED/E501 on the second entry, got %+v", entries[1])
	}
}

// A whole request or response is one line and can be far larger than a default
// line buffer; such a line must neither stop the read nor drop later entries.
func TestReadEntries_VeryLongLine_DoesNotDropItOrLaterEntries(t *testing.T) {
	huge := strings.Repeat("x", 600*1024)
	path := writeLogFile(t, `{"type":"response","agent_instance_id":"w#1","response":{"agent_instance_id":"w#1","status_code":"SUCCESS","status_message":"`+huge+`"}}`+"\n"+
		`{"type":"response","agent_instance_id":"w#2","response":{"agent_instance_id":"w#2","status_code":"BLOCKED","error_code":"E501"}}`+"\n")

	entries, err := dispatchlog.ReadEntries(path)

	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(entries) != 2 || entries[0].AgentInstanceID != "w#1" || entries[1].AgentInstanceID != "w#2" {
		t.Fatalf("want both entries in order, got %d entries: %+v", len(entries), entries)
	}
	if entries[1].StatusCode != domain.StatusBLOCKED {
		t.Errorf("want BLOCKED on the entry after the long line, got %+v", entries[1])
	}
}

func TestReadEntries_MissingFile_ReturnsAnError(t *testing.T) {
	_, err := dispatchlog.ReadEntries(filepath.Join(t.TempDir(), "absent.log"))

	if err == nil {
		t.Fatal("want an error for a log that cannot be opened, got nil")
	}
}

func TestRunLogPath_IsThePathTheLoggerWritesTo(t *testing.T) {
	dir := t.TempDir()
	l := dispatchlog.New(dir)
	l.SetRunID(readerRunID)
	l.LogError("worker#1", "x")
	l.Close()

	got := dispatchlog.RunLogPath(dir, readerRunID)

	if got == "" || got != l.Path() {
		t.Errorf("RunLogPath = %q, want the logger's path %q", got, l.Path())
	}
}

func TestIsRunLog_TellsARunsOwnLogFromOtherFiles(t *testing.T) {
	dir := t.TempDir()
	own := dispatchlog.RunLogPath(dir, readerRunID)
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"the run's own log", own, true},
		{"empty path", "", false},
		{"startup fallback file", filepath.Join(dir, "RunnerLogs", "startup-20261106T135514.000000000Z-dispatch.log"), false},
		{"another run's log", dispatchlog.RunLogPath(dir, "20261107T100000Z-aaaa"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dispatchlog.IsRunLog(c.path, readerRunID); got != c.want {
				t.Errorf("IsRunLog(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}
