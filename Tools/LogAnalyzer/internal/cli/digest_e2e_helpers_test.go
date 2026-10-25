package cli_test

// Helpers for the end-to-end digest tests: real scanner, reader, pricing and
// digest adapters on the tool-local fixtures in testdata/digest, driven through
// cli.Execute so the whole path from log file to report JSON is exercised.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/pricing"
)

// Run ids in the testdata/digest/logs fixture.
const (
	e2eRichRun    = "20261020T090000Z-a001" // sessions, notifications, tool calls, compaction, per-record usage
	e2eLegacyRun  = "20261020T100000Z-a002" // second harness, legacy usage, an unpriced model
	e2eLossyRun   = "20261021T090000Z-a003" // one malformed line
	e2eCleanRun   = "20261021T100000Z-a004"
	e2eFixtureDir = "../../testdata/digest"
)

const e2ePricingYAML = `
models:
  claude-opus-4:
    input: "15.00"
    cache_read: "1.50"
    cache_write_5m: "18.75"
    cache_write_1h: "30.00"
    output: "75.00"
  claude-sonnet-4:
    input: "3.00"
    cache_read: "0.30"
    cache_write_5m: "3.75"
    cache_write_1h: "6.00"
    output: "15.00"
`

// e2eFixture returns the path of a fixture under testdata/digest.
func e2eFixture(parts ...string) string {
	return filepath.Join(append([]string{e2eFixtureDir}, parts...)...)
}

// e2eService builds a service on the real adapters. The pricing file lives in a
// per-test temporary root, never in the user's configuration.
func e2eService(t *testing.T) *app.Service {
	t.Helper()
	root := t.TempDir()
	cfg := filepath.Join(root, filepath.FromSlash(pricing.ConfigRelPath))
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatalf("create pricing dir: %v", err)
	}
	if err := os.WriteFile(cfg, []byte(e2ePricingYAML), 0o644); err != nil {
		t.Fatalf("write pricing file: %v", err)
	}
	return app.New(app.Deps{
		Source:      logscan.NewScanner(),
		Reader:      logread.NewReader(),
		Pricing:     pricing.NewStore(root),
		Digests:     digest.NewStore(),
		Clock:       newFakeClock(),
		Interaction: cli.NewInteraction(&bytes.Buffer{}, cli.Answers{}),
		WorkDir:     t.TempDir(),
		ToolVersion: "e2e",
	})
}

// e2eRun runs the CLI against the real adapters.
func e2eRun(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.Execute(cli.Options{Service: e2eService(t), Stdout: &out, Stderr: &errOut}, args)
	return code, out.String(), errOut.String()
}

// e2eExport exports path (optionally one run) to a fresh digest file and returns its path.
func e2eExport(t *testing.T, path, runID string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "export.digest.json")
	args := []string{"export", "--path", path, "--out", out}
	if runID != "" {
		args = append(args, "--run", runID)
	}
	code, _, stderr := e2eRun(t, args...)
	if code != cli.ExitSuccess {
		t.Fatalf("export %v exited %d: %s", args, code, stderr)
	}
	return out
}

// e2eReport returns `report --format json` for path as a generic JSON object.
func e2eReport(t *testing.T, path string) map[string]any {
	t.Helper()
	code, stdout, stderr := e2eRun(t, "report", "--format", "json", "--path", path)
	if code != cli.ExitSuccess {
		t.Fatalf("report on %s exited %d: %s", path, code, stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("report output is not JSON: %v", err)
	}
	return m
}

// e2eRunsOf returns the report's named runs keyed by run id.
func e2eRunsOf(report map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	runs, _ := report["runs"].([]any)
	for _, r := range runs {
		run := r.(map[string]any)
		out[run["run_id"].(string)] = run
	}
	return out
}

// e2eFindingsOf returns the quality findings of a report.
func e2eFindingsOf(report map[string]any) []map[string]any {
	q, _ := report["quality"].(map[string]any)
	list, _ := q["findings"].([]any)
	var out []map[string]any
	for _, f := range list {
		out = append(out, f.(map[string]any))
	}
	return out
}

// e2eFindingsOfKind returns the findings of one kind.
func e2eFindingsOfKind(report map[string]any, kind domain.FindingKind) []map[string]any {
	var out []map[string]any
	for _, f := range e2eFindingsOf(report) {
		if f["kind"] == string(kind) {
			out = append(out, f)
		}
	}
	return out
}

// e2eIncomplete returns the report's quality.incomplete flag.
func e2eIncomplete(report map[string]any) bool {
	q, _ := report["quality"].(map[string]any)
	v, _ := q["incomplete"].(bool)
	return v
}

// e2eFirstDiff returns "" when a and b are deeply equal, else the path of the first difference.
func e2eFirstDiff(path string, a, b any) string {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return path + ": type differs"
		}
		keys := map[string]bool{}
		for k := range av {
			keys[k] = true
		}
		for k := range bv {
			keys[k] = true
		}
		var sorted []string
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			if d := e2eFirstDiff(path+"."+k, av[k], bv[k]); d != "" {
				return d
			}
		}
		return ""
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return fmt.Sprintf("%s: length or type differs", path)
		}
		for i := range av {
			if d := e2eFirstDiff(fmt.Sprintf("%s[%d]", path, i), av[i], bv[i]); d != "" {
				return d
			}
		}
		return ""
	}
	if fmt.Sprint(a) != fmt.Sprint(b) || (a == nil) != (b == nil) {
		return fmt.Sprintf("%s: %v != %v", path, a, b)
	}
	return ""
}

// e2eNormalised returns a copy of the report with the fields the round trip may
// change removed: the source, and each finding's path, line and detail. Findings
// become a sorted multiset of kind, severity, actor and run id; the run id is
// dropped for kinds that named no run in the reference report (runlessKinds),
// because an export attributes those by path.
func e2eNormalised(report map[string]any, runlessKinds map[string]bool) map[string]any {
	raw, _ := json.Marshal(report)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	delete(m, "source")
	q, _ := m["quality"].(map[string]any)
	var keys []any
	for _, f := range e2eFindingsOf(m) {
		run := fmt.Sprint(f["run_id"])
		if runlessKinds[fmt.Sprint(f["kind"])] {
			run = ""
		}
		keys = append(keys, fmt.Sprintf("%v|%v|%v|%s", f["kind"], f["severity"], f["actor"], run))
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].(string) < keys[j].(string) })
	q["findings"] = keys
	return m
}

// e2eRunlessKinds returns the finding kinds that carry no run id in report.
func e2eRunlessKinds(report map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, f := range e2eFindingsOf(report) {
		if _, has := f["run_id"]; !has {
			out[fmt.Sprint(f["kind"])] = true
		}
	}
	return out
}

// e2eReadFile returns the file content or fails the test.
func e2eReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// e2eTokens returns one of a run's total token counts (nil when not provided).
func e2eTokens(run map[string]any, name string) any {
	totals := run["totals"].(map[string]any)
	return totals["tokens"].(map[string]any)[name]
}
