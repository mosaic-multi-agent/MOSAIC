package cli_test

// End-to-end tests of ingest-otel on the tool-local fixtures in
// testdata/ingestotel, through cli.Execute with the real scanner, reader and
// ingest adapter. Every test works on a temporary copy of the run folder.
//
// Fixture (run 20261031T100000Z-f001, harness vscode-ghcp, session vsc-session-6,
// invocation Implementation#1 with spawn id toolu_A):
//   - run folder state: one pending usage span (chat-sub-1) from an earlier,
//     non-final ingest; the stored cursor sits after line 1 of the export, so
//     chat-orch-1 (line 1, neither placed nor pending) is only reached by
//     reading the whole file
//   - export.jsonl: chat-orch-1 (root span of the orchestrator session),
//     chat-sub-1 (chat session unknown, stays pending until a final ingest) and
//     chat-foreign-1 (a conversation that is not part of this run)
//
// Rules under test:
//   - After the command the run has nothing pending, chat-orch-1 sits in the
//     orchestrator stream, chat-sub-1 in the unattributed stream, and the
//     foreign span nowhere.
//   - Repeating the command changes no file.
//   - The export file never changes.
//   - A missing export, a held lock and an unknown state version leave the run
//     folder untouched and exit with their documented codes.
//   - The report and total say "usage not fully ingested" before the command and
//     nothing about it after.

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/otelusage"
	"mosaic-log-analyzer/internal/pricing"
)

const (
	ingestFixtureDir = "../../testdata/ingestotel"
	ingestE2ERun     = "20261031T100000Z-f001"
)

// ingestE2E is a temporary logs root holding a copy of the fixture run and a
// copy of the export file.
type ingestE2E struct {
	logsRoot string
	runDir   string
	export   string
	env      map[string]string
}

func newIngestE2E(t *testing.T) *ingestE2E {
	t.Helper()
	tmp := t.TempDir()
	e := &ingestE2E{
		logsRoot: filepath.Join(tmp, "OrchestrationLogs"),
		export:   filepath.Join(tmp, "copilot-otel.jsonl"),
		env:      map[string]string{},
	}
	e.runDir = filepath.Join(e.logsRoot, ingestE2ERun)
	copyTree(t, filepath.Join(ingestFixtureDir, "runs"), e.logsRoot)
	data, err := os.ReadFile(filepath.Join(ingestFixtureDir, "export.jsonl"))
	if err != nil {
		t.Fatalf("read export fixture: %v", err)
	}
	if err := os.WriteFile(e.export, data, 0o644); err != nil {
		t.Fatalf("write export copy: %v", err)
	}
	return e
}

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

// snapshot returns the content of every file under dir keyed by relative path.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}

func assertSameSnapshot(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	for name, content := range before {
		got, ok := after[name]
		if !ok {
			t.Errorf("%s: file %s disappeared", what, name)
		} else if got != content {
			t.Errorf("%s: file %s changed", what, name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			t.Errorf("%s: file %s appeared", what, name)
		}
	}
}

// service builds the real service with the ingest adapter on both ports.
func (e *ingestE2E) service(t *testing.T) *app.Service {
	t.Helper()
	adapter := otelusage.New()
	return app.New(app.Deps{
		Source: logscan.NewScanner(), Reader: logread.NewReader(),
		Pricing: pricing.NewStore(t.TempDir(), t.TempDir()), Clock: newFakeClock(),
		Interaction: cli.NewInteraction(&bytes.Buffer{}, cli.Answers{}),
		WorkDir:     t.TempDir(), Ingester: adapter, IngestProbe: adapter,
	})
}

func (e *ingestE2E) run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.Execute(cli.Options{
		Service: e.service(t), Stdout: &out, Stderr: &errOut,
		Getenv: func(k string) string { return e.env[k] },
	}, args)
	return code, out.String(), errOut.String()
}

func (e *ingestE2E) ingest(t *testing.T, extra ...string) (code int, stdout, stderr string) {
	t.Helper()
	args := append([]string{"ingest-otel", "--path", e.logsRoot, "--run", ingestE2ERun}, extra...)
	return e.run(t, args...)
}

// usageRecords returns the otel usage_record lines of one stream file of the run, as objects.
func (e *ingestE2E) usageRecords(t *testing.T, rel string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.runDir, filepath.FromSlash(rel)))
	if err != nil {
		return nil
	}
	var out []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["event"] == "usage_record" && m["source"] == "otel" {
			out = append(out, m)
		}
	}
	return out
}

func recordIDs(records []map[string]any) []string {
	var ids []string
	for _, r := range records {
		ids = append(ids, r["record_id"].(string))
	}
	return ids
}

func (e *ingestE2E) pendingRecords(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.runDir, ".otel-ingest", "state.json"))
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

func TestE2EIngestOtel_PathArgument_PlacesUsageAndLeavesNothingPending(t *testing.T) {
	e := newIngestE2E(t)
	if e.pendingRecords(t) != 1 {
		t.Fatalf("fixture run has %d pending records before the command, want 1", e.pendingRecords(t))
	}

	code, stdout, stderr := e.ingest(t, e.export)

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr: %q)", code, cli.ExitSuccess, stderr)
	}
	if want := "run " + ingestE2ERun + ": 2 records written, 1 finalised as unattributed, 0 pending"; strings.TrimSpace(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if n := e.pendingRecords(t); n != 0 {
		t.Errorf("%d records pending after the command, want 0", n)
	}
	if ids := recordIDs(e.usageRecords(t, "00_orchestrator_events.jsonl")); len(ids) != 1 || ids[0] != "chat-orch-1" {
		t.Errorf("orchestrator stream records = %v, want [chat-orch-1]", ids)
	}
	if ids := recordIDs(e.usageRecords(t, "00_unattributed_events.jsonl")); len(ids) != 1 || ids[0] != "chat-sub-1" {
		t.Errorf("unattributed stream records = %v, want [chat-sub-1]", ids)
	}
	for name, content := range snapshot(t, e.runDir) {
		if strings.Contains(content, "chat-foreign-1") {
			t.Errorf("span of a conversation outside the run was written to %s", name)
		}
	}
}

func TestE2EIngestOtel_EnvironmentVariable_GivesTheSameResultAsTheArgument(t *testing.T) {
	e := newIngestE2E(t)
	e.env[ingestEnvVar] = e.export

	code, _, stderr := e.ingest(t)

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr: %q)", code, cli.ExitSuccess, stderr)
	}
	if n := e.pendingRecords(t); n != 0 {
		t.Errorf("%d records pending after the command, want 0", n)
	}
	if ids := recordIDs(e.usageRecords(t, "00_unattributed_events.jsonl")); len(ids) != 1 {
		t.Errorf("unattributed stream records = %v, want one", ids)
	}
}

func TestE2EIngestOtel_Repeated_ChangesNothing(t *testing.T) {
	e := newIngestE2E(t)
	initial := snapshot(t, e.runDir)
	if code, _, stderr := e.ingest(t, e.export); code != cli.ExitSuccess {
		t.Fatalf("first run exit = %d (stderr: %q)", code, stderr)
	}
	before := snapshot(t, e.runDir)
	if len(before) == len(initial) && func() bool {
		for k, v := range before {
			if initial[k] != v {
				return false
			}
		}
		return true
	}() {
		t.Fatal("the first run changed nothing, so the repeat proves nothing")
	}

	code, stdout, stderr := e.ingest(t, e.export)

	if code != cli.ExitSuccess {
		t.Fatalf("second run exit = %d (stderr: %q)", code, stderr)
	}
	if want := "run " + ingestE2ERun + ": 0 records written, 0 finalised as unattributed, 0 pending"; strings.TrimSpace(stdout) != want {
		t.Errorf("second run stdout = %q, want %q", stdout, want)
	}
	assertSameSnapshot(t, "repeated ingest", before, snapshot(t, e.runDir))
}

func TestE2EIngestOtel_NeverModifiesTheExportFile(t *testing.T) {
	e := newIngestE2E(t)
	before, _ := os.ReadFile(e.export)

	if code, _, stderr := e.ingest(t, e.export); code != cli.ExitSuccess {
		t.Fatalf("exit = %d (stderr: %q)", code, stderr)
	}

	after, _ := os.ReadFile(e.export)
	if !bytes.Equal(before, after) {
		t.Error("the export file was modified")
	}
}

func TestE2EIngestOtel_MissingExportFile_ExitsUsageAndLeavesTheRunUntouched(t *testing.T) {
	e := newIngestE2E(t)
	before := snapshot(t, e.runDir)

	code, _, stderr := e.ingest(t, filepath.Join(filepath.Dir(e.export), "no-such-file.jsonl"))

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr, "file_missing") {
		t.Errorf("stderr = %q, want the file_missing diagnostic", stderr)
	}
	assertSameSnapshot(t, "missing export", before, snapshot(t, e.runDir))
}

func TestE2EIngestOtel_UnknownStateVersion_ExitsSkippedAndLeavesTheRunUntouched(t *testing.T) {
	e := newIngestE2E(t)
	statePath := filepath.Join(e.runDir, ".otel-ingest", "state.json")
	unknown := `{"state_version": 2, "cursor": {"offset": 0, "head_len": 0, "head_sha256": "x"}, "placements": {}, "pending": [], "links": {}}`
	if err := os.WriteFile(statePath, []byte(unknown), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, e.runDir)

	code, _, stderr := e.ingest(t, e.export)

	if code != cli.ExitIngestSkipped {
		t.Errorf("exit code = %d, want %d", code, cli.ExitIngestSkipped)
	}
	if !strings.Contains(stderr, "unknown_state_version") {
		t.Errorf("stderr = %q, want the unknown_state_version diagnostic", stderr)
	}
	assertSameSnapshot(t, "unknown state version", before, snapshot(t, e.runDir))
}

func TestE2EIngestOtel_LockHeldByAnotherIngest_ExitsSkippedAndLeavesTheRunUntouched(t *testing.T) {
	e := newIngestE2E(t)
	lock := `{"holder": "python/4242", "acquired_at": "` + time.Now().UTC().Format(time.RFC3339) + `"}`
	if err := os.WriteFile(filepath.Join(e.runDir, ".otel-ingest", "lock"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, e.runDir)

	code, _, stderr := e.ingest(t, e.export)

	if code != cli.ExitIngestSkipped {
		t.Errorf("exit code = %d, want %d", code, cli.ExitIngestSkipped)
	}
	if !strings.Contains(stderr, "lock_held") {
		t.Errorf("stderr = %q, want the lock_held diagnostic", stderr)
	}
	assertSameSnapshot(t, "held lock", before, snapshot(t, e.runDir))
}

func TestE2EIngestOtel_UnknownRun_ExitsNoData(t *testing.T) {
	e := newIngestE2E(t)

	code, _, stderr := e.run(t, "ingest-otel", "--path", e.logsRoot, "--run", "20990101T000000Z-zzzz", e.export)

	if code != cli.ExitNoData {
		t.Errorf("exit code = %d, want %d (stderr: %q)", code, cli.ExitNoData, stderr)
	}
}

func TestE2EIngestOtel_ReportAndTotalNameThePendingUsageUntilItIsIngested(t *testing.T) {
	e := newIngestE2E(t)
	_, textBefore, _ := e.run(t, "report", "--format", "text", "--path", e.logsRoot)
	_, totalBefore, _ := e.run(t, "total", "--format", "json", "--path", e.logsRoot, "--run", ingestE2ERun)

	if code, _, stderr := e.ingest(t, e.export); code != cli.ExitSuccess {
		t.Fatalf("ingest exit = %d (stderr: %q)", code, stderr)
	}
	_, textAfter, _ := e.run(t, "report", "--format", "text", "--path", e.logsRoot)
	_, totalAfter, _ := e.run(t, "total", "--format", "json", "--path", e.logsRoot, "--run", ingestE2ERun)

	if !strings.Contains(textBefore, "usage not fully ingested") ||
		!strings.Contains(textBefore, "ingest-otel --run "+ingestE2ERun) {
		t.Errorf("report before the ingest lacks the notice and the pointer:\n%s", textBefore)
	}
	if strings.Contains(textAfter, "not fully ingested") {
		t.Errorf("report after the ingest still shows the notice:\n%s", textAfter)
	}
	var before, after map[string]any
	if err := json.Unmarshal([]byte(totalBefore), &before); err != nil {
		t.Fatalf("total before is not JSON: %v\n%s", err, totalBefore)
	}
	if err := json.Unmarshal([]byte(totalAfter), &after); err != nil {
		t.Fatalf("total after is not JSON: %v\n%s", err, totalAfter)
	}
	if before["usage_not_fully_ingested"] != true || before["pending_usage_records"] != float64(1) {
		t.Errorf("total before = %v, want usage_not_fully_ingested true and pending_usage_records 1", before)
	}
	if _, present := after["usage_not_fully_ingested"]; present {
		t.Errorf("total after the ingest still has usage_not_fully_ingested: %v", after)
	}
	if _, present := after["pending_usage_records"]; present {
		t.Errorf("total after the ingest still has pending_usage_records: %v", after)
	}
}
