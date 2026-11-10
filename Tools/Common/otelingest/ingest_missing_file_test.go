package otelingest_test

import (
	"bytes"
	"testing"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"
)

// A missing OTel file writes nothing and names the cause; the harness creates the file at its first export, and
// an ingest after that works normally.
func TestMissingOTelFileThenLateFile(t *testing.T) {
	r := newIngestRun(t)
	logBefore := r.read(t, r.orchestratorLog())

	res := r.ingest(true)

	if res.Status != otelingest.StatusFileMissing {
		t.Fatalf("status = %q, want %q", res.Status, otelingest.StatusFileMissing)
	}
	if !hasDiagnostic(res, diag.KindFileMissing) {
		t.Errorf("no %q diagnostic: %+v", diag.KindFileMissing, res.Diagnostics)
	}
	if res.RecordsWritten != 0 || res.Pending != 0 {
		t.Errorf("a missing file produced work: records %d, pending %d", res.RecordsWritten, res.Pending)
	}
	if r.exists(r.statePath()) || r.exists(r.lockPath()) {
		t.Errorf("a missing file left ingest state behind")
	}
	if !bytes.Equal(logBefore, r.read(t, r.orchestratorLog())) {
		t.Errorf("a missing file changed the logs of the run")
	}

	r.writeOTel(t, chatLine("o1", testSession, ""), chatLine("o2", testSession, ""))
	res = r.ingest(false)

	if res.Status != otelingest.StatusIngested || res.RecordsWritten != 2 || res.LinesRead != 2 {
		t.Errorf("late file: status %q records %d lines %d, want ingested, 2, 2", res.Status, res.RecordsWritten, res.LinesRead)
	}
	if hasDiagnostic(res, diag.KindFileMissing) {
		t.Errorf("file_missing reported although the file exists")
	}
	if !r.exists(r.statePath()) {
		t.Errorf("no state written for the late file")
	}
}
