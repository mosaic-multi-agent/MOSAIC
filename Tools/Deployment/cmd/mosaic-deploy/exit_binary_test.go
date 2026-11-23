package main

// exit_binary_test.go runs the built binary to verify fatal exits in CLI mode: an unexpected
// failure writes a failure file and adds one stderr line naming it, while success, documented
// outcome codes and the JSON contract stay as they were.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinary_StartupFailure_KeepsExitCodeOneAndWritesAFailureFile(t *testing.T) {
	root := newDuplicateIDMosaicRoot(t)

	run := runCopiedDeployBinary(t, renderJSONArgs(root, t.TempDir())...)

	if run.ExitCode != 1 {
		t.Fatalf("exit code %d, want 1\nstderr: %s", run.ExitCode, run.Stderr)
	}
	files := failureFiles(t, run.BinaryDir)
	if len(files) != 1 {
		t.Fatalf("failure files beside the executable = %v, want exactly one\nstderr: %s", files, run.Stderr)
	}
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read failure file: %v", err)
	}
	for _, want := range []string{"Tool: mosaic-deploy", "Kind: failure", "Exit code: 1", "load catalog"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("failure file lacks %q:\n%s", want, content)
		}
	}
}

func TestBinary_StartupFailure_StderrIsUnchangedExceptOneLineNamingTheFile(t *testing.T) {
	root := newDuplicateIDMosaicRoot(t)

	run := runCopiedDeployBinary(t, renderJSONArgs(root, t.TempDir())...)

	before, path, ok := splitFailurePathLine(run.Stderr)
	if !ok {
		t.Fatalf("stderr does not end with a %q line:\n%s", failurePathLinePrefix, run.Stderr)
	}
	if !strings.HasPrefix(before, "error: load catalog: ") {
		t.Errorf("stderr before the path line = %q, want it to start with the existing error line", before)
	}
	if strings.Count(strings.TrimRight(before, "\r\n"), "\n") != 0 {
		t.Errorf("stderr before the path line has more than the one error line: %q", before)
	}
	files := failureFiles(t, run.BinaryDir)
	if len(files) != 1 || filepath.Clean(path) != filepath.Clean(files[0]) {
		t.Errorf("path line names %q, want the written file %v", path, files)
	}
}

func TestBinary_StartupFailure_StdoutStaysEmpty(t *testing.T) {
	root := newDuplicateIDMosaicRoot(t)

	run := runCopiedDeployBinary(t, renderJSONArgs(root, t.TempDir())...)

	if run.ExitCode != 1 {
		t.Fatalf("exit code %d, want 1", run.ExitCode)
	}
	if run.Stdout != "" {
		t.Errorf("stdout = %q, want empty on a startup failure", run.Stdout)
	}
}

func TestBinary_RenderJSON_Success_WritesNoFileAndKeepsEnvelope(t *testing.T) {
	root := newStackMosaicRoot(t)

	run := runCopiedDeployBinary(t, renderJSONArgs(root, t.TempDir())...)

	if run.ExitCode != 0 {
		t.Fatalf("exit code %d, want 0\nstderr: %s", run.ExitCode, run.Stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(run.Stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, run.Stdout)
	}
	if files := failureFiles(t, run.BinaryDir); len(files) != 0 {
		t.Errorf("success wrote failure files: %v", files)
	}
	if strings.Contains(run.Stderr, failurePathLinePrefix) {
		t.Errorf("success stderr names a failure file:\n%s", run.Stderr)
	}
}

func TestBinary_UsageOutcome_ExitsThreeWithoutAFailureFile(t *testing.T) {
	cases := map[string][]string{
		"no arguments and no terminal": nil,
		"unknown flag":                 {"render", "--no-such-flag", "--mosaic-root", newStackMosaicRoot(t)},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			run := runCopiedDeployBinary(t, args...)

			if run.ExitCode != 3 {
				t.Fatalf("exit code %d, want 3\nstderr: %s", run.ExitCode, run.Stderr)
			}
			if files := failureFiles(t, run.BinaryDir); len(files) != 0 {
				t.Errorf("usage outcome wrote failure files: %v", files)
			}
			if strings.Contains(run.Stderr, failurePathLinePrefix) {
				t.Errorf("usage outcome stderr names a failure file:\n%s", run.Stderr)
			}
		})
	}
}

func TestBinary_ParallelStartupFailures_WriteSeparateFilesIntoOneFolder(t *testing.T) {
	root := newDuplicateIDMosaicRoot(t)
	const runs = 3
	first := runCopiedDeployBinary(t, renderJSONArgs(root, t.TempDir())...)
	dir := first.BinaryDir
	exe := filepath.Join(dir, filepath.Base(deployBinary(t)))
	results := make(chan binaryRun, runs)
	for i := 0; i < runs; i++ {
		go func() { results <- runBinaryAt(t, exe, renderJSONArgs(root, t.TempDir())...) }()
	}
	for i := 0; i < runs; i++ {
		if r := <-results; r.ExitCode != 1 {
			t.Errorf("parallel run exit code %d, want 1\nstderr: %s", r.ExitCode, r.Stderr)
		}
	}

	if files := failureFiles(t, dir); len(files) != runs+1 {
		t.Errorf("failure files = %d, want %d distinct files", len(files), runs+1)
	}
}

func TestBinary_CommandRuntimeFailure_KeepsExitCodeAndJSONAndAddsOnlyThePathLine(t *testing.T) {
	root := newStackMosaicRoot(t)

	run := runCopiedDeployBinary(t, renderMissingAgentArgs(root, t.TempDir())...)

	if run.ExitCode != 1 {
		t.Fatalf("exit code %d, want 1\nstderr: %s", run.ExitCode, run.Stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(run.Stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, run.Stdout)
	}
	if doc["error"] != true || doc["exitCode"] != float64(1) {
		t.Errorf("envelope = %v, want error true and exitCode 1", doc)
	}
	before, path, ok := splitFailurePathLine(run.Stderr)
	if !ok {
		t.Fatalf("stderr does not end with a %q line:\n%s", failurePathLinePrefix, run.Stderr)
	}
	if !strings.HasPrefix(before, "error: ") || strings.Count(strings.TrimRight(before, "\r\n"), "\n") != 0 {
		t.Errorf("stderr before the path line = %q, want the one existing error line", before)
	}
	files := failureFiles(t, run.BinaryDir)
	if len(files) != 1 || filepath.Clean(path) != filepath.Clean(files[0]) {
		t.Errorf("path line names %q, want the written file %v", path, files)
	}
}
