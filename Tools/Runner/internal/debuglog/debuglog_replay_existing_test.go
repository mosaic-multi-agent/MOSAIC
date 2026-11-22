package debuglog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
)

// ============================================================
// Replay into an existing run log (resumed run)
// ============================================================

// seedExistingRunLog creates RunnerLogs/<runID>/<runID>.log with the given
// content, as left behind by an earlier process of the same run, and returns
// its path.
func seedExistingRunLog(t *testing.T, workDir, runID string, content []byte) string {
	t.Helper()
	runDir := filepath.Join(workDir, LogsFolderName, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("seedExistingRunLog: MkdirAll(%q): %v", runDir, err)
	}
	p := filepath.Join(runDir, runID+".log")
	if err := os.WriteFile(p, content, 0644); err != nil {
		t.Fatalf("seedExistingRunLog: WriteFile(%q): %v", p, err)
	}
	return p
}

func TestLogger_Replay_ExistingRunLog_KeepsOldContentAndAppendsBufferedEntries(t *testing.T) {
	// A resumed run already has a run log. The buffered pre-identity entries
	// must land after the old bytes; the old bytes stay exactly as they were.
	workDir := t.TempDir()
	oldContent := []byte("[2026-08-05T14:00:00Z] earlier process entry\nsecond old line\n")
	runLogPath := seedExistingRunLog(t, workDir, validRunID, oldContent)

	logger := New(workDir)
	logger.Log(domain.EventRunnerStart, "first buffered entry")
	logger.Log(domain.EventSessionDispatchStart, "second buffered entry")
	buffered, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", logger.Path(), err)
	}

	logger.SetRunID(validRunID)
	logger.Close()

	got, err := os.ReadFile(runLogPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", runLogPath, err)
	}
	want := append(append([]byte{}, oldContent...), buffered...)
	if !bytes.Equal(got, want) {
		t.Errorf("run log must be old content followed by buffered entries\nwant:\n%s\ngot:\n%s", want, got)
	}
	if gotPath := logger.Path(); gotPath != runLogPath {
		t.Errorf("Path() after replay = %q, want %q", gotPath, runLogPath)
	}
}

func TestLogger_Replay_ExistingRunLog_WithoutTrailingNewline_StillKeepsOldBytesAsPrefix(t *testing.T) {
	// The old bytes are never rewritten, even when they do not end in a newline.
	workDir := t.TempDir()
	oldContent := []byte("old line without trailing newline")
	runLogPath := seedExistingRunLog(t, workDir, validRunID, oldContent)

	logger := New(workDir)
	logger.Log(domain.EventRunnerStart, "buffered entry")
	buffered, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", logger.Path(), err)
	}

	logger.SetRunID(validRunID)
	logger.Close()

	got, err := os.ReadFile(runLogPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", runLogPath, err)
	}
	want := append(append([]byte{}, oldContent...), buffered...)
	if !bytes.Equal(got, want) {
		t.Errorf("run log must start with the old bytes and end with the buffered entries\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestLogger_Replay_ExistingRunLog_RemovesOutOfRunFile_AfterSuccess(t *testing.T) {
	workDir := t.TempDir()
	seedExistingRunLog(t, workDir, validRunID, []byte("old\n"))

	logger := New(workDir)
	logger.Log(domain.EventRunnerStart, "buffered entry")
	outOfRunPath := logger.Path()

	logger.SetRunID(validRunID)
	logger.Close()

	if _, err := os.Stat(outOfRunPath); !os.IsNotExist(err) {
		t.Errorf("out-of-run file %q must be removed after a successful replay; stat err = %v", outOfRunPath, err)
	}
}

func TestLogger_Replay_ExistingRunLog_LaterEntriesFollowReplayedEntries(t *testing.T) {
	// After a successful replay into an existing log, entries logged later
	// continue after the replayed entries; old content stays the prefix.
	workDir := t.TempDir()
	oldContent := []byte("old line\n")
	runLogPath := seedExistingRunLog(t, workDir, validRunID, oldContent)

	logger := New(workDir)
	logger.Log(domain.EventRunnerStart, "buffered entry")
	logger.SetRunID(validRunID)
	logger.Log(domain.EventSessionDispatchStart, "entry after replay")
	logger.Close()

	got, err := os.ReadFile(runLogPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", runLogPath, err)
	}
	if !bytes.HasPrefix(got, oldContent) {
		t.Errorf("old content must remain the prefix of the run log\ngot:\n%q", got)
	}
	s := string(got[len(oldContent):])
	iBuf := indexOf(s, "buffered entry")
	iLate := indexOf(s, "entry after replay")
	if iBuf < 0 || iLate < 0 || iBuf > iLate {
		t.Errorf("buffered entry must precede the later entry after the old content\ntail:\n%s", s)
	}
}

func TestLogger_Replay_ExistingRunLogCannotBeOpened_LeavesItAndOutOfRunFileIntact(t *testing.T) {
	// Regression coverage: an existing run log that cannot be opened for
	// writing (read-only) must survive a failed replay byte-for-byte, must
	// not be removed, and the out-of-run file must keep receiving entries.
	// Write and sync failures after a successful open cannot be provoked
	// without production hooks and are not covered here.
	workDir := t.TempDir()
	oldContent := []byte("old line one\nold line two\n")
	runLogPath := seedExistingRunLog(t, workDir, validRunID, oldContent)
	if err := os.Chmod(runLogPath, 0444); err != nil {
		t.Fatalf("Chmod(%q): %v", runLogPath, err)
	}
	t.Cleanup(func() { _ = os.Chmod(runLogPath, 0644) })
	if f, err := os.OpenFile(runLogPath, os.O_WRONLY|os.O_APPEND, 0); err == nil {
		_ = f.Close()
		t.Skip("read-only file can still be opened for writing in this environment (for example running as root)")
	}

	logger := New(workDir)
	logger.Log(domain.EventRunnerStart, "buffered entry")
	outOfRunPath := logger.Path()
	bufferedBefore, err := os.ReadFile(outOfRunPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", outOfRunPath, err)
	}

	logger.SetRunID(validRunID)
	logger.Log(domain.EventSessionDispatchStart, "entry after failed replay")
	logger.Close()

	got, err := os.ReadFile(runLogPath)
	if err != nil {
		t.Fatalf("existing run log must not be removed by a failed replay: %v", err)
	}
	if !bytes.Equal(got, oldContent) {
		t.Errorf("existing run log changed after failed replay\nwant:\n%q\ngot:\n%q", oldContent, got)
	}
	if gotPath := logger.Path(); gotPath != outOfRunPath {
		t.Errorf("Path() after failed replay = %q, want unchanged %q", gotPath, outOfRunPath)
	}
	after, err := os.ReadFile(outOfRunPath)
	if err != nil {
		t.Fatalf("out-of-run file must survive a failed replay: %v", err)
	}
	if !bytes.HasPrefix(after, bufferedBefore) {
		t.Errorf("out-of-run file must keep its buffered entries as prefix\nbefore:\n%q\nafter:\n%q", bufferedBefore, after)
	}
}

// indexOf returns the index of sub in s, or -1.
func indexOf(s, sub string) int {
	return bytes.Index([]byte(s), []byte(sub))
}
