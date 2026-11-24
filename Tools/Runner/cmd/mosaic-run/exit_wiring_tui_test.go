package main

// exit_wiring_tui_test.go drives the TUI-mode exit wiring through injected seams: which ends of
// the process report a failure, write a failure file and pause on a Windows TUI start.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/session"
	"mosaic-run/internal/tui"
)

func failingTUI(msg string) func(context.Context, session.Session, tui.Options) error {
	return func(context.Context, session.Session, tui.Options) error { return errors.New(msg) }
}

func TestTUIMode_WorkingDirectoryFailure_ReportsWritesFileAndPausesOnWindows(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--tui"}, Getwd: failingGetwd, RunTUI: noopTUI})

	content := requireReported(t, run, "getting working directory", true)

	if !strings.Contains(content, "Kind: failure") || !strings.Contains(content, "cwd gone") {
		t.Errorf("failure file lacks the kind or the message:\n%s", content)
	}
}

func TestTUIMode_TUIError_ReportsWritesFileAndPausesOnWindows(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--tui"}, Getwd: getwdOf(t.TempDir()), RunTUI: failingTUI("terminal unusable")})

	content := requireReported(t, run, "tui: terminal unusable", true)

	if !strings.Contains(content, "Kind: failure") || !strings.Contains(content, "Exit code: 1") {
		t.Errorf("failure file lacks the kind or exit code:\n%s", content)
	}
}

func TestTUIMode_TUIError_OnOtherPlatformsReportsWithoutPause(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "linux", Args: []string{"--tui"}, Getwd: getwdOf(t.TempDir()), RunTUI: failingTUI("terminal unusable")})

	requireReported(t, run, "tui: terminal unusable", false)
}

func TestTUIMode_StartedByTerminalDetection_PausesOnWindows(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", IsTerminal: true, Getwd: failingGetwd, RunTUI: noopTUI})

	requireReported(t, run, "getting working directory", true)
}

func TestTUIMode_NormalEnd_ExitsZeroWithoutReportOrPause(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--tui"}, Getwd: getwdOf(t.TempDir()), RunTUI: noopTUI})

	requireSingleExit(t, run, 0)
	requireNothingReported(t, run)
}

func TestTUIMode_RunIdentityUsageError_ExitsTwoWithoutFileOrPause(t *testing.T) {
	args := []string{"--tui", "--run", "20260101T000000Z-abcd", "--new-run"}
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: args, Getwd: getwdOf(t.TempDir()), RunTUI: noopTUI})

	requireSingleExit(t, run, 2)
	if !strings.Contains(run.Stderr, "error: ") {
		t.Errorf("stderr lacks the usage error line:\n%s", run.Stderr)
	}
	requireNothingReported(t, run)
}

func TestTUIMode_ExternalInterrupt_KeepsTodaysStderrLineAndExitOneWithoutReport(t *testing.T) {
	interrupted := fmt.Errorf("program was killed: %w", tea.ErrInterrupted)
	runTUI := func(context.Context, session.Session, tui.Options) error { return interrupted }
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--tui"}, Getwd: getwdOf(t.TempDir()), RunTUI: runTUI})

	requireSingleExit(t, run, 1)
	if !strings.Contains(run.Stderr, "tui: "+interrupted.Error()) {
		t.Errorf("stderr lacks today's line %q:\n%s", "tui: "+interrupted.Error(), run.Stderr)
	}
	requireNothingReported(t, run)
}

func TestTUIMode_PanicInTUI_ReportsMessageAndStackWritesFileAndPauses(t *testing.T) {
	runTUI := func(context.Context, session.Session, tui.Options) error { panic("kaboom") }
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--tui"}, Getwd: getwdOf(t.TempDir()), RunTUI: runTUI})

	content := requireReported(t, run, "panic: kaboom", true)

	if !strings.Contains(run.Stderr, "goroutine") {
		t.Errorf("stderr lacks the stack:\n%s", run.Stderr)
	}
	if !strings.Contains(content, "Kind: panic") || !strings.Contains(content, "Stack:") {
		t.Errorf("failure file lacks the panic kind or stack:\n%s", content)
	}
}

func TestTUIMode_PanicBeforeTheTUIStarts_ReportsWritesFileAndPausesOnWindows(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--tui"}, Getwd: panickingGetwd, RunTUI: noopTUI})

	content := requireReported(t, run, "panic: getwd exploded", true)

	if !strings.Contains(content, "Kind: panic") || !strings.Contains(content, "Stack:") {
		t.Errorf("failure file lacks the panic kind or stack:\n%s", content)
	}
	if got := strings.Count(run.Stderr, "panic: getwd exploded"); got != 1 {
		t.Errorf("panic reported %d times, want once\n%s", got, run.Stderr)
	}
}

func TestTUIMode_PanicInTUI_IsReportedOnce(t *testing.T) {
	runTUI := func(context.Context, session.Session, tui.Options) error { panic("kaboom") }
	run := runWiring(t, wiringSetup{GOOS: "linux", Args: []string{"--tui"}, Getwd: getwdOf(t.TempDir()), RunTUI: runTUI})

	if got := strings.Count(run.Stderr, "panic: kaboom"); got != 1 {
		t.Errorf("panic reported %d times, want once\n%s", got, run.Stderr)
	}
	requireSingleExit(t, run, 1)
}

func TestTUIMode_RunIdentityFailureThatIsNotUsage_ReportsWritesFileAndPausesOnWindows(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-workdir")
	run := runWiring(t, wiringSetup{GOOS: "windows", Args: []string{"--tui"}, Getwd: getwdOf(missing), RunTUI: noopTUI})

	content := requireReported(t, run, "scanning for runs", true)

	if !strings.Contains(content, "Kind: failure") || !strings.Contains(content, "Exit code: 1") {
		t.Errorf("failure file lacks the kind or exit code:\n%s", content)
	}
}

func TestTUIMode_FailureFolderUnavailable_StillShowsMessageWithoutFileOrSecondError(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "linux", Args: []string{"--tui"}, Getwd: getwdOf(t.TempDir()),
		RunTUI: failingTUI("terminal unusable"), FailureDirErr: errors.New("no folder")})

	requireSingleExit(t, run, 1)
	if got := strings.Count(run.Stderr, "tui: terminal unusable"); got != 1 {
		t.Errorf("message shown %d times, want once\n%s", got, run.Stderr)
	}
	if files, _ := filepath.Glob(filepath.Join(run.Dir, "*.txt")); len(files) != 0 {
		t.Errorf("failure files written: %v", files)
	}
	if strings.Contains(run.Stderr, failurePathLinePrefix) {
		t.Errorf("stderr names a failure file:\n%s", run.Stderr)
	}
}
