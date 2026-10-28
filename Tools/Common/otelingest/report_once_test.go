package otelingest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"
)

func noticesPath(runDir string) string {
	return filepath.Join(runDir, ".otel-ingest", "notices.json")
}

func newNoticeRunDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "OrchestrationLogs", "20261028T000000Z-0000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReportOnceFirstCallYesRepeatNo(t *testing.T) {
	dir := newNoticeRunDir(t)
	if !otelingest.ReportOnce(dir, diag.KindVariableUnset) {
		t.Fatal("first call for a run and kind must report true")
	}
	for i := 0; i < 3; i++ {
		if otelingest.ReportOnce(dir, diag.KindVariableUnset) {
			t.Fatalf("repeat call %d for the same run and kind must report false", i+1)
		}
	}
}

func TestReportOnceKindsAreIndependent(t *testing.T) {
	dir := newNoticeRunDir(t)
	if !otelingest.ReportOnce(dir, diag.KindVariableUnset) {
		t.Fatal("variable_unset first call must be true")
	}
	if !otelingest.ReportOnce(dir, diag.KindFileMissing) {
		t.Fatal("file_missing first call must be true although variable_unset was reported")
	}
	if otelingest.ReportOnce(dir, diag.KindVariableUnset) || otelingest.ReportOnce(dir, diag.KindFileMissing) {
		t.Fatal("both kinds were reported already and must now be false")
	}
}

func TestReportOnceRunsAreIndependent(t *testing.T) {
	a, b := newNoticeRunDir(t), newNoticeRunDir(t)
	if !otelingest.ReportOnce(a, diag.KindFileMissing) {
		t.Fatal("first run first call must be true")
	}
	if !otelingest.ReportOnce(b, diag.KindFileMissing) {
		t.Fatal("a different run must get its own first notice")
	}
	if otelingest.ReportOnce(a, diag.KindFileMissing) {
		t.Fatal("first run repeat must be false")
	}
}

func TestReportOnceRecordHasDocumentedShape(t *testing.T) {
	dir := newNoticeRunDir(t)
	otelingest.ReportOnce(dir, diag.KindVariableUnset)
	otelingest.ReportOnce(dir, diag.KindFileMissing)
	data, err := os.ReadFile(noticesPath(dir))
	if err != nil {
		t.Fatalf("notices.json must exist next to the ingest state: %v", err)
	}
	var rec struct {
		Reported map[string]string `json:"reported"`
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("notices.json is not valid JSON of the documented shape: %v", err)
	}
	if len(rec.Reported) != 2 {
		t.Fatalf("want 2 reported kinds, got %v", rec.Reported)
	}
	for _, k := range []diag.Kind{diag.KindVariableUnset, diag.KindFileMissing} {
		ts, ok := rec.Reported[string(k)]
		if !ok || ts == "" {
			t.Fatalf("kind %q missing or without time in %v", k, rec.Reported)
		}
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil || len(top) != 1 {
		t.Fatalf("notices.json must have exactly the member \"reported\", got %v", top)
	}
}

func TestReportOnceKeepsEarlierKindsAcrossCalls(t *testing.T) {
	dir := newNoticeRunDir(t)
	otelingest.ReportOnce(dir, diag.KindVariableUnset)
	first, err := os.ReadFile(noticesPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	otelingest.ReportOnce(dir, diag.KindFileMissing)
	var before, after struct {
		Reported map[string]string `json:"reported"`
	}
	_ = json.Unmarshal(first, &before)
	data, _ := os.ReadFile(noticesPath(dir))
	_ = json.Unmarshal(data, &after)
	if after.Reported[string(diag.KindVariableUnset)] != before.Reported[string(diag.KindVariableUnset)] ||
		before.Reported[string(diag.KindVariableUnset)] == "" {
		t.Fatalf("first-reported time of an earlier kind must be preserved: before %v after %v", before, after)
	}
}

// An unreadable record must still report yes (duplicate preferred over lost).
func TestReportOnceUnreadableRecordReportsYes(t *testing.T) {
	dir := newNoticeRunDir(t)
	// A directory where the record file should be: it can be neither read nor replaced as a file.
	if err := os.MkdirAll(noticesPath(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if !otelingest.ReportOnce(dir, diag.KindVariableUnset) {
			t.Fatalf("call %d with an unreadable record must report true", i+1)
		}
	}
}

func TestReportOnceCorruptRecordReportsYes(t *testing.T) {
	dir := newNoticeRunDir(t)
	if err := os.MkdirAll(filepath.Dir(noticesPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(noticesPath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !otelingest.ReportOnce(dir, diag.KindFileMissing) {
		t.Fatal("a corrupt record cannot prove an earlier report, so the call must report true")
	}
}

// An unwritable location must report yes every time.
func TestReportOnceUnwritableLocationReportsYes(t *testing.T) {
	dir := newNoticeRunDir(t)
	// A plain file where the .otel-ingest directory should be: nothing can be created inside.
	if err := os.WriteFile(filepath.Join(dir, ".otel-ingest"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if !otelingest.ReportOnce(dir, diag.KindFileMissing) {
			t.Fatalf("call %d with an unwritable record must report true", i+1)
		}
	}
}

func TestReportOnceMissingRunFolderReportsYes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-such-run")
	if !otelingest.ReportOnce(dir, diag.KindVariableUnset) {
		t.Fatal("must report true when the record cannot be written")
	}
}
