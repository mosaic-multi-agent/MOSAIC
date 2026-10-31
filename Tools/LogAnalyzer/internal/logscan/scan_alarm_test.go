package logscan_test

// Tests for the alarm source: logscan.Scanner reads the adapter diagnostics file
// in the logs root and reports transcript format-change alarms.
//
// Rules under test:
//   - Only transcript_format_changed lines with a valid run id are returned, in
//     file order, with run, harness, session, timestamp, path and 1-based line.
//   - Other kinds, lines without or with an invalid run id and malformed lines
//     are ignored; alarms for runs absent from the root are still returned.
//   - A single-run source reads the file in the run folder's parent directory.
//   - A missing file, a digest source and an unusable source yield nil.
//   - The diagnostics file is not mistaken for a run and raises no finding.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logscan"
)

const (
	alarmRunA = "20261031T100000Z-a001"
	alarmRunB = "20261031T100000Z-a002"
	alarmRunC = "20261031T100000Z-a003"
)

// alarmLogsRoot builds a logs root with two run folders and the fixture
// diagnostics file copied in.
func alarmLogsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	makeOrchestratorFile(t, makeRunFolder(t, root, alarmRunA))
	makeOrchestratorFile(t, makeRunFolder(t, root, alarmRunB))
	fixture := filepath.Join("..", "..", "testdata", "requestedtools", "alarms", "mosaic-logger-diagnostics.jsonl")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "mosaic-logger-diagnostics.jsonl"), raw, 0o600); err != nil {
		t.Fatalf("write diagnostics: %v", err)
	}
	return root
}

func TestTranscriptAlarms_LogsRoot_ReturnsOnlyValidFormatChangeAlarmsInFileOrder(t *testing.T) {
	root := alarmLogsRoot(t)
	s := logscan.NewScanner()

	got := s.TranscriptAlarms(context.Background(), s.Classify(root))

	if len(got) != 3 {
		t.Fatalf("alarms = %+v, want 3 (runs a001, a003, a001)", got)
	}
	wantRuns := []string{alarmRunA, alarmRunC, alarmRunA}
	wantSessions := []string{"s1", "s5", "s6"}
	wantLines := []int{1, 6, 7}
	for i, a := range got {
		if a.Run != domain.NamedRun(wantRuns[i]) || a.SessionID != wantSessions[i] || a.Line != wantLines[i] {
			t.Errorf("alarm %d = run %+v session %q line %d, want %s %s line %d",
				i, a.Run, a.SessionID, a.Line, wantRuns[i], wantSessions[i], wantLines[i])
		}
		if a.Harness != domain.HarnessGHCPCLI {
			t.Errorf("alarm %d harness = %q, want ghcp-cli", i, a.Harness)
		}
		if a.Path != filepath.Join(root, "mosaic-logger-diagnostics.jsonl") {
			t.Errorf("alarm %d path = %q, want the diagnostics file", i, a.Path)
		}
	}
	if want := time.Date(2026, 10, 31, 10, 5, 0, 0, time.UTC); !got[0].At.Equal(want) {
		t.Errorf("alarm 0 time = %v, want %v", got[0].At, want)
	}
}

func TestTranscriptAlarms_SingleRun_ReadsTheFileInTheParentDirectory(t *testing.T) {
	root := alarmLogsRoot(t)
	s := logscan.NewScanner()
	src := domain.Source{Kind: domain.SourceSingleRun, Path: filepath.Join(root, alarmRunA)}

	got := s.TranscriptAlarms(context.Background(), src)

	if len(got) != 3 {
		t.Errorf("alarms = %d, want 3 read from the logs root above the run folder", len(got))
	}
}

func TestTranscriptAlarms_WithoutDiagnosticsFile_IsNil(t *testing.T) {
	root := t.TempDir()
	makeOrchestratorFile(t, makeRunFolder(t, root, alarmRunA))
	s := logscan.NewScanner()

	if got := s.TranscriptAlarms(context.Background(), s.Classify(root)); got != nil {
		t.Errorf("alarms = %+v, want nil", got)
	}
}

func TestTranscriptAlarms_OtherSourceKinds_AreNil(t *testing.T) {
	root := alarmLogsRoot(t)
	diag := filepath.Join(root, "mosaic-logger-diagnostics.jsonl")
	s := logscan.NewScanner()
	for name, src := range map[string]domain.Source{
		"digest file":  {Kind: domain.SourceDigestFile, Path: diag},
		"digest dir":   {Kind: domain.SourceDigestDir, Path: root},
		"unusable":     {Kind: domain.SourceUnusable, Path: root},
		"not found":    {Kind: domain.SourceNotFound, Path: root},
		"missing root": {Kind: domain.SourceLogsRoot, Path: filepath.Join(root, "nope")},
	} {
		t.Run(name, func(t *testing.T) {
			if got := s.TranscriptAlarms(context.Background(), src); got != nil {
				t.Errorf("alarms = %+v, want nil", got)
			}
		})
	}
}

func TestEnumerate_LogsRootWithDiagnosticsFile_RaisesNoFindingAndNoExtraRun(t *testing.T) {
	root := alarmLogsRoot(t)
	s := logscan.NewScanner()

	inv, findings := s.Enumerate(s.Classify(root))

	if len(inv.Runs) != 2 {
		t.Errorf("runs = %d, want 2: the diagnostics file is not a run", len(inv.Runs))
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none", findings)
	}
}
