package main

// Composition-root behaviour of the fatal-exit wiring for CLI runs and for an
// unusable failure-file location, driven through injected seams.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-common/fatal"

	"mosaic-log-analyzer/internal/tui"
)

func TestRun_CLIPanic_ReportsMessageStackAndFileWithoutPause(t *testing.T) {
	got := callRun(t, wiringCase{
		args:   []string{"report"},
		goos:   "windows",
		runCLI: func([]string) int { panic("boom in the cli") },
	})

	assertExitedOnce(t, got, 1)
	if !strings.Contains(got.stderr, "boom in the cli") || !strings.Contains(got.stderr, "goroutine") {
		t.Errorf("stderr = %q, want the panic value and a stack", got.stderr)
	}
	path, content := onlyFailureFile(t, got.failureDir)
	if !strings.Contains(content, "Kind: panic") || !strings.Contains(content, "Stack:") {
		t.Errorf("failure file = %q, want a panic report with its stack", content)
	}
	if !strings.Contains(got.stderr, path) {
		t.Errorf("stderr = %q, want it to name the failure file %q", got.stderr, path)
	}
	if strings.Contains(got.stderr, fatal.PausePrompt) || got.stdinReads != 0 {
		t.Errorf("stderr = %q, stdin reads = %d, want no pause in CLI mode", got.stderr, got.stdinReads)
	}
}

func TestRun_CLIEndsWithItsCode_ExitsWithItWithoutReport(t *testing.T) {
	for _, code := range []int{0, 2, 3, 4, 5} {
		got := callRun(t, wiringCase{
			args:   []string{"report"},
			goos:   "windows",
			runCLI: func([]string) int { return code },
		})

		assertExitedOnce(t, got, code)
		assertNoReport(t, got)
	}
}

func TestRun_VersionOnWindowsTerminal_NeverPausesAndWritesNoFile(t *testing.T) {
	got := callRun(t, wiringCase{
		args:     []string{"--version"},
		goos:     "windows",
		terminal: true,
		runCLI:   func([]string) int { return 0 },
	})

	assertExitedOnce(t, got, 0)
	assertNoReport(t, got)
	if got.runTUICalls != 0 {
		t.Errorf("the TUI ran for --version")
	}
}

func TestRun_FailureFileLocationUnresolvable_StillShowsMessageWithoutSecondError(t *testing.T) {
	got := callRun(t, wiringCase{
		args:         []string{"--tui"},
		goos:         "windows",
		noFailureDir: true,
		runTUI:       func(tui.Options) (int, error) { return 1, errors.New("could not open a new TTY") },
	})

	assertExitedOnce(t, got, 1)
	if !strings.Contains(got.stderr, "could not open a new TTY") {
		t.Errorf("stderr = %q, want the message", got.stderr)
	}
	if strings.Contains(got.stderr, "failure report") || strings.Contains(got.stderr, "no location") {
		t.Errorf("stderr = %q, want no path line and no second error", got.stderr)
	}
	if files := failureFiles(t, got.failureDir); len(files) != 0 {
		t.Errorf("failure files = %v, want none", files)
	}
}
