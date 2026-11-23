package main

// exit_binary_test.go runs a private copy of the built mosaic-run binary to verify fatal exits
// in CLI mode: an unexpected failure (exit code 1) writes a failure file beside the executable
// and adds one stderr line naming it, while success and documented outcome codes write none.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// copiedRunnerRun is the result of one run of a private binary copy.
type copiedRunnerRun struct {
	binaryResult
	BinaryDir string // folder holding the private copy; failure files land here
}

// runCopiedRunnerBinary copies the shared test binary into a fresh folder and runs the copy in
// workDir with extraEnv added to the environment.
func runCopiedRunnerBinary(t *testing.T, workDir string, extraEnv []string, args ...string) copiedRunnerRun {
	t.Helper()
	src := runnerBinary(t)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read built binary: %v", err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatalf("copy binary: %v", err)
	}
	cmd := exec.Command(dst, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	res := binaryResult{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		t.Fatalf("run mosaic-run %v: %v", args, runErr)
	}
	return copiedRunnerRun{binaryResult: res, BinaryDir: dir}
}

// runnerFailureFiles lists the failure files in dir.
func runnerFailureFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "mosaic-run-failure-*.txt"))
	if err != nil {
		t.Fatalf("glob failure files: %v", err)
	}
	return files
}

// splitFailurePathLine splits stderr into the text before the failure-file path line and the
// path it names; ok is false when the last non-empty line is not a path line.
func splitFailurePathLine(stderr string) (before, path string, ok bool) {
	trimmed := strings.TrimRight(stderr, "\r\n")
	idx := strings.LastIndex(trimmed, "\n")
	last := strings.TrimRight(trimmed[idx+1:], "\r")
	if !strings.HasPrefix(last, failurePathLinePrefix) {
		return stderr, "", false
	}
	return trimmed[:idx+1], strings.TrimPrefix(last, failurePathLinePrefix), true
}

// newMissingHarnessCatalog creates a minimal test catalog and returns its root. With it, the
// --dev test command reaches harness resolution, which fails with exit code 1 when the
// harness binary is not on PATH.
func newMissingHarnessCatalog(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	wfDir := filepath.Join(root, "Tools", "Runner", "TestCatalog", "Workflows", "MosaicTest")
	if err := os.MkdirAll(filepath.Join(wfDir, "Fixtures", "smoke-single"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: smoke-single\nname: Test Workflow\nmodes:\n  - auto\n---\n"
	if err := os.WriteFile(filepath.Join(wfDir, "smoke-single.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// runExitOneFailure runs the binary into the failure that ends with exit code 1: the harness
// binary cannot be found on an empty PATH.
func runExitOneFailure(t *testing.T) copiedRunnerRun {
	t.Helper()
	catalog := newMissingHarnessCatalog(t)
	return runCopiedRunnerBinary(t, t.TempDir(), []string{"PATH=" + t.TempDir(), "Path=" + t.TempDir()},
		"--dev", "test", "--catalog", catalog, "--workflow", "smoke-single", "--harness", "claude-code")
}

func TestBinary_ExitOneFailure_KeepsExitCodeAndWritesAFailureFile(t *testing.T) {
	run := runExitOneFailure(t)

	if run.ExitCode != 1 {
		t.Fatalf("exit code %d, want 1\nstderr: %s", run.ExitCode, run.Stderr)
	}
	files := runnerFailureFiles(t, run.BinaryDir)
	if len(files) != 1 {
		t.Fatalf("failure files beside the executable = %v, want exactly one\nstderr: %s", files, run.Stderr)
	}
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read failure file: %v", err)
	}
	for _, want := range []string{"Tool: mosaic-run " + ToolVersion, "Kind: failure", "Exit code: 1", "resolving harness binaries"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("failure file lacks %q:\n%s", want, content)
		}
	}
}

func TestBinary_ExitOneFailure_StderrIsUnchangedExceptOneLineNamingTheFile(t *testing.T) {
	run := runExitOneFailure(t)

	before, path, ok := splitFailurePathLine(run.Stderr)
	if !ok {
		t.Fatalf("stderr does not end with a %q line:\n%s", failurePathLinePrefix, run.Stderr)
	}
	if !strings.HasPrefix(before, "error: resolving harness binaries: ") {
		t.Errorf("stderr before the path line = %q, want it to start with the existing error line", before)
	}
	if strings.Contains(before, failurePathLinePrefix) {
		t.Errorf("the path line appears more than once:\n%s", run.Stderr)
	}
	files := runnerFailureFiles(t, run.BinaryDir)
	if len(files) != 1 || filepath.Clean(path) != filepath.Clean(files[0]) {
		t.Errorf("path line names %q, want the written file %v", path, files)
	}
}

func TestBinary_NoUnexpectedFailure_WritesNoFileAndAddsNoLine(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{"success (version)", []string{"--version"}, 0},
		{"usage error", []string{"--dev", "test", "--catalog", "somewhere"}, 3},
		{"run identity refused", []string{"run", "--harness", "fake", "--run", "20260101T000000Z-abcd", "--mode", "auto", "--workflow", "w", "--task", "t", "--review-loop-limit", "none"}, 2},
		{"refused before any invocation", []string{"run", "--new-run", "--mode", "auto", "--review-loop-limit", "3", "--workflow", "wf", "--task", "x", "--harness", "fake"}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runCopiedRunnerBinary(t, t.TempDir(), nil, tc.args...)

			if run.ExitCode != tc.wantExit {
				t.Fatalf("exit code %d, want %d\nstderr: %s", run.ExitCode, tc.wantExit, run.Stderr)
			}
			if files := runnerFailureFiles(t, run.BinaryDir); len(files) != 0 {
				t.Errorf("failure files written: %v", files)
			}
			if strings.Contains(run.Stderr, failurePathLinePrefix) {
				t.Errorf("stderr names a failure file:\n%s", run.Stderr)
			}
		})
	}
}
