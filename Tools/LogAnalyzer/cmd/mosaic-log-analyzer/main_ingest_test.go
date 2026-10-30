package main

// Composition-root wiring of the OTel usage ingest. main() exits the process, so
// these tests build the real binary and run it on temporary copies of the
// tool-local fixtures in testdata/ingestotel and testdata/pendingusage.
//
// Rules under test:
//   - ingest-otel is a known subcommand backed by a real ingester: it places the
//     usage of the fixture export and leaves nothing pending.
//   - total on a run with pending usage reports it, which needs the pending
//     usage probe to be wired.

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	name := "mosaic-log-analyzer-under-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func copyDir(t *testing.T, src, dst string) {
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

func runBinary(t *testing.T, bin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runBinaryIn(t, bin, "", t.TempDir(), args...)
}

// runBinaryIn runs the binary in dir ("" = the test's own folder) with the user
// config folder pointed at configBase, so no run reads or writes the real one.
func runBinaryIn(t *testing.T, bin, dir, configBase string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), configEnv(configBase)...)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run binary: %v", err)
	}
	return code, out.String(), errOut.String()
}

func TestBinary_IngestOtel_PlacesUsageOfTheFixtureExportAndLeavesNothingPending(t *testing.T) {
	bin := buildBinary(t)
	tmp := t.TempDir()
	logs := filepath.Join(tmp, "OrchestrationLogs")
	copyDir(t, filepath.Join("..", "..", "testdata", "ingestotel", "runs"), logs)
	export := filepath.Join(tmp, "otel.jsonl")
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ingestotel", "export.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(export, data, 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runBinary(t, bin, "ingest-otel", "--path", logs, "--run", "20261031T100000Z-f001", export)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stdout: %q, stderr: %q)", code, stdout, stderr)
	}
	if want := "2 records written, 1 finalised as unattributed, 0 pending"; !strings.Contains(stdout, want) {
		t.Errorf("stdout = %q, want it to contain %q", stdout, want)
	}
}

func TestBinary_Total_RunWithPendingUsage_ReportsPendingUsageRecords(t *testing.T) {
	bin := buildBinary(t)
	logs := filepath.Join(t.TempDir(), "logs")
	copyDir(t, filepath.Join("..", "..", "testdata", "pendingusage", "runs"), logs)

	code, stdout, stderr := runBinary(t, bin, "total", "--format", "json", "--path", logs, "--run", "20261031T110000Z-c001")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("total output is not JSON: %v\n%s", err, stdout)
	}
	if got["pending_usage_records"] != float64(2) {
		t.Errorf("total = %v, want pending_usage_records 2", got)
	}
}
