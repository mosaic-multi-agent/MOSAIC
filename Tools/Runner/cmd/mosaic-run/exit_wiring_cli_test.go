package main

// exit_wiring_cli_test.go drives the CLI and version paths of the exit wiring through injected
// seams: a failure is reported with a path line, nothing ever pauses, and the version paths
// report nothing.

import (
	"strings"
	"testing"
)

func TestCLIMode_WorkingDirectoryFailure_WritesFileNamesItAndNeverPauses(t *testing.T) {
	cases := map[string][]string{
		"run":      {"run"},
		"dev test": {"--dev", "test"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			run := runWiring(t, wiringSetup{GOOS: "windows", Args: args, Getwd: failingGetwd, RunTUI: noopTUI})

			content := requireReported(t, run, "error: getting working directory", false)

			if !strings.Contains(content, "Kind: failure") || !strings.Contains(content, "cwd gone") {
				t.Errorf("failure file lacks the kind or the message:\n%s", content)
			}
		})
	}
}

func TestCLIMode_PositionalArgumentOnATerminal_DoesNotPause(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"run"}, IsTerminal: true, Getwd: failingGetwd, RunTUI: noopTUI})

	requireReported(t, run, "error: getting working directory", false)
}

func TestCLIMode_PanicOutsideTheTUI_ReportsWritesFileAndNeverPauses(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"run"}, Getwd: panickingGetwd, RunTUI: noopTUI})

	content := requireReported(t, run, "panic: getwd exploded", false)

	if !strings.Contains(run.Stderr, "goroutine") {
		t.Errorf("stderr lacks the stack:\n%s", run.Stderr)
	}
	if !strings.Contains(content, "Kind: panic") || !strings.Contains(content, "Stack:") {
		t.Errorf("failure file lacks the panic kind or stack:\n%s", content)
	}
	if got := strings.Count(run.Stderr, "panic: getwd exploded"); got != 1 {
		t.Errorf("panic reported %d times, want once\n%s", got, run.Stderr)
	}
}

func TestVersion_AloneOnATerminal_PrintsVersionAndExitsZeroWithoutReport(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--version"}, IsTerminal: true, Getwd: failingGetwd, RunTUI: noopTUI})

	requireSingleExit(t, run, 0)
	if !strings.Contains(run.Stdout, ToolVersion) {
		t.Errorf("stdout %q lacks the version %s", run.Stdout, ToolVersion)
	}
	requireNothingReported(t, run)
}

func TestVersion_CombinedWithTUIFlag_ExitsThreeWithoutReportOrPause(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--version", "--tui"}, IsTerminal: true, Getwd: failingGetwd, RunTUI: noopTUI})

	requireSingleExit(t, run, 3)
	requireNothingReported(t, run)
}
