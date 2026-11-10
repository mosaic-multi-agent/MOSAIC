package otelingest_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	"mosaic-common/otelingest"
)

// A run without an ingest state reports nothing present.
func TestReadPendingStatusWithoutState(t *testing.T) {
	r := newIngestRun(t)

	got := otelingest.ReadPendingStatus(r.runDir)

	if got != (otelingest.PendingStatus{}) {
		t.Errorf("status = %+v, want the zero value", got)
	}
	if r.exists(r.lockPath()) || r.exists(r.statePath()) {
		t.Errorf("the query created ingest files")
	}
}

// Pending usage left by an ingest is visible to the query; once placed it is gone.
func TestReadPendingStatusCountsPendingSpans(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("orphan-1", testSession, "missing-parent"), chatLine("orphan-2", testSession, "missing-parent"),
		chatLine("o1", testSession, ""))
	if res := r.ingest(false); res.Pending != 2 {
		t.Fatalf("ingest left %d pending, want 2", res.Pending)
	}

	got := otelingest.ReadPendingStatus(r.runDir)

	if want := (otelingest.PendingStatus{StatePresent: true, StateKnown: true, Pending: 2}); got != want {
		t.Errorf("status = %+v, want %+v", got, want)
	}

	if res := r.ingest(true); res.Finalized != 2 {
		t.Fatalf("final ingest finalised %d, want 2", res.Finalized)
	}
	if got := otelingest.ReadPendingStatus(r.runDir); got.Pending != 0 || !got.StateKnown {
		t.Errorf("after the final ingest: %+v, want known state with 0 pending", got)
	}
}

// The query neither takes the lock nor changes the state, and works while another ingest holds the lock.
func TestReadPendingStatusIsReadOnlyAndIgnoresLock(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("orphan-1", testSession, "missing-parent"))
	r.ingest(false)
	stateBefore := r.read(t, r.statePath())
	info, err := os.Stat(r.statePath())
	if err != nil {
		t.Fatal(err)
	}
	lock := lockContent(fixedNow.Add(-time.Minute))
	r.writeLock(t, lock)

	got := otelingest.ReadPendingStatus(r.runDir)

	if got.Pending != 1 || !got.StateKnown {
		t.Errorf("status = %+v, want 1 pending in a known state", got)
	}
	if !bytes.Equal(stateBefore, r.read(t, r.statePath())) {
		t.Errorf("the query changed state.json")
	}
	after, err := os.Stat(r.statePath())
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(info.ModTime()) {
		t.Errorf("the query rewrote state.json")
	}
	if string(r.read(t, r.lockPath())) != lock {
		t.Errorf("the query touched the lock")
	}
}

// A state this version cannot read, or one that is not valid JSON, is reported as present but not known.
func TestReadPendingStatusUnreadableState(t *testing.T) {
	for name, content := range map[string]string{
		"unknown version": `{"state_version": 2, "pending": [{"x": 1}]}`,
		"not json":        `{"state_version": 1, "pend`,
	} {
		t.Run(name, func(t *testing.T) {
			r := newIngestRun(t)
			if err := os.MkdirAll(r.runDir+"/.otel-ingest", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(r.statePath(), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			got := otelingest.ReadPendingStatus(r.runDir)

			if want := (otelingest.PendingStatus{StatePresent: true}); got != want {
				t.Errorf("status = %+v, want %+v", got, want)
			}
		})
	}
}
