package cli_test

// End-to-end tests of the "usage not fully ingested" report on the tool-local
// fixtures in testdata/pendingusage, with the real scanner, reader and ingest
// state probe, driven through cli.Execute. The fixtures are read in place.
//
// Fixture runs (each with one orchestrator usage record of 1000 input tokens):
//
//	c001  ingest state holds 2 pending spans
//	c002  ingest state holds none (everything placed)
//	c003  no ingest state at all
//	c004  ingest state of a version this tool does not know
//
// Rules under test:
//   - Only c001 and c004 carry a notice; c001 names the count and the
//     ingest-otel command for its run, c004 says the state is unreadable.
//   - total keeps its committed keys; the two new keys appear only for c001
//     (count) and c004 (flag only).
//   - The notice never changes a figure.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/otelusage"
	"mosaic-log-analyzer/internal/pricing"
)

const (
	pendingLogs    = "../../testdata/pendingusage/runs"
	pendingRun     = "20261031T110000Z-c001"
	placedRun      = "20261031T110000Z-c002"
	noStateRun     = "20261031T110000Z-c003"
	unreadableRun  = "20261031T110000Z-c004"
	pendingWordage = "usage not fully ingested"
)

// pricedRoot returns a runtime root whose pricing file prices the fixture
// model, so total raises no unpriced-model key for these runs.
func pricedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "MosaicLogAnalyzer", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	const yaml = "models:\n  claude-sonnet-4:\n    input: \"3.00\"\n    cache_read: \"0.30\"\n" +
		"    cache_write_5m: \"3.75\"\n    cache_write_1h: \"6.00\"\n    output: \"15.00\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pricing.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write pricing file: %v", err)
	}
	return root
}

func pendingE2ERun(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	probe := otelusage.New()
	svc := app.New(app.Deps{
		Source: logscan.NewScanner(), Reader: logread.NewReader(),
		Pricing: pricing.NewStore(pricedRoot(t)), Clock: newFakeClock(),
		Interaction: cli.NewInteraction(&bytes.Buffer{}, cli.Answers{}),
		WorkDir:     t.TempDir(), IngestProbe: probe, Ingester: probe,
	})
	var out, errOut bytes.Buffer
	code = cli.Execute(cli.Options{Service: svc, Stdout: &out, Stderr: &errOut}, args)
	return code, out.String(), errOut.String()
}

func pendingTotal(t *testing.T, runID string) map[string]any {
	t.Helper()
	code, stdout, stderr := pendingE2ERun(t, "total", "--format", "json", "--path", pendingLogs, "--run", runID)
	if code != cli.ExitSuccess {
		t.Fatalf("total %s exited %d: %s", runID, code, stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("total output is not JSON: %v\n%s", err, stdout)
	}
	return m
}

func TestE2EPendingUsage_TextReport_NoticeOnlyOnRunsWithPendingOrUnreadableState(t *testing.T) {
	code, out, stderr := pendingE2ERun(t, "report", "--format", "text", "--path", pendingLogs)
	if code != cli.ExitSuccess {
		t.Fatalf("report exited %d: %s", code, stderr)
	}
	section := func(runID string) string {
		i := strings.Index(out, "Run: "+runID)
		if i < 0 {
			t.Fatalf("text report lacks run %s:\n%s", runID, out)
		}
		return sectionOf(out, i)
	}

	pending := section(pendingRun)
	if !strings.Contains(pending, pendingWordage) || !strings.Contains(pending, "2 records pending") ||
		!strings.Contains(pending, "mosaic-log-analyzer ingest-otel --run "+pendingRun) {
		t.Errorf("section of %s lacks the notice, the count or the command:\n%s", pendingRun, pending)
	}
	unreadable := section(unreadableRun)
	if !strings.Contains(unreadable, pendingWordage+": ingest state unreadable by this version") ||
		strings.Contains(unreadable, "ingest-otel") {
		t.Errorf("section of %s lacks the unreadable-state notice or points at ingest-otel:\n%s", unreadableRun, unreadable)
	}
	for _, id := range []string{placedRun, noStateRun} {
		if strings.Contains(section(id), "not fully ingested") {
			t.Errorf("section of %s carries a notice:\n%s", id, section(id))
		}
	}
}

func TestE2EPendingUsage_JSONReport_UsageIngestOnlyOnRunsWithPendingOrUnreadableState(t *testing.T) {
	runs := e2eRunsOf(e2eReportWith(t, pendingLogs))

	ing, ok := runs[pendingRun]["usage_ingest"].(map[string]any)
	if !ok || ing["pending_records"] != float64(2) {
		t.Errorf("%s usage_ingest = %v, want pending_records 2", pendingRun, runs[pendingRun]["usage_ingest"])
	}
	un, ok := runs[unreadableRun]["usage_ingest"].(map[string]any)
	if !ok || un["state_unreadable"] != true {
		t.Errorf("%s usage_ingest = %v, want state_unreadable true", unreadableRun, runs[unreadableRun]["usage_ingest"])
	}
	for _, id := range []string{placedRun, noStateRun} {
		if _, present := runs[id]["usage_ingest"]; present {
			t.Errorf("%s has a usage_ingest key", id)
		}
	}
}

// e2eReportWith is e2eReport with the ingest state probe wired.
func e2eReportWith(t *testing.T, path string) map[string]any {
	t.Helper()
	code, stdout, stderr := pendingE2ERun(t, "report", "--format", "json", "--path", path)
	if code != cli.ExitSuccess {
		t.Fatalf("report on %s exited %d: %s", path, code, stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("report output is not JSON: %v", err)
	}
	return m
}

func TestE2EPendingUsage_Total_PendingRunAddsTheCountAndKeepsTheCommittedKeys(t *testing.T) {
	total := pendingTotal(t, pendingRun)

	assertKeySet(t, "total", sortedKeys(total), []string{
		"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
		"usage_not_fully_ingested", "pending_usage_records",
	})
	if total["usage_not_fully_ingested"] != true || total["pending_usage_records"] != float64(2) {
		t.Errorf("total = %v, want usage_not_fully_ingested true and pending_usage_records 2", total)
	}
	if in := total["tokens"].(map[string]any)["input"]; in != float64(1000) {
		t.Errorf("tokens.input = %v, want 1000: pending spans add no figures", in)
	}
	if total["schema_version"] != "1" {
		t.Errorf("schema_version = %v, want \"1\"", total["schema_version"])
	}
}

func TestE2EPendingUsage_Total_UnreadableStateFlagsItWithoutACount(t *testing.T) {
	total := pendingTotal(t, unreadableRun)

	if total["usage_not_fully_ingested"] != true {
		t.Errorf("total = %v, want usage_not_fully_ingested true", total)
	}
	if _, present := total["pending_usage_records"]; present {
		t.Errorf("total has a pending_usage_records key for an unreadable state: %v", total)
	}
}

func TestE2EPendingUsage_Total_RunsWithoutPendingUsageKeepExactlyTheCommittedKeys(t *testing.T) {
	for _, id := range []string{placedRun, noStateRun} {
		total := pendingTotal(t, id)

		assertKeySet(t, "total "+id, sortedKeys(total), []string{
			"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
		})
	}
}
