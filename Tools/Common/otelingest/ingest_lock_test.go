package otelingest_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"
)

func lockContent(acquired time.Time) string {
	return fmt.Sprintf(`{"holder":"python/4242","acquired_at":%q}`, acquired.UTC().Format("2006-01-02T15:04:05.000Z"))
}

// An ingest that finds a fresh lock skips its turn: nothing is read or written and the lock is left alone.
func TestIngestSkipsWhenLockHeld(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	lock := lockContent(fixedNow.Add(-time.Minute))
	r.writeLock(t, lock)
	logBefore := r.read(t, r.orchestratorLog())

	res := r.ingest(false)

	if res.Status != otelingest.StatusLockHeld {
		t.Fatalf("status = %q, want %q", res.Status, otelingest.StatusLockHeld)
	}
	if res.RecordsWritten != 0 || res.LinesRead != 0 {
		t.Errorf("a skipped ingest reported work: records %d, lines %d", res.RecordsWritten, res.LinesRead)
	}
	if !hasDiagnostic(res, diag.KindLockHeld) {
		t.Errorf("no %q diagnostic: %+v", diag.KindLockHeld, res.Diagnostics)
	}
	if !bytes.Equal(logBefore, r.read(t, r.orchestratorLog())) {
		t.Errorf("a skipped ingest changed the logs of the run")
	}
	if r.exists(r.statePath()) {
		t.Errorf("a skipped ingest wrote state.json")
	}
	if got := string(r.read(t, r.lockPath())); got != lock {
		t.Errorf("the lock of the other ingest was touched: %q", got)
	}
}

// The skipped work is not lost: once the lock is gone the next ingest reads everything.
func TestIngestAfterLockReleasedReadsSkippedWork(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	r.writeLock(t, lockContent(fixedNow.Add(-time.Minute)))
	if res := r.ingest(false); res.Status != otelingest.StatusLockHeld {
		t.Fatalf("status = %q, want lock_held", res.Status)
	}
	if err := os.Remove(r.lockPath()); err != nil {
		t.Fatal(err)
	}

	res := r.ingest(false)

	if res.Status != otelingest.StatusIngested || res.RecordsWritten != 1 {
		t.Errorf("status %q records %d, want ingested with 1 record", res.Status, res.RecordsWritten)
	}
}

// A lock left behind by a crashed ingest is broken once it is older than LockStaleAfter.
func TestIngestBreaksStaleLock(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	r.writeLock(t, lockContent(fixedNow.Add(-otelingest.LockStaleAfter-time.Minute)))

	res := r.ingest(false)

	if res.Status != otelingest.StatusIngested || res.RecordsWritten != 1 {
		t.Fatalf("status %q records %d, want ingested with 1 record", res.Status, res.RecordsWritten)
	}
	if r.exists(r.lockPath()) {
		t.Errorf("the lock is still there after the ingest")
	}
}

// A lock just inside LockStaleAfter still blocks.
func TestIngestRespectsLockJustInsideStaleAge(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	r.writeLock(t, lockContent(fixedNow.Add(-otelingest.LockStaleAfter+time.Minute)))

	if res := r.ingest(false); res.Status != otelingest.StatusLockHeld {
		t.Errorf("status = %q, want %q", res.Status, otelingest.StatusLockHeld)
	}
}

// A lock exactly LockStaleAfter old is already stale: it blocks only while its age is below the limit.
func TestIngestBreaksLockAtExactlyStaleAge(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	r.writeLock(t, lockContent(fixedNow.Add(-otelingest.LockStaleAfter)))

	res := r.ingest(false)

	if res.Status != otelingest.StatusIngested || res.RecordsWritten != 1 {
		t.Errorf("status %q records %d, want ingested with 1 record", res.Status, res.RecordsWritten)
	}
}

// Without a readable acquired_at the age comes from the modification time of the lock file.
func TestIngestLockAgeFallsBackToFileModTime(t *testing.T) {
	for _, tc := range []struct {
		name string
		age  time.Duration
		want otelingest.Status
	}{
		{"fresh", time.Minute, otelingest.StatusLockHeld},
		{"stale", time.Hour, otelingest.StatusIngested},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newIngestRun(t)
			r.writeOTel(t, chatLine("o1", testSession, ""))
			r.writeLock(t, "{}")
			mod := fixedNow.Add(-tc.age)
			if err := os.Chtimes(r.lockPath(), mod, mod); err != nil {
				t.Fatal(err)
			}
			if res := r.ingest(false); res.Status != tc.want {
				t.Errorf("status = %q, want %q", res.Status, tc.want)
			}
		})
	}
}

// Every path releases the lock it took: after a normal ingest and after one refused for an unknown state version.
func TestIngestReleasesLockOnEveryPath(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	if res := r.ingest(false); res.Status != otelingest.StatusIngested {
		t.Fatalf("status = %q", res.Status)
	}
	if r.exists(r.lockPath()) {
		t.Errorf("lock left behind after a normal ingest")
	}

	unknown := `{"state_version": 9, "placements": {}, "pending": [], "links": {}}`
	if err := os.WriteFile(r.statePath(), []byte(unknown), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := r.ingest(false); res.Status != otelingest.StatusUnknownState {
		t.Fatalf("status = %q, want unknown_state", res.Status)
	}
	if r.exists(r.lockPath()) {
		t.Errorf("lock left behind after an unknown-state refusal")
	}
	if got := string(r.read(t, r.statePath())); got != unknown {
		t.Errorf("the unknown state was rewritten: %q", got)
	}
}

// The lock is also released when the OTel file does not exist. Regression guard: it passes while no lock is taken.
func TestIngestReleasesLockWhenOTelFileMissing(t *testing.T) {
	r := newIngestRun(t)

	if res := r.ingest(false); res.Status != otelingest.StatusFileMissing {
		t.Fatalf("status = %q, want %q", res.Status, otelingest.StatusFileMissing)
	}
	if r.exists(r.lockPath()) {
		t.Errorf("lock left behind after a missing-file ingest")
	}
}

// Concurrent ingests are serialised: whatever the interleaving, each span is written exactly once.
func TestConcurrentIngestsWriteEachRecordOnce(t *testing.T) {
	r := newIngestRun(t)
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, chatLine(fmt.Sprintf("o%02d", i), testSession, ""))
	}
	r.writeOTel(t, lines...)

	var wg sync.WaitGroup
	results := make([]otelingest.Result, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = otelingest.Ingest(otelingest.Request{RunDir: r.runDir, OTelPath: r.otelPath})
		}()
	}
	wg.Wait()

	ingested := 0
	for _, res := range results {
		switch res.Status {
		case otelingest.StatusIngested:
			ingested++
		case otelingest.StatusLockHeld:
		default:
			t.Errorf("unexpected status %q (err %v)", res.Status, res.Err)
		}
	}
	if ingested == 0 {
		t.Fatalf("no ingest got the lock")
	}
	if res := otelingest.Ingest(otelingest.Request{RunDir: r.runDir, OTelPath: r.otelPath}); res.Status != otelingest.StatusIngested {
		t.Fatalf("follow-up ingest status = %q", res.Status)
	}
	if n := strings.Count(string(r.read(t, r.orchestratorLog())), `"event":"usage_record"`); n != 20 {
		t.Errorf("orchestrator stream holds %d usage records, want 20", n)
	}
}
