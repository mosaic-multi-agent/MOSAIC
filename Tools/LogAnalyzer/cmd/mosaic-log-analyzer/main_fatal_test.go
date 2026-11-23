package main

// Binary-level behaviour of the failure file in CLI mode: an unexpected failure
// (exit code 1) writes one in the global config folder (the Log Analyzer's
// documented per-user location) and adds one stderr line naming it, with the exit
// code unchanged; success and documented outcome codes write none.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const failureLinePrefix = "failure report written to "

// binaryFailureFiles lists the failure files in the global config folder of a run
// whose user config folder was configBase.
func binaryFailureFiles(t *testing.T, configBase string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(globalDirUnder(configBase), "mosaic-*-failure-*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// writeMalformedGlobalPricing makes the tool fail with exit code 1 on every run
// that loads pricing.
func writeMalformedGlobalPricing(t *testing.T, configBase string) {
	t.Helper()
	dir := globalDirUnder(configBase)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pricing.yaml"), []byte("models: [unclosed\n  : :\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBinary_UnexpectedFailure_WritesFailureFileInGlobalConfigFolderAndNamesItOnStderr(t *testing.T) {
	bin := buildBinary(t)
	configBase := t.TempDir()
	writeMalformedGlobalPricing(t, configBase)
	logs := filepath.Join(t.TempDir(), "logs")
	copyDir(t, filepath.Join("..", "..", "testdata", "ingestotel", "runs"), logs)

	code, stdout, stderr := runBinaryIn(t, bin, "", configBase, "report", "--path", logs)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %q)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
	files := binaryFailureFiles(t, configBase)
	if len(files) != 1 {
		t.Fatalf("failure files = %v, want exactly one in the global config folder", files)
	}
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, failureLinePrefix) {
		t.Fatalf("last stderr line = %q, want it to start with %q (stderr: %q)", last, failureLinePrefix, stderr)
	}
	if got := strings.TrimPrefix(last, failureLinePrefix); filepath.Clean(got) != filepath.Clean(files[0]) {
		t.Errorf("stderr names %q, want the written file %q", got, files[0])
	}
	if !strings.Contains(stderr, "pricing") || strings.Count(stderr, failureLinePrefix) != 1 {
		t.Errorf("stderr = %q, want the existing error text plus exactly one path line", stderr)
	}
	content := readText(t, files[0])
	for _, want := range []string{"Tool: mosaic-log-analyzer " + ToolVersion, "Kind: failure", "Exit code: 1", "pricing"} {
		if !strings.Contains(content, want) {
			t.Errorf("failure file = %q, want it to contain %q", content, want)
		}
	}
}

func TestBinary_Success_WritesNoFailureFile(t *testing.T) {
	bin := buildBinary(t)
	configBase := t.TempDir()
	logs := filepath.Join(t.TempDir(), "logs")
	copyDir(t, filepath.Join("..", "..", "testdata", "ingestotel", "runs"), logs)

	code, _, stderr := runBinaryIn(t, bin, "", configBase, "report", "--path", logs)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr)
	}
	if files := binaryFailureFiles(t, configBase); len(files) != 0 {
		t.Errorf("failure files = %v, want none after success", files)
	}
	if strings.Contains(stderr, failureLinePrefix) {
		t.Errorf("stderr = %q, want no failure-file line", stderr)
	}
}

// ingestFixture copies the ingest fixture (a logs tree with one run, and an export)
// into a temp folder and returns the logs root, the run id and the export path.
func ingestFixture(t *testing.T) (logs, runID, export string) {
	t.Helper()
	tmp := t.TempDir()
	logs = filepath.Join(tmp, "logs")
	copyDir(t, filepath.Join("..", "..", "testdata", "ingestotel", "runs"), logs)
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ingestotel", "export.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	export = filepath.Join(tmp, "otel.jsonl")
	if err := os.WriteFile(export, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return logs, "20261031T100000Z-f001", export
}

func TestBinary_DocumentedOutcomeCodes_WriteNoFailureFile(t *testing.T) {
	bin := buildBinary(t)
	cases := []struct {
		name string
		want int
		args func(t *testing.T) []string
	}{
		{"unknown command is a usage error", 3, func(*testing.T) []string { return []string{"no-such-command"} }},
		{"source that does not exist", 3, func(t *testing.T) []string {
			return []string{"report", "--path", filepath.Join(t.TempDir(), "missing")}
		}},
		{"path that is not a logs tree", 4, func(t *testing.T) []string {
			return []string{"report", "--path", t.TempDir()}
		}},
		{"ingest of a run that is not in the source", 2, func(t *testing.T) []string {
			logs, _, export := ingestFixture(t)
			return []string{"ingest-otel", "--path", logs, "--run", "20990101T000000Z-zzzz", export}
		}},
		{"ingest skipped because another ingest holds the lock", 5, func(t *testing.T) []string {
			logs, runID, export := ingestFixture(t)
			lock := `{"holder": "python/4242", "acquired_at": "` + time.Now().UTC().Format(time.RFC3339) + `"}`
			lockPath := filepath.Join(logs, runID, ".otel-ingest", "lock")
			if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lockPath, []byte(lock), 0o644); err != nil {
				t.Fatal(err)
			}
			return []string{"ingest-otel", "--path", logs, "--run", runID, export}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configBase := t.TempDir()

			code, _, stderr := runBinaryIn(t, bin, "", configBase, tc.args(t)...)

			if code != tc.want {
				t.Fatalf("exit code = %d, want %d (stderr: %q)", code, tc.want, stderr)
			}
			if files := binaryFailureFiles(t, configBase); len(files) != 0 {
				t.Errorf("failure files = %v, want none for a documented outcome", files)
			}
			if strings.Contains(stderr, failureLinePrefix) {
				t.Errorf("stderr = %q, want no failure-file line", stderr)
			}
		})
	}
}
