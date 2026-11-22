package main

// version_binary_test.go specifies "mosaic-deploy --version": one stdout line, exit 0, usable
// from any directory (no MOSAIC root needed), with no side effects.
//
// Known limitation: the fresh-console "Press Enter to exit" non-prompt cannot be observed
// through pipes and is not covered here.

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
)

func wantVersionLine() string {
	return "mosaic-deploy " + ToolVersion + "\n"
}

func TestVersionFlag_PrintsOneLineAndExitsZeroOutsideAnyMosaicRoot(t *testing.T) {
	dir := t.TempDir()

	run := runDeployBinaryIn(t, dir, "--version")

	if run.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", run.ExitCode, run.Stderr)
	}
	if run.Stdout != wantVersionLine() {
		t.Errorf("stdout = %q, want %q", run.Stdout, wantVersionLine())
	}
	if run.Stderr != "" {
		t.Errorf("stderr = %q, want empty", run.Stderr)
	}
}

func TestVersionFlag_WritesNoFilesToWorkingDirectory(t *testing.T) {
	dir := t.TempDir()

	run := runDeployBinaryIn(t, dir, "--version")

	if run.Stdout != wantVersionLine() {
		t.Fatalf("stdout = %q, want %q", run.Stdout, wantVersionLine())
	}
	if n := dirEntryCount(t, dir); n != 0 {
		entries, _ := os.ReadDir(dir)
		t.Errorf("working directory gained %d entries after --version: %v", n, entries)
	}
}

// Runs the binary inside a real (fixture) MOSAIC root, so root resolution succeeds.
func TestVersionFlag_WorksInsideAMosaicRootToo(t *testing.T) {
	root := newStackMosaicRoot(t)

	run := runDeployBinaryIn(t, root, "--version")

	if run.ExitCode != 0 || run.Stdout != wantVersionLine() {
		t.Errorf("exit=%d stdout=%q, want exit 0 and %q; stderr: %s",
			run.ExitCode, run.Stdout, wantVersionLine(), run.Stderr)
	}
}

// Chosen convention: --version is honoured only when it is the sole argument. Combined with
// anything else it is an ordinary unknown flag: a usage error, no stdout. Each case runs inside a
// valid fixture root, so the failure cannot come from root resolution.
func TestVersionFlag_CombinedWithOtherArgumentsIsAUsageError(t *testing.T) {
	cases := []struct {
		name string
		args func(root string) []string
	}{
		{"after subcommand", func(string) []string { return []string{"deploy", "--version"} }},
		{"before subcommand", func(string) []string { return []string{"--version", "deploy"} }},
		{"with global flag", func(root string) []string { return []string{"--mosaic-root", root, "--version"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newStackMosaicRoot(t)

			run := runDeployBinaryIn(t, root, tc.args(root)...)

			if run.ExitCode != cli.ExitUsage {
				t.Errorf("exit code = %d, want %d (usage); stderr: %s", run.ExitCode, cli.ExitUsage, run.Stderr)
			}
			if run.Stdout != "" {
				t.Errorf("stdout = %q, want empty", run.Stdout)
			}
			if !strings.Contains(run.Stderr, "--version") {
				t.Errorf("stderr does not mention --version: %q", run.Stderr)
			}
		})
	}
}

// Regression guard for the zero-argument path next to the new check: with no terminal attached,
// the usage line is printed and the exit code is the usage code.
func TestNoArguments_NonTerminal_StillPrintsUsageAndExitsUsage(t *testing.T) {
	run := runDeployBinaryIn(t, t.TempDir())

	if run.ExitCode != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d; stderr: %s", run.ExitCode, cli.ExitUsage, run.Stderr)
	}
	if !strings.Contains(run.Stderr, "usage") {
		t.Errorf("stderr = %q, want a usage line", run.Stderr)
	}
}

func TestVersionRequested_OnlyForLoneFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"lone flag", []string{"--version"}, true},
		{"no args", nil, false},
		{"subcommand", []string{"deploy"}, false},
		{"flag after subcommand", []string{"deploy", "--version"}, false},
		{"flag before subcommand", []string{"--version", "deploy"}, false},
		{"with value form", []string{"--version=1"}, false},
		{"other flag only", []string{"--help"}, false},
		{"flag as value of global flag", []string{"--mosaic-root", "--version"}, false},
		{"flag twice", []string{"--version", "--version"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionRequested(tc.args); got != tc.want {
				t.Errorf("versionRequested(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestRunVersion_WritesExactlyTheVersionLine(t *testing.T) {
	var buf bytes.Buffer

	code := runVersion(&buf)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if buf.String() != wantVersionLine() {
		t.Errorf("output = %q, want %q", buf.String(), wantVersionLine())
	}
}

// failingWriter records that a write was attempted and always fails.
type failingWriter struct{ attempts int }

func (w *failingWriter) Write([]byte) (int, error) {
	w.attempts++
	return 0, errors.New("write failed")
}

func TestRunVersion_WriteFailureReturnsExitFailure(t *testing.T) {
	w := &failingWriter{}

	code := runVersion(w)

	if w.attempts == 0 {
		t.Errorf("runVersion never attempted to write the version line")
	}
	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want %d on write failure", code, cli.ExitFailure)
	}
}
