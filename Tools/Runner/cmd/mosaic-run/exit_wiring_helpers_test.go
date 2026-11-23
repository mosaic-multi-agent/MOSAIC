package main

// exit_wiring_helpers_test.go holds the recording seams and assertions shared by the exit
// wiring tests. Nothing in this file is a test itself.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-common/fatal"

	"mosaic-run/internal/session"
	"mosaic-run/internal/tui"
)

const failurePathLinePrefix = "failure report written to "

// wiringRun is the observable result of one runTool call.
type wiringRun struct {
	Exits   []int
	Stderr  string
	Stdout  string
	Dir     string // failure folder
	StdinIn bool   // true when the pause read the input
}

// wiringSetup describes one run: the platform, the seams and the arguments.
type wiringSetup struct {
	GOOS       string
	Args       []string
	IsTerminal bool
	Getwd      func() (string, error)
	RunTUI     func(ctx context.Context, sess session.Session, opts tui.Options) error
	// FailureDirErr, when set, makes the failure folder lookup fail with this error.
	FailureDirErr error
}

// runWiring calls runTool with recording seams. A panic that escapes runTool fails the test.
func runWiring(t *testing.T, s wiringSetup) wiringRun {
	t.Helper()
	fatal.Reset()
	t.Cleanup(fatal.Reset)
	dir := t.TempDir()
	var stderr, stdout bytes.Buffer
	stdin := bytes.NewReader([]byte("\n"))
	var exits []int
	base := fatal.Config{
		Args:       append([]string{"mosaic-run"}, s.Args...),
		GOOS:       s.GOOS,
		Stdin:      stdin,
		Stderr:     &stderr,
		Exit:       func(code int) { exits = append(exits, code) },
		Now:        func() time.Time { return time.Date(2026, 11, 23, 17, 31, 44, 0, time.UTC) },
		FailureDir: func() (string, error) { return dir, nil },
	}
	if s.FailureDirErr != nil {
		base.FailureDir = func() (string, error) { return "", s.FailureDirErr }
	}
	seams := exitSeams{
		IsTerminal: func() bool { return s.IsTerminal },
		Getwd:      s.Getwd,
		Stdout:     &stdout,
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
	return wiringRun{Exits: exits, Stderr: stderr.String(), Stdout: stdout.String(), Dir: dir, StdinIn: stdin.Len() == 0}
}

func getwdOf(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

func failingGetwd() (string, error) { return "", errors.New("cwd gone") }

func panickingGetwd() (string, error) { panic("getwd exploded") }

func noopTUI(context.Context, session.Session, tui.Options) error { return nil }

func requireSingleExit(t *testing.T, run wiringRun, want int) {
	t.Helper()
	if len(run.Exits) != 1 || run.Exits[0] != want {
		t.Fatalf("exit calls = %v, want exactly [%d]\nstderr: %s", run.Exits, want, run.Stderr)
	}
}

// requireReported asserts the failure message, path line, file and (when pause) the Enter wait.
// It returns the failure file content.
func requireReported(t *testing.T, run wiringRun, message string, pause bool) string {
	t.Helper()
	requireSingleExit(t, run, 1)
	if !strings.Contains(run.Stderr, message) {
		t.Errorf("stderr lacks %q:\n%s", message, run.Stderr)
	}
	files, _ := filepath.Glob(filepath.Join(run.Dir, "mosaic-run-failure-*.txt"))
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
	if !strings.Contains(string(content), "Tool: mosaic-run "+ToolVersion) {
		t.Errorf("failure file lacks the tool and version line:\n%s", content)
	}
	return string(content)
}

// requireNothingReported asserts no failure file, no path line and no pause.
func requireNothingReported(t *testing.T, run wiringRun) {
	t.Helper()
	if files, _ := filepath.Glob(filepath.Join(run.Dir, "*.txt")); len(files) != 0 {
		t.Errorf("failure files written: %v", files)
	}
	if strings.Contains(run.Stderr, failurePathLinePrefix) {
		t.Errorf("stderr names a failure file:\n%s", run.Stderr)
	}
	if strings.Contains(run.Stderr, fatal.PausePrompt) || run.StdinIn {
		t.Errorf("paused for Enter (prompt shown or input read)\nstderr: %s", run.Stderr)
	}
}
