package main

// version_binary_helpers_test.go builds the mosaic-run binary once per test run and runs it
// as a subprocess, so tests can observe real exit codes, streams and written files. Nothing
// in this file is a test itself.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	runBinaryOnce sync.Once
	runBinaryPath string
	runBinaryErr  error
	runBinaryDir  string
)

// TestMain removes the shared binary directory after the package's tests ran.
func TestMain(m *testing.M) {
	code := m.Run()
	if runBinaryDir != "" {
		_ = os.RemoveAll(runBinaryDir)
	}
	os.Exit(code)
}

// runnerBinary returns the path of a freshly built mosaic-run binary.
func runnerBinary(t *testing.T) string {
	t.Helper()
	runBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mosaic-run-bin-")
		if err != nil {
			runBinaryErr = err
			return
		}
		runBinaryDir = dir
		name := "mosaic-run"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		runBinaryPath = filepath.Join(dir, name)
		out, err := exec.Command("go", "build", "-o", runBinaryPath, ".").CombinedOutput()
		if err != nil {
			runBinaryErr = errors.New("go build failed: " + err.Error() + "\n" + string(out))
		}
	})
	if runBinaryErr != nil {
		t.Fatalf("build mosaic-run: %v", runBinaryErr)
	}
	return runBinaryPath
}

// binaryResult is the observable result of one subprocess run.
type binaryResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// runRunnerBinary runs the binary in workDir with args (stdin is empty, stdout and stderr
// are pipes) and returns its output and exit code.
func runRunnerBinary(t *testing.T, workDir string, args ...string) binaryResult {
	t.Helper()
	cmd := exec.Command(runnerBinary(t), args...)
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := binaryResult{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		t.Fatalf("run mosaic-run %v: %v", args, err)
	}
	return res
}
