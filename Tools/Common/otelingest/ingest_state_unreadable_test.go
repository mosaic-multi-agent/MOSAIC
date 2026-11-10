package otelingest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"
)

// A state.json that is not parseable is treated like one of an unknown version: nothing is read or written, the
// lock is released and the cause is reported.
func TestIngestRefusesUnparseableState(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	if err := os.MkdirAll(filepath.Dir(r.statePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	garbage := "this is { not json"
	if err := os.WriteFile(r.statePath(), []byte(garbage), 0o644); err != nil {
		t.Fatal(err)
	}
	logBefore := r.read(t, r.orchestratorLog())

	res := r.ingest(false)

	if res.Status != otelingest.StatusUnknownState {
		t.Fatalf("status = %q, want %q", res.Status, otelingest.StatusUnknownState)
	}
	if res.RecordsWritten != 0 || res.LinesRead != 0 {
		t.Errorf("a refused ingest reported work: records %d, lines %d", res.RecordsWritten, res.LinesRead)
	}
	if !hasDiagnostic(res, diag.KindUnknownStateVersion) {
		t.Errorf("no %q diagnostic: %+v", diag.KindUnknownStateVersion, res.Diagnostics)
	}
	if got := string(r.read(t, r.statePath())); got != garbage {
		t.Errorf("the unreadable state was rewritten: %q", got)
	}
	if !bytes.Equal(logBefore, r.read(t, r.orchestratorLog())) {
		t.Errorf("a refused ingest changed the logs of the run")
	}
	if r.exists(r.lockPath()) {
		t.Errorf("lock left behind after refusing an unreadable state")
	}
}
