package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"mosaic-run/internal/cli"
)

func TestToolVersion_IsBumpedForVersionFlag(t *testing.T) {
	if ToolVersion != "1.11.1" {
		t.Fatalf("ToolVersion = %q, want 1.11.1", ToolVersion)
	}
}

// TestVersionRequested: only the first case is evidence for new behavior; the
// false cases are regression coverage that keeps every other invocation out of
// the version path (the combination outcome is asserted by the dispatch and
// binary tests).
func TestVersionRequested(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"lone version flag", []string{"--version"}, true},
		{"no arguments", nil, false},
		{"run subcommand", []string{"run", "--harness", "fake"}, false},
		{"tui flag only", []string{"--tui"}, false},
		{"dev test routing", []string{"--dev", "test"}, false},
		{"version before subcommand", []string{"--version", "run"}, false},
		{"version after subcommand", []string{"run", "--version"}, false},
		{"version with tui", []string{"--tui", "--version"}, false},
		{"version with dev", []string{"--dev", "--version"}, false},
		{"version with value", []string{"--version=1"}, false},
		{"version as value of value-bearing flag", []string{"run", "--harness", "--version"}, false},
		{"version twice", []string{"--version", "--version"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionRequested(tc.args); got != tc.want {
				t.Fatalf("versionRequested(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestRunVersion_WritesExactlyOneLineAndExitsZero(t *testing.T) {
	var buf strings.Builder

	code := runVersion(&buf)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "mosaic-run " + ToolVersion + "\n"
	if buf.String() != want {
		t.Fatalf("output = %q, want %q", buf.String(), want)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunVersion_WriteFailureReturnsFailureExit(t *testing.T) {
	if code := runVersion(failingWriter{}); code != cli.ExitFailure {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitFailure)
	}
}

// A combined --version is a usage error: exit 2, no version line on stdout, a
// message on stderr, and no files (no logger, run identity or run folder).
func TestBinary_VersionCombinedWithOtherArgs_IsUsageErrorWithoutSideEffects(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"version before subcommand", []string{"--version", "run"}},
		{"version after subcommand", []string{"run", "--version"}},
		{"version with dev", []string{"--dev", "--version"}},
		{"version with value", []string{"--version=1"}},
		{"version twice", []string{"--version", "--version"}},
		{"version as value of value-bearing flag", []string{"run", "--harness", "--version"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()

			res := runRunnerBinary(t, workDir, tc.args...)

			if res.ExitCode != cli.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr: %s", res.ExitCode, cli.ExitUsage, res.Stderr)
			}
			if res.Stdout != "" {
				t.Fatalf("stdout = %q, want empty", res.Stdout)
			}
			if strings.TrimSpace(res.Stderr) == "" {
				t.Fatal("stderr is empty, want a usage message")
			}
			entries, err := os.ReadDir(workDir)
			if err != nil {
				t.Fatalf("read work dir: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("work dir has %d new entries, want none", len(entries))
			}
		})
	}
}

func TestBinary_LoneVersion_PrintsVersionLineWithoutSideEffects(t *testing.T) {
	workDir := t.TempDir()

	res := runRunnerBinary(t, workDir, "--version")

	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", res.ExitCode, res.Stderr)
	}
	if want := "mosaic-run " + ToolVersion + "\n"; res.Stdout != want {
		t.Fatalf("stdout = %q, want %q", res.Stdout, want)
	}
	if res.Stderr != "" {
		t.Fatalf("stderr = %q, want empty", res.Stderr)
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatalf("read work dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("work dir has %d new entries (log or run folder created), want none", len(entries))
	}
}
