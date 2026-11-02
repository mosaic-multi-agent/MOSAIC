package main

// Composition-root wiring of the transcript format-change alarm source. main()
// exits the process, so this test builds the real binary and runs it on a
// temporary logs root made from tool-local fixtures.
//
// Rule under test:
//   - A run named in a transcript_format_changed entry of the adapter
//     diagnostics file in the logs root shows a transcript_format_changed
//     finding in the JSON report, and a run without an entry does not.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	alarmedRunID = "20261031T100000Z-a001"
	cleanRunID   = "20261031T100000Z-a002"
)

// writeAlarmRun writes a minimal finished run folder under root.
func writeAlarmRun(t *testing.T, root, runID string) {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "pendingusage", "runs", "20261031T110000Z-c003", "00_orchestrator_events.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "00_orchestrator_events.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBinary_Report_RunWithFormatChangeAlarm_ShowsTheFindingOnlyForThatRun(t *testing.T) {
	bin := buildBinary(t)
	logs := filepath.Join(t.TempDir(), "logs")
	writeAlarmRun(t, logs, alarmedRunID)
	writeAlarmRun(t, logs, cleanRunID)
	diag, err := os.ReadFile(filepath.Join("..", "..", "testdata", "requestedtools", "alarms", "mosaic-logger-diagnostics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "mosaic-logger-diagnostics.jsonl"), diag, 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runBinary(t, bin, "report", "--format", "json", "--path", logs)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr)
	}
	var got struct {
		Quality struct {
			Findings []struct {
				Kind  string `json:"kind"`
				RunID string `json:"run_id"`
			} `json:"findings"`
		} `json:"quality"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("report output is not JSON: %v\n%s", err, stdout)
	}
	alarmed, clean := 0, 0
	for _, f := range got.Quality.Findings {
		if f.Kind != "transcript_format_changed" {
			continue
		}
		switch f.RunID {
		case alarmedRunID:
			alarmed++
		case cleanRunID:
			clean++
		}
	}
	if alarmed != 1 {
		t.Errorf("format-change findings for the alarmed run = %d, want 1 (findings: %+v)", alarmed, got.Quality.Findings)
	}
	if clean != 0 {
		t.Errorf("format-change findings for the run without an entry = %d, want 0", clean)
	}
}
