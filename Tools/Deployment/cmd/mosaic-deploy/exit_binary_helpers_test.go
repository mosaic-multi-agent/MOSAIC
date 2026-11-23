package main

// exit_binary_helpers_test.go runs a private copy of the built mosaic-deploy binary so the
// failure files it writes beside its executable can be inspected without interference from
// other tests. Nothing in this file is a test itself.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const failurePathLinePrefix = "failure report written to "

// copiedBinaryRun is the result of one run of a private binary copy.
type copiedBinaryRun struct {
	binaryRun
	BinaryDir string // folder holding the private copy; failure files land here
}

// runCopiedDeployBinary copies the shared test binary into a fresh folder and runs the copy.
func runCopiedDeployBinary(t *testing.T, args ...string) copiedBinaryRun {
	t.Helper()
	src := deployBinary(t)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read built binary: %v", err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatalf("copy binary: %v", err)
	}
	return copiedBinaryRun{binaryRun: runBinaryAt(t, dst, args...), BinaryDir: dir}
}

// runBinaryAt runs the executable at exe with args; safe to call from a goroutine.
func runBinaryAt(t *testing.T, exe string, args ...string) binaryRun {
	t.Helper()
	cmd := exec.Command(exe, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	res := binaryRun{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		t.Errorf("run mosaic-deploy %v: %v", args, runErr)
		res.ExitCode = -1
	}
	return res
}

// failureFiles lists the failure files in dir.
func failureFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "mosaic-deploy-failure-*.txt"))
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
	last := trimmed[idx+1:]
	last = strings.TrimRight(last, "\r")
	if !strings.HasPrefix(last, failurePathLinePrefix) {
		return stderr, "", false
	}
	return trimmed[:idx+1], strings.TrimPrefix(last, failurePathLinePrefix), true
}

// newDuplicateIDMosaicRoot returns a MOSAIC root whose default catalog has two agents that
// share one numeric id, which makes the catalogue load fail at startup.
func newDuplicateIDMosaicRoot(t *testing.T) string {
	t.Helper()
	root := newStackMosaicRoot(t)
	def := filepath.Join(root, "Catalog")
	writeStackAgent(t, def, "dup-one", "dup-001", "1.0.0")
	writeStackAgent(t, def, "dup-two", "dup-001", "1.0.0")
	return root
}

// renderJSONArgs returns the arguments of a successful render of the fixture's default agent.
func renderJSONArgs(root, workspace string) []string {
	return []string{"render", "--source-agent", "default-only-agent", "--workflows", "", "--overwrite",
		"--output", "json", "--mosaic-root", root, "--harness", "claude-code", "--workspace", workspace}
}

// renderMissingAgentArgs returns the arguments of a render whose source agent is not in the
// catalogue: the tool starts fine and the command itself fails with result code 1.
func renderMissingAgentArgs(root, workspace string) []string {
	return []string{"render", "--source-agent", "no-such-agent", "--workflows", "", "--overwrite",
		"--output", "json", "--mosaic-root", root, "--harness", "claude-code", "--workspace", workspace}
}
