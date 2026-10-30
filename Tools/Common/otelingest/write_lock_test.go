package otelingest_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"
)

// orchestratorLockFile is the append lock of the orchestrator stream, as the log format defines it.
func (r *ingestRun) orchestratorLockFile() string {
	return filepath.Join(r.runDir, ".00_orchestrator_events.jsonl.lock")
}

func (r *ingestRun) holdAppendLock(t *testing.T) {
	t.Helper()
	f, err := os.OpenFile(r.orchestratorLockFile(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("take append lock: %v", err)
	}
	f.Close()
}

// bigChatLine is a root chat span of the run's session whose model name makes the written line long.
func bigChatLine(id string, modelSize int) string {
	model := strings.Repeat("m", modelSize)
	return fmt.Sprintf(`{"type":"span","traceId":"0123456789abcdef0123456789abcdef","spanId":%q,"name":"chat","kind":2,"attributes":{"gen_ai.operation.name":"chat","gen_ai.conversation.id":%q,"gen_ai.response.model":%q,"gen_ai.usage.input_tokens":10,"gen_ai.usage.output_tokens":1},"status":{"code":0},"events":[]}`, id, testSession, model)
}

func usageRecordCount(t *testing.T, data []byte) int {
	t.Helper()
	n := 0
	for _, l := range bytes.Split(data, []byte("\n")) {
		if bytes.Contains(l, []byte(`"usage_record"`)) {
			n++
		}
	}
	return n
}

func TestOTelWriterWaitsForHeldAppendLockThenWrites(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	r.holdAppendLock(t)
	go func() {
		time.Sleep(600 * time.Millisecond)
		_ = os.Remove(r.orchestratorLockFile())
	}()
	start := time.Now()

	res := r.ingest(false)

	if elapsed := time.Since(start); elapsed < 450*time.Millisecond {
		t.Errorf("ingest finished after %v without waiting for the append lock", elapsed)
	}
	if res.Status != otelingest.StatusIngested || res.RecordsWritten != 1 {
		t.Fatalf("status %q, records %d, want ingested with 1 record (err %v)", res.Status, res.RecordsWritten, res.Err)
	}
	if usageRecordCount(t, r.read(t, r.orchestratorLog())) != 1 {
		t.Error("usage_record not in the orchestrator stream")
	}
	if r.exists(r.orchestratorLockFile()) {
		t.Error("append lock file left behind")
	}
}

func TestOTelWriterDropsLineAndReportsDiagnosticWhenLockTimesOut(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	r.holdAppendLock(t)
	logBefore := r.read(t, r.orchestratorLog())
	start := time.Now()

	res := r.ingest(false)

	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("ingest took %v, the lock wait must be bounded", elapsed)
	}
	if res.Err != nil || res.Status != otelingest.StatusIngested {
		t.Fatalf("status %q, err %v: a lock timeout must not fail the ingest", res.Status, res.Err)
	}
	if res.RecordsWritten != 0 || res.Pending != 1 {
		t.Errorf("records %d, pending %d, want the dropped call kept pending", res.RecordsWritten, res.Pending)
	}
	if !bytes.Equal(logBefore, r.read(t, r.orchestratorLog())) {
		t.Error("a timed-out append changed the stream")
	}
	var found *diag.Diagnostic
	for i := range res.Diagnostics {
		if res.Diagnostics[i].Kind == diag.KindAppendTimedOut {
			found = &res.Diagnostics[i]
		}
	}
	if found == nil || found.Counts["lines"] != 1 {
		t.Errorf("want one %q diagnostic counting 1 line, got %+v", diag.KindAppendTimedOut, res.Diagnostics)
	}
}

func TestOTelWriterWritesDroppedLineOnLaterIngest(t *testing.T) {
	r := newIngestRun(t)
	r.writeOTel(t, chatLine("o1", testSession, ""))
	r.holdAppendLock(t)
	_ = r.ingest(false)
	if err := os.Remove(r.orchestratorLockFile()); err != nil {
		t.Fatal(err)
	}

	res := r.ingest(false)

	if res.RecordsWritten != 1 || usageRecordCount(t, r.read(t, r.orchestratorLog())) != 1 {
		t.Errorf("records %d: the line dropped by the timeout was not written by the next ingest", res.RecordsWritten)
	}
}

// While another writer holds the append lock, a multi-line ingest of long lines must write
// nothing; after release every line lands whole. Without the lock the lines appear at once.
func TestOTelWriterWritesNothingWhileLockHeldThenKeepsEveryLineWhole(t *testing.T) {
	r := newIngestRun(t)
	const spans = 6
	var lines []string
	for i := 0; i < spans; i++ {
		lines = append(lines, bigChatLine(fmt.Sprintf("o%d", i), 150*1024))
	}
	r.writeOTel(t, lines...)
	r.holdAppendLock(t)
	logBefore := r.read(t, r.orchestratorLog())
	done := make(chan otelingest.Result, 1)
	go func() { done <- r.ingest(false) }()

	time.Sleep(500 * time.Millisecond)
	if !bytes.Equal(logBefore, r.read(t, r.orchestratorLog())) {
		t.Error("the stream changed while the append lock was held by someone else")
	}
	if err := os.Remove(r.orchestratorLockFile()); err != nil {
		t.Fatal(err)
	}
	res := <-done

	if res.RecordsWritten != spans {
		t.Fatalf("records written %d, want %d (status %q, diagnostics %+v)", res.RecordsWritten, spans, res.Status, res.Diagnostics)
	}
	f, err := os.Open(r.orchestratorLog())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	count := 0
	for sc.Scan() {
		var obj map[string]any
		if err := json.Unmarshal(sc.Bytes(), &obj); err != nil {
			t.Fatalf("line %d is blank or torn: %v", count, err)
		}
		count++
	}
	if want := 1 + spans; count != want {
		t.Errorf("stream has %d lines, want %d", count, want)
	}
}
