package main

// exit_wiring_test.go drives the composition root's exit wiring through injected seams: which
// ends of the process report a failure, write a failure file and pause on a Windows TUI start.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/fatal"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/tui"
)

// wiringRun is the observable result of one runTool call.
type wiringRun struct {
	Exits   []int
	Stderr  string
	Dir     string // failure folder
	StdinIn bool   // true when the pause read the input
}

// wiringSetup describes one run: the platform, the seams and the arguments.
type wiringSetup struct {
	GOOS string
	Args []string
	// FailureDir overrides the failure folder lookup; nil means a working temp folder.
	FailureDir func() (string, error)
	Getwd      func() (string, error)
	RunTUI     func(ctx context.Context, svc app.Service, opts tui.Options) error
}

// runWiring calls runTool with recording seams. A panic that escapes runTool fails the test.
func runWiring(t *testing.T, s wiringSetup) wiringRun {
	t.Helper()
	fatal.Reset()
	t.Cleanup(fatal.Reset)
	dir := t.TempDir()
	var stderr bytes.Buffer
	stdin := bytes.NewReader([]byte("\n"))
	var exits []int
	base := fatal.Config{
		Args:       append([]string{"mosaic-deploy"}, s.Args...),
		GOOS:       s.GOOS,
		Stdin:      stdin,
		Stderr:     &stderr,
		Exit:       func(code int) { exits = append(exits, code) },
		Now:        func() time.Time { return time.Date(2026, 11, 23, 17, 31, 44, 0, time.UTC) },
		FailureDir: func() (string, error) { return dir, nil },
	}
	if s.FailureDir != nil {
		base.FailureDir = s.FailureDir
	}
	seams := exitSeams{
		IsTerminal: func() bool { return len(s.Args) == 0 && s.RunTUI != nil },
		Getwd:      s.Getwd,
		Stdout:     &bytes.Buffer{},
		RunTUI:     s.RunTUI,
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("runTool let a panic escape: %v", r)
			}
		}()
		runTool(base, s.Args, seams)
	}()
	return wiringRun{Exits: exits, Stderr: stderr.String(), Dir: dir, StdinIn: stdin.Len() == 0}
}

func getwdOf(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

func noopTUI(context.Context, app.Service, tui.Options) error { return nil }

func requireSingleExit(t *testing.T, run wiringRun, want int) {
	t.Helper()
	if len(run.Exits) != 1 || run.Exits[0] != want {
		t.Fatalf("exit calls = %v, want exactly [%d]\nstderr: %s", run.Exits, want, run.Stderr)
	}
}

// requireReported asserts the failure message, path line, file and (when pause) the Enter wait.
func requireReported(t *testing.T, run wiringRun, message string, pause bool) []byte {
	t.Helper()
	requireSingleExit(t, run, 1)
	if !strings.Contains(run.Stderr, message) {
		t.Errorf("stderr lacks %q:\n%s", message, run.Stderr)
	}
	files, _ := filepath.Glob(filepath.Join(run.Dir, "mosaic-deploy-failure-*.txt"))
	if len(files) != 1 {
		t.Fatalf("failure files = %v, want exactly one\nstderr: %s", files, run.Stderr)
	}
	if !strings.Contains(run.Stderr, failurePathLinePrefix+files[0]) {
		t.Errorf("stderr does not name the failure file %s:\n%s", files[0], run.Stderr)
	}
	if got := strings.Contains(run.Stderr, fatal.PausePrompt); got != pause {
		t.Errorf("pause prompt shown = %v, want %v\nstderr: %s", got, pause, run.Stderr)
	}
	if run.StdinIn != pause {
		t.Errorf("waited for Enter = %v, want %v", run.StdinIn, pause)
	}
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read failure file: %v", err)
	}
	return content
}

func requireNothingReported(t *testing.T, run wiringRun) {
	t.Helper()
	if files, _ := filepath.Glob(filepath.Join(run.Dir, "*.txt")); len(files) != 0 {
		t.Errorf("failure files written: %v", files)
	}
	if strings.Contains(run.Stderr, fatal.PausePrompt) || run.StdinIn {
		t.Errorf("paused for Enter\nstderr: %s", run.Stderr)
	}
	if strings.Contains(run.Stderr, failurePathLinePrefix) {
		t.Errorf("stderr names a failure file:\n%s", run.Stderr)
	}
}

func TestRunTool_TUIOnWindows_RootResolutionFailure_ShowsMessageAndPathAndPauses(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(t.TempDir()), RunTUI: noopTUI})

	requireReported(t, run, "must be run from the MOSAIC repository root", true)
}

func TestRunTool_TUIOnWindows_WorkingDirectoryFailure_ShowsMessageAndPathAndPauses(t *testing.T) {
	getwd := func() (string, error) { return "", errors.New("cwd gone") }

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwd, RunTUI: noopTUI})

	requireReported(t, run, "cwd gone", true)
}

func TestRunTool_TUIOnWindows_CatalogLoadFailure_ShowsMessageAndPathAndPauses(t *testing.T) {
	root := newDuplicateIDMosaicRoot(t)

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: noopTUI})

	content := requireReported(t, run, "error: load catalog: ", true)
	for _, want := range []string{"Kind: failure", "Exit code: 1", "load catalog"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("failure file lacks %q:\n%s", want, content)
		}
	}
}

func TestRunTool_TUIOnWindows_TUIError_ShowsMessageAndPathAndPauses(t *testing.T) {
	root := newStackMosaicRoot(t)
	failing := func(context.Context, app.Service, tui.Options) error { return errors.New("terminal unusable") }

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: failing})

	requireReported(t, run, "error: terminal unusable", true)
}

func TestRunTool_TUIOnLinux_Failure_WritesFileButDoesNotPause(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "linux", Getwd: getwdOf(t.TempDir()), RunTUI: noopTUI})

	requireReported(t, run, "must be run from the MOSAIC repository root", false)
}

func TestRunTool_TUIEndsNormally_ExitsZeroWithoutReportOrPause(t *testing.T) {
	root := newStackMosaicRoot(t)

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: noopTUI})

	requireSingleExit(t, run, 0)
	requireNothingReported(t, run)
}

func TestRunTool_TUIInterrupted_KeepsStderrLineAndExitOneWithoutFileOrPause(t *testing.T) {
	root := newStackMosaicRoot(t)
	interrupted := func(context.Context, app.Service, tui.Options) error {
		return fmt.Errorf("program was interrupted: %w", tea.ErrInterrupted)
	}

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: interrupted})

	requireSingleExit(t, run, 1)
	if want := "error: " + fmt.Errorf("program was interrupted: %w", tea.ErrInterrupted).Error() + "\n"; run.Stderr != want {
		t.Errorf("stderr = %q, want exactly %q", run.Stderr, want)
	}
	requireNothingReported(t, run)
}

func TestRunTool_PanicInTUI_IsReportedWithMessageAndStackAndPauses(t *testing.T) {
	root := newStackMosaicRoot(t)
	panicking := func(context.Context, app.Service, tui.Options) error { panic("worker exploded") }

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(root), RunTUI: panicking})

	content := requireReported(t, run, "panic: worker exploded", true)
	if !strings.Contains(run.Stderr, "goroutine") {
		t.Errorf("stderr has no stack trace:\n%s", run.Stderr)
	}
	for _, want := range []string{"Kind: panic", "Exit code: 1", "Stack:"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("failure file lacks %q:\n%s", want, content)
		}
	}
}

func TestRunTool_CLIStartupFailure_WritesFileNamesItAndNeverPauses(t *testing.T) {
	root := newDuplicateIDMosaicRoot(t)
	args := renderJSONArgs(root, t.TempDir())

	run := runWiring(t, wiringSetup{GOOS: "windows", Args: args})

	requireReported(t, run, "error: load catalog: ", false)
	if _, _, ok := splitFailurePathLine(run.Stderr); !ok {
		t.Errorf("path line is not the last stderr line:\n%s", run.Stderr)
	}
}

func TestRunTool_CLIUsageWithoutTerminal_ExitsThreeWithoutReport(t *testing.T) {
	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(t.TempDir())})

	requireSingleExit(t, run, 3)
	if !strings.Contains(run.Stderr, "usage: mosaic-deploy") {
		t.Errorf("stderr lacks the usage line:\n%s", run.Stderr)
	}
	requireNothingReported(t, run)
}

func TestRunTool_CLISuccess_ExitsZeroWithoutReport(t *testing.T) {
	root := newStackMosaicRoot(t)

	run := runWiring(t, wiringSetup{GOOS: "windows", Args: renderJSONArgs(root, t.TempDir())})

	requireSingleExit(t, run, 0)
	requireNothingReported(t, run)
}

func TestRunTool_CLICommandRuntimeFailure_WritesFileNamesItAndKeepsExitOne(t *testing.T) {
	root := newStackMosaicRoot(t)

	run := runWiring(t, wiringSetup{GOOS: "windows", Args: renderMissingAgentArgs(root, t.TempDir())})

	requireReported(t, run, "error: ", false)
	if _, _, ok := splitFailurePathLine(run.Stderr); !ok {
		t.Errorf("path line is not the last stderr line:\n%s", run.Stderr)
	}
}

func TestRunTool_FailureFolderUnavailable_ShowsMessageWithoutPathLineAndStillPauses(t *testing.T) {
	unavailable := func() (string, error) { return "", errors.New("no folder") }

	run := runWiring(t, wiringSetup{GOOS: "windows", Getwd: getwdOf(t.TempDir()), RunTUI: noopTUI, FailureDir: unavailable})

	requireSingleExit(t, run, 1)
	if !strings.Contains(run.Stderr, "must be run from the MOSAIC repository root") {
		t.Errorf("stderr lacks the failure message:\n%s", run.Stderr)
	}
	if strings.Contains(run.Stderr, failurePathLinePrefix) {
		t.Errorf("stderr names a failure file although none could be written:\n%s", run.Stderr)
	}
	if !strings.Contains(run.Stderr, fatal.PausePrompt) || !run.StdinIn {
		t.Errorf("TUI start did not pause for Enter\nstderr: %s", run.Stderr)
	}
}
