package otelingest_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/otelingest"
)

// isStreamFile reports whether a path relative to the run folder is a stream that may hold ingested records.
func isStreamFile(rel string) bool {
	parts := strings.Split(rel, "/")
	switch {
	case len(parts) == 1:
		return parts[0] == "00_orchestrator_events.jsonl" || parts[0] == "00_unattributed_events.jsonl"
	case len(parts) == 2:
		return parts[1] == "03_events.jsonl" && parts[0][0] != '.'
	}
	return false
}

// readOTelRecords returns the usage_record lines with source "otel" of a JSONL stream as data without timestamp.
func readOTelRecords(t *testing.T, path string) []any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var recs []any
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			t.Fatalf("%s: a stream line is not a whole JSON object: %v", path, err)
		}
		if obj["event"] == "usage_record" && obj["source"] == "otel" {
			delete(obj, "timestamp")
			recs = append(recs, obj)
		}
	}
	return recs
}

// collectStreams reads the OTel usage records of every selected stream file under root, keyed by relative path.
func collectStreams(t *testing.T, root string, selected func(rel string) bool) map[string][]any {
	t.Helper()
	out := map[string][]any{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return out
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if !selected(rel) {
			return nil
		}
		if recs := readOTelRecords(t, p); len(recs) > 0 {
			out[rel] = recs
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collect streams under %s: %v", root, err)
	}
	return out
}

func assertJSONEqual(t *testing.T, what string, want, got any) {
	t.Helper()
	if reflect.DeepEqual(want, got) {
		return
	}
	w, _ := json.MarshalIndent(want, "", "  ")
	g, _ := json.MarshalIndent(got, "", "  ")
	t.Errorf("%s differs\nwant:\n%s\ngot:\n%s", what, w, g)
}

// toData round-trips a Go value through JSON so it compares with decoded fixture files.
func toData(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertStreams(t *testing.T, w *caseWorkspace, step string) {
	t.Helper()
	want := collectStreams(t, filepath.Join(w.caseDir, "expected", step, "streams"), func(string) bool { return true })
	got := collectStreams(t, w.runDir, isStreamFile)
	assertJSONEqual(t, "ingested usage_record streams", toData(t, want), toData(t, got))
}

func assertState(t *testing.T, w *caseWorkspace, step string) {
	t.Helper()
	statePath := filepath.Join(w.runDir, ".otel-ingest", "state.json")
	wantPath := filepath.Join(w.caseDir, "expected", step, "state.json")
	if _, err := os.Stat(wantPath); os.IsNotExist(err) {
		if _, err := os.Stat(statePath); err == nil {
			t.Errorf("state.json exists but the case expects no state file")
		}
		return
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("state.json was not written: %v", err)
		return
	}
	var want, got any
	readJSONInto(t, wantPath, &want)
	readJSONInto(t, statePath, &got)
	assertJSONEqual(t, ".otel-ingest/state.json", want, got)
}

func assertResult(t *testing.T, w *caseWorkspace, step string, res otelingest.Result) {
	t.Helper()
	if res.Err != nil {
		t.Errorf("ingest failed: %v", res.Err)
	}
	var want any
	readJSONInto(t, filepath.Join(w.caseDir, "expected", step, "result.json"), &want)
	got := map[string]any{
		"status": string(res.Status), "records_written": res.RecordsWritten, "finalized": res.Finalized,
		"pending": res.Pending, "lines_read": res.LinesRead, "lines_malformed": res.LinesMalformed,
	}
	assertJSONEqual(t, "result", want, toData(t, got))
}

func assertDiagnostics(t *testing.T, w *caseWorkspace, step string, res otelingest.Result) {
	t.Helper()
	want := []any{}
	wantPath := filepath.Join(w.caseDir, "expected", step, "diagnostics.json")
	if _, err := os.Stat(wantPath); err == nil {
		readJSONInto(t, wantPath, &want)
	}
	got := []any{}
	for _, d := range res.Diagnostics {
		entry := map[string]any{"kind": string(d.Kind)}
		if len(d.Counts) > 0 {
			entry["counts"] = d.Counts
		}
		if len(d.IDs) > 0 {
			entry["ids"] = d.IDs
		}
		got = append(got, entry)
	}
	assertJSONEqual(t, "diagnostics", want, toData(t, got))
}

// assertLogsAppendOnly checks that the ingest only appended to the MOSAIC log files that were there before it
// and wrote nothing to unknown-run/.
func assertLogsAppendOnly(t *testing.T, w *caseWorkspace) {
	t.Helper()
	for rel, before := range w.logsAtRun {
		after, err := os.ReadFile(filepath.Join(w.runDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s: existing log file is gone: %v", rel, err)
			continue
		}
		if !bytes.HasPrefix(after, before) {
			t.Errorf("%s: existing log content was changed, ingest may only append whole lines", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(w.logsRoot, "unknown-run")); err == nil {
		t.Errorf("ingest wrote to unknown-run/")
	}
}
