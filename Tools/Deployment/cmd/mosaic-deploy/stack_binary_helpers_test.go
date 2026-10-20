package main

// stack_binary_helpers_test.go builds the mosaic-deploy binary once per test run and runs it
// as a subprocess, so tests can observe real exit codes, stderr and written files. Nothing in
// this file is a test itself.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  error
	binaryDir  string
)

// TestMain removes the shared binary directory after the package's tests ran.
func TestMain(m *testing.M) {
	code := m.Run()
	if binaryDir != "" {
		_ = os.RemoveAll(binaryDir)
	}
	os.Exit(code)
}

// deployBinary returns the path of a freshly built mosaic-deploy binary.
func deployBinary(t *testing.T) string {
	t.Helper()
	binaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mosaic-deploy-bin-")
		if err != nil {
			binaryErr = err
			return
		}
		binaryDir = dir
		name := "mosaic-deploy"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		binaryPath = filepath.Join(dir, name)
		out, err := exec.Command("go", "build", "-o", binaryPath, ".").CombinedOutput()
		if err != nil {
			binaryErr = errors.New("go build failed: " + err.Error() + "\n" + string(out))
		}
	})
	if binaryErr != nil {
		t.Fatalf("build mosaic-deploy: %v", binaryErr)
	}
	return binaryPath
}

// binaryRun is the observable result of one subprocess run.
type binaryRun struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// runDeployBinary runs the binary with args and returns its output and exit code.
func runDeployBinary(t *testing.T, args ...string) binaryRun {
	t.Helper()
	cmd := exec.Command(deployBinary(t), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := binaryRun{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		t.Fatalf("run mosaic-deploy %v: %v", args, err)
	}
	return res
}

// runSummaryDoc is the part of the --output json run summary the stack tests read.
type runSummaryDoc struct {
	Outcome string
	Actions []struct {
		Ref struct {
			Kind string
			Key  string
		}
		SourceVersion string
	}
}

// parseRunSummary decodes the JSON run summary on stdout.
func parseRunSummary(t *testing.T, run binaryRun) runSummaryDoc {
	t.Helper()
	var doc runSummaryDoc
	if err := json.Unmarshal([]byte(run.Stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\nstdout: %s\nstderr: %s", err, run.Stdout, run.Stderr)
	}
	return doc
}

// deployedVersions maps each action's agent key to its source version.
func (d runSummaryDoc) deployedVersions() map[string]string {
	out := map[string]string{}
	for _, a := range d.Actions {
		if a.Ref.Kind == "agent" {
			out[a.Ref.Key] = a.SourceVersion
		}
	}
	return out
}

// dirEntryCount returns the number of entries under dir (recursively, files and folders).
func dirEntryCount(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(_ string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return n - 1 // exclude dir itself
}
