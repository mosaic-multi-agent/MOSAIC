package main

// Binary-level tests for the version request: they build the real
// executable and run it, proving the composition root hooks the version
// route in before any configuration, workspace or dependency construction.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	versionBinaryOnce sync.Once
	versionBinaryPath string
	versionBinaryErr  error
)

// buildVersionBinary compiles this package once per test run into a temp dir.
func buildVersionBinary(t *testing.T) string {
	t.Helper()
	versionBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "agent-test-version-bin-")
		if err != nil {
			versionBinaryErr = err
			return
		}
		versionBinaryPath = filepath.Join(dir, "mosaic-agent-test"+exeSuffix())
		out, err := exec.Command("go", "build", "-o", versionBinaryPath, ".").CombinedOutput()
		if err != nil {
			versionBinaryErr = &buildError{err: err, output: string(out)}
		}
	})
	if versionBinaryErr != nil {
		t.Fatalf("building the binary: %v", versionBinaryErr)
	}
	return versionBinaryPath
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.output }

type versionRun struct {
	stdout, stderr string
	exitCode       int
}

// runVersionBinary runs the built binary in an empty working directory with
// hostile configuration: nonexistent MOSAIC root, logger bundle, cost tool
// and deploy tool, and the given args.
func runVersionBinary(t *testing.T, args ...string) versionRun {
	t.Helper()
	bin := buildVersionBinary(t)
	cwd := t.TempDir()
	missing := filepath.Join(cwd, "does-not-exist")

	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(),
		"MOSAIC_AGENT_TEST_MOSAIC_ROOT="+missing,
		"MOSAIC_AGENT_TEST_LOGGER_BUNDLE="+missing,
		"MOSAIC_AGENT_TEST_COST_TOOL="+missing,
		"MOSAIC_AGENT_TEST_DEPLOY_TOOL="+missing,
		"MOSAIC_AGENT_TEST_CATALOG_FOLDER="+missing,
	)
	cmd.Stdin = strings.NewReader("")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running binary: %v", err)
		}
		code = ee.ExitCode()
	}
	if entries, _ := os.ReadDir(cwd); len(entries) != 0 {
		t.Errorf("working directory gained %d entries after the run: %v", len(entries), entries)
	}
	return versionRun{stdout.String(), stderr.String(), code}
}

func assertVersionOnly(t *testing.T, run versionRun) {
	t.Helper()
	want := "mosaic-agent-test " + ToolVersion + "\n"
	if run.exitCode != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", run.exitCode, run.stderr)
	}
	if run.stdout != want {
		t.Errorf("stdout = %q, want %q", run.stdout, want)
	}
	if run.stderr != "" {
		t.Errorf("stderr = %q, want empty", run.stderr)
	}
}

func TestBinary_LoneVersion_PrintsOneLineExitsZeroWithHostileConfiguration(t *testing.T) {
	assertVersionOnly(t, runVersionBinary(t, "--version"))
}

func TestBinary_VersionWithUnknownHarness_StillPrintsVersionBeforeWiring(t *testing.T) {
	// An unknown harness makes dependency construction fail with a usage
	// error; the version request must be decided before that happens.
	assertVersionOnly(t, runVersionBinary(t, "--harness", "no-such-harness", "--version"))
}

func TestBinary_VersionAfterCommand_PrintsVersionBeforeWiring(t *testing.T) {
	assertVersionOnly(t, runVersionBinary(t, "run", "no-such-suite.yaml", "--version"))
}
