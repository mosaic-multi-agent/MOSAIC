package main

// The --version flag. A lone --version prints one line, "mosaic-log-analyzer
// <ToolVersion>", and exits 0 without doing anything else: no TUI, no config
// files, no workspace access. Any combination with other arguments is not a
// version request; it follows the normal command handling (a usage error where
// the command does not know the flag), and never prints the version line.

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func wantVersionLine() string { return "mosaic-log-analyzer " + ToolVersion + "\n" }

func TestVersionRequested_LoneVersionFlagOnly(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"lone --version", []string{"--version"}, true},
		{"no args", nil, false},
		{"empty args", []string{}, false},
		{"subcommand only", []string{"report"}, false},
		{"subcommand then --version", []string{"report", "--version"}, false},
		{"--version then subcommand", []string{"--version", "report"}, false},
		{"--version with --tui", []string{"--version", "--tui"}, false},
		{"--version with a value flag", []string{"--version", "--path", "x"}, false},
		{"--version as value of --path", []string{"--path", "--version"}, false},
		{"--version with equals value", []string{"--version=1"}, false},
		{"other flags only", []string{"--tui", "--path", "x"}, false},
		{"short look-alike", []string{"-version"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionRequested(tc.args); got != tc.want {
				t.Errorf("versionRequested(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestRunVersion_WritesTheVersionLineAndReturnsZero(t *testing.T) {
	var out bytes.Buffer

	code := runVersion(&out)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := out.String(); got != wantVersionLine() {
		t.Errorf("output = %q, want %q", got, wantVersionLine())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestRunVersion_WriteFailure_ReturnsFailureExitCode(t *testing.T) {
	if code := runVersion(failingWriter{}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func dirEntryCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestBinary_LoneVersion_PrintsOneLineAndTouchesNothing(t *testing.T) {
	bin := buildBinary(t)
	base, work := t.TempDir(), t.TempDir()

	code, stdout, stderr := runBinaryIn(t, bin, work, base, "--version")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr)
	}
	if stdout != wantVersionLine() {
		t.Errorf("stdout = %q, want %q", stdout, wantVersionLine())
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if n := dirEntryCount(t, base); n != 0 {
		t.Errorf("the user config folder must stay empty, has %d entries", n)
	}
	if n := dirEntryCount(t, work); n != 0 {
		t.Errorf("the working folder must stay empty, has %d entries", n)
	}
}

func TestBinary_VersionWithSubcommand_IsNotAVersionRequest(t *testing.T) {
	bin := buildBinary(t)
	base, work := t.TempDir(), t.TempDir()

	code, stdout, stderr := runBinaryIn(t, bin, work, base, "report", "--version")

	if code == 0 {
		t.Errorf("exit code = 0, want a usage failure (stdout: %q)", stdout)
	}
	if strings.Contains(stdout, wantVersionLine()) || strings.Contains(stderr, wantVersionLine()) {
		t.Errorf("a combined invocation must not print the version line (stdout %q, stderr %q)", stdout, stderr)
	}
}
