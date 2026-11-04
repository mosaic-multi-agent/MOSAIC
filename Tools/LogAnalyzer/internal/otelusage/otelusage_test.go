package otelusage_test

// Tests for the adapter over the Go ingest package, on the tool-local fixtures
// in testdata/ingestotel and testdata/pendingusage (copied to a temporary
// folder before anything is written).
//
// Rules under test:
//   - IngestRun runs a final, whole-file ingest: spans before the stored cursor
//     are read too, pending spans end up unattributed, the export is untouched.
//   - Each non-success status of the ingest comes back as its own status with a
//     diagnostic of the matching kind.
//   - PendingUsage reads the state without touching it and tells an absent state,
//     a state without pending spans, pending spans and an unreadable state apart.

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/otelusage"
)

const (
	ingestFixture  = "../../testdata/ingestotel"
	pendingFixture = "../../testdata/pendingusage/runs"
	fixtureRun     = "20261031T100000Z-f001"
)

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// fixtureRunDir copies the fixture run to a temporary folder and returns it with
// the path of a copy of the export file.
func fixtureRunDir(t *testing.T) (runDir, export string) {
	t.Helper()
	tmp := t.TempDir()
	runDir = filepath.Join(tmp, "OrchestrationLogs", fixtureRun)
	copyTree(t, filepath.Join(ingestFixture, "runs", fixtureRun), runDir)
	data, err := os.ReadFile(filepath.Join(ingestFixture, "export.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	export = filepath.Join(tmp, "otel.jsonl")
	if err := os.WriteFile(export, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return runDir, export
}

// pendingInState counts the pending spans in the run's ingest state file,
// independently of the production probe.
func pendingInState(t *testing.T, runDir string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runDir, ".otel-ingest", "state.json"))
	if err != nil {
		t.Fatalf("read ingest state: %v", err)
	}
	var st struct {
		Pending []json.RawMessage `json:"pending"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parse ingest state: %v", err)
	}
	return len(st.Pending)
}

func diagnosticKinds(res domain.UsageIngestResult) map[string]bool {
	kinds := map[string]bool{}
	for _, d := range res.Diagnostics {
		kinds[d.Kind] = true
	}
	return kinds
}

func TestIngestRun_PendingRunAndWholeExport_PlacesEverythingAndFinalisesThePending(t *testing.T) {
	runDir, export := fixtureRunDir(t)

	res, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export})

	if err != nil {
		t.Fatalf("IngestRun: %v", err)
	}
	if res.Status != domain.IngestDone || res.RecordsWritten != 2 || res.Finalized != 1 || res.Pending != 0 {
		t.Errorf("result = %+v, want done with 2 written, 1 finalised, 0 pending (chat-orch-1 sits before the stored cursor; the recovery reads the whole file)", res)
	}
	if !diagnosticKinds(res)["pending_finalized"] {
		t.Errorf("diagnostics = %+v, want a pending_finalized entry", res.Diagnostics)
	}
}

func TestIngestRun_LeavesNoPendingUsage(t *testing.T) {
	runDir, export := fixtureRunDir(t)
	if got := pendingInState(t, runDir); got != 1 {
		t.Fatalf("fixture has %d pending records, want 1", got)
	}

	if _, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export}); err != nil {
		t.Fatalf("IngestRun: %v", err)
	}

	if got := pendingInState(t, runDir); got != 0 {
		t.Errorf("%d pending records in the state after the ingest, want 0", got)
	}
}

func TestIngestRun_SpanBeforeTheStoredCursor_IsPlacedInItsStream(t *testing.T) {
	runDir, export := fixtureRunDir(t)

	if _, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export}); err != nil {
		t.Fatalf("IngestRun: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(runDir, "00_orchestrator_events.jsonl"))
	if !bytes.Contains(data, []byte("chat-orch-1")) {
		t.Error("chat-orch-1, which lies before the stored cursor, was not placed: the whole file was not read")
	}
}

func TestIngestRun_CorruptStateFile_ReportsUnknownStateAndLeavesItAlone(t *testing.T) {
	runDir, export := fixtureRunDir(t)
	statePath := filepath.Join(runDir, ".otel-ingest", "state.json")
	corrupt := []byte("{not json")
	if err := os.WriteFile(statePath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export})

	if err != nil {
		t.Fatalf("IngestRun: %v", err)
	}
	if res.Status != domain.IngestUnknownState {
		t.Errorf("status = %v, want unknown state", res.Status)
	}
	if after, _ := os.ReadFile(statePath); !bytes.Equal(after, corrupt) {
		t.Error("the corrupt state file was modified")
	}
}

func TestIngestRun_ExportThatCannotBeRead_ReportsFailedWithAnError(t *testing.T) {
	runDir, _ := fixtureRunDir(t)
	asDirectory := t.TempDir()

	res, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: asDirectory})

	if res.Status != domain.IngestFailed || err == nil {
		t.Errorf("result = %+v, err = %v, want failed status with a non-nil error", res, err)
	}
}

func TestIngestRun_NeverModifiesTheExportFile(t *testing.T) {
	runDir, export := fixtureRunDir(t)
	before, _ := os.ReadFile(export)

	if _, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export}); err != nil {
		t.Fatalf("IngestRun: %v", err)
	}

	after, _ := os.ReadFile(export)
	if !bytes.Equal(before, after) {
		t.Error("the export file was modified")
	}
}

func TestIngestRun_MissingExport_ReportsFileMissingAndWritesNothing(t *testing.T) {
	runDir, export := fixtureRunDir(t)
	stateBefore, _ := os.ReadFile(filepath.Join(runDir, ".otel-ingest", "state.json"))

	res, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export + ".gone"})

	if err != nil {
		t.Fatalf("IngestRun: %v", err)
	}
	if res.Status != domain.IngestFileMissing || !diagnosticKinds(res)["file_missing"] {
		t.Errorf("result = %+v, want file missing with a file_missing diagnostic", res)
	}
	stateAfter, _ := os.ReadFile(filepath.Join(runDir, ".otel-ingest", "state.json"))
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Error("the ingest state changed for a missing export")
	}
}

func TestIngestRun_FreshLockOfAnotherIngest_ReportsLockHeld(t *testing.T) {
	runDir, export := fixtureRunDir(t)
	lock := `{"holder": "python/4242", "acquired_at": "` + time.Now().UTC().Format(time.RFC3339) + `"}`
	if err := os.WriteFile(filepath.Join(runDir, ".otel-ingest", "lock"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export})

	if err != nil {
		t.Fatalf("IngestRun: %v", err)
	}
	if res.Status != domain.IngestLockHeld || !diagnosticKinds(res)["lock_held"] {
		t.Errorf("result = %+v, want lock held with a lock_held diagnostic", res)
	}
	if _, statErr := os.Stat(filepath.Join(runDir, ".otel-ingest", "lock")); statErr != nil {
		t.Error("the lock of the other ingest was removed")
	}
}

func TestIngestRun_StateOfUnknownVersion_ReportsUnknownStateAndLeavesItAlone(t *testing.T) {
	runDir, export := fixtureRunDir(t)
	statePath := filepath.Join(runDir, ".otel-ingest", "state.json")
	unknown := []byte(`{"state_version": 2, "placements": {}, "pending": [], "links": {}}`)
	if err := os.WriteFile(statePath, unknown, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := otelusage.New().IngestRun(context.Background(), domain.UsageIngestRequest{RunDir: runDir, ExportPath: export})

	if err != nil {
		t.Fatalf("IngestRun: %v", err)
	}
	if res.Status != domain.IngestUnknownState || !diagnosticKinds(res)["unknown_state_version"] {
		t.Errorf("result = %+v, want unknown state with an unknown_state_version diagnostic", res)
	}
	if after, _ := os.ReadFile(statePath); !bytes.Equal(after, unknown) {
		t.Error("the state of unknown version was modified")
	}
}

func TestPendingUsage_ReadsPendingCountAndUnreadableStateFromRunFolders(t *testing.T) {
	cases := []struct {
		run  string
		want domain.UsageIngestState
	}{
		{"20261031T110000Z-c001", domain.UsageIngestState{PendingRecords: 2}},
		{"20261031T110000Z-c002", domain.UsageIngestState{}},
		{"20261031T110000Z-c003", domain.UsageIngestState{}},
		{"20261031T110000Z-c004", domain.UsageIngestState{StateUnreadable: true}},
	}
	adapter := otelusage.New()
	for _, c := range cases {
		t.Run(c.run, func(t *testing.T) {
			if got := adapter.PendingUsage(filepath.Join(pendingFixture, c.run)); got != c.want {
				t.Errorf("PendingUsage = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestPendingUsage_CorruptStateFile_IsUnreadable(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, ".otel-ingest"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, ".otel-ingest", "state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := otelusage.New().PendingUsage(runDir)

	if !got.StateUnreadable || got.PendingRecords != 0 {
		t.Errorf("PendingUsage = %+v, want unreadable with no count", got)
	}
}

func TestPendingUsage_ChangesNothingInTheRunFolder(t *testing.T) {
	runDir, _ := fixtureRunDir(t)
	statePath := filepath.Join(runDir, ".otel-ingest", "state.json")
	before, _ := os.ReadFile(statePath)

	otelusage.New().PendingUsage(runDir)

	after, _ := os.ReadFile(statePath)
	if !bytes.Equal(before, after) {
		t.Error("the state file changed")
	}
	if _, err := os.Stat(filepath.Join(runDir, ".otel-ingest", "lock")); err == nil {
		t.Error("the probe left a lock behind")
	}
}
