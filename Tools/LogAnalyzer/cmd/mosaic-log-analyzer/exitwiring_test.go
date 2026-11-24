package main

// Composition-root behaviour of the fatal-exit wiring, driven through injected
// seams: how each way the process can end is reported, whether a failure file is
// written, and whether a Windows TUI start waits for Enter.

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/fatal"

	"mosaic-log-analyzer/internal/tui"
)

// wiringRun is the observable result of one call of run.
type wiringRun struct {
	exits       []int
	stderr      string
	failureDir  string
	stdinReads  int
	runTUICalls int
}

// recorder is a goroutine-safe stderr buffer.
type recorder struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (r *recorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.b.Write(p)
}

func (r *recorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.b.String()
}

// countingReader serves one Enter and counts reads.
type countingReader struct {
	mu    sync.Mutex
	reads int
	r     io.Reader
}

func (c *countingReader) Read(p []byte) (int, error) {
	c.mu.Lock()
	c.reads++
	c.mu.Unlock()
	return c.r.Read(p)
}

// wiringCase describes one call of run.
type wiringCase struct {
	args   []string
	goos   string
	getwd  func() (string, error)
	runTUI func(opts tui.Options) (int, error)
	// runCLI replaces the CLI frontend (default: the real one).
	runCLI func(args []string) int
	// noFailureDir makes the failure-file location unresolvable.
	noFailureDir bool
	// terminal is the answer of the terminal check (default false).
	terminal bool
}

// callRun runs run with every seam replaced and the user config folder and working
// directory pointed at temp folders. A panic escaping run (for example the
// unimplemented wiring) fails the test instead of crashing the test binary.
func callRun(t *testing.T, c wiringCase) wiringRun {
	t.Helper()
	base := t.TempDir()
	for _, name := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME"} {
		t.Setenv(name, base)
	}
	fatal.Reset()
	t.Cleanup(fatal.Reset)

	failureDir := t.TempDir()
	stderr := &recorder{}
	stdin := &countingReader{r: strings.NewReader("\n")}
	res := wiringRun{failureDir: failureDir}

	getwd := c.getwd
	if getwd == nil {
		work := t.TempDir()
		getwd = func() (string, error) { return work, nil }
	}
	runTUI := c.runTUI
	if runTUI == nil {
		runTUI = func(tui.Options) (int, error) { return 0, nil }
	}
	cfg := fatal.Config{
		Args:       append([]string{"mosaic-log-analyzer"}, c.args...),
		GOOS:       c.goos,
		Stdin:      stdin,
		Stderr:     stderr,
		Exit:       func(code int) { res.exits = append(res.exits, code) },
		FailureDir: func() (string, error) { return failureDir, nil },
	}
	if c.noFailureDir {
		cfg.FailureDir = func() (string, error) { return "", errors.New("no location") }
	}
	seams := exitSeams{
		IsTerminal: func(uintptr) bool { return c.terminal },
		Getwd:      getwd,
		RunCLI:     c.runCLI,
		RunTUI: func(opts tui.Options) (int, error) {
			res.runTUICalls++
			return runTUI(opts)
		},
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("run panicked instead of ending through the reporter: %v", r)
			}
		}()
		run(cfg, c.args, seams)
	}()

	res.stderr = stderr.String()
	stdin.mu.Lock()
	res.stdinReads = stdin.reads
	stdin.mu.Unlock()
	return res
}

// failureFiles lists the failure files in dir.
func failureFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "mosaic-log-analyzer-failure-*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func onlyFailureFile(t *testing.T, dir string) (path, content string) {
	t.Helper()
	files := failureFiles(t, dir)
	if len(files) != 1 {
		t.Fatalf("failure files = %v, want exactly one", files)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	return files[0], string(data)
}

func assertExitedOnce(t *testing.T, got wiringRun, want int) {
	t.Helper()
	if len(got.exits) == 0 || got.exits[0] != want {
		t.Errorf("exit codes = %v, want first %d", got.exits, want)
	}
}

// assertReportedWithFile checks message, path line and file for an unexpected failure.
func assertReportedWithFile(t *testing.T, got wiringRun, wantMessage string) {
	t.Helper()
	assertExitedOnce(t, got, 1)
	if !strings.Contains(got.stderr, wantMessage) {
		t.Errorf("stderr = %q, want it to contain %q", got.stderr, wantMessage)
	}
	path, content := onlyFailureFile(t, got.failureDir)
	if !strings.Contains(got.stderr, path) {
		t.Errorf("stderr = %q, want it to name the failure file %q", got.stderr, path)
	}
	if !strings.Contains(content, wantMessage) {
		t.Errorf("failure file = %q, want it to contain %q", content, wantMessage)
	}
}

func assertNoReport(t *testing.T, got wiringRun) {
	t.Helper()
	if files := failureFiles(t, got.failureDir); len(files) != 0 {
		t.Errorf("failure files = %v, want none", files)
	}
	if strings.Contains(got.stderr, "failure report") {
		t.Errorf("stderr = %q, want no failure-file line", got.stderr)
	}
	if strings.Contains(got.stderr, fatal.PausePrompt) || got.stdinReads != 0 {
		t.Errorf("paused (stderr %q, stdin reads %d), want no pause", got.stderr, got.stdinReads)
	}
}

func TestRun_TUICannotRun_ShowsMessageAndFilePathAndPausesOnWindows(t *testing.T) {
	got := callRun(t, wiringCase{
		args:   []string{"--tui"},
		goos:   "windows",
		runTUI: func(tui.Options) (int, error) { return 1, errors.New("could not open a new TTY") },
	})

	assertReportedWithFile(t, got, "could not open a new TTY")
	if !strings.Contains(got.stderr, fatal.PausePrompt) || got.stdinReads == 0 {
		t.Errorf("stderr = %q, stdin reads = %d, want the Enter prompt and a wait", got.stderr, got.stdinReads)
	}
	path, _ := onlyFailureFile(t, got.failureDir)
	pathAt, promptAt := strings.Index(got.stderr, path), strings.Index(got.stderr, fatal.PausePrompt)
	if pathAt < 0 || promptAt < 0 || pathAt > promptAt {
		t.Errorf("stderr = %q, want the file path (%q) shown before the prompt", got.stderr, path)
	}
}

func TestRun_TUICannotRun_OnAnotherOSDoesNotPause(t *testing.T) {
	got := callRun(t, wiringCase{
		args:   []string{"--tui"},
		goos:   "linux",
		runTUI: func(tui.Options) (int, error) { return 1, errors.New("could not open a new TTY") },
	})

	assertReportedWithFile(t, got, "could not open a new TTY")
	if strings.Contains(got.stderr, fatal.PausePrompt) || got.stdinReads != 0 {
		t.Errorf("stderr = %q, stdin reads = %d, want no pause outside Windows", got.stderr, got.stdinReads)
	}
}

func TestRun_StartupFailureInTUIMode_ShowsMessageAndFilePathAndPausesOnWindows(t *testing.T) {
	got := callRun(t, wiringCase{
		args:  []string{"--tui"},
		goos:  "windows",
		getwd: func() (string, error) { return "", errors.New("directory is gone") },
	})

	assertReportedWithFile(t, got, "getting working directory")
	if !strings.Contains(got.stderr, "directory is gone") {
		t.Errorf("stderr = %q, want the cause", got.stderr)
	}
	if !strings.Contains(got.stderr, fatal.PausePrompt) || got.stdinReads == 0 {
		t.Errorf("stderr = %q, stdin reads = %d, want the Enter prompt and a wait", got.stderr, got.stdinReads)
	}
	if got.runTUICalls != 0 {
		t.Errorf("the TUI ran %d times after a startup failure", got.runTUICalls)
	}
}

func TestRun_StartupFailureInCLIMode_WritesFileAndNeverPauses(t *testing.T) {
	got := callRun(t, wiringCase{
		args:  []string{"report"},
		goos:  "windows",
		getwd: func() (string, error) { return "", errors.New("directory is gone") },
	})

	assertReportedWithFile(t, got, "getting working directory")
	if strings.Contains(got.stderr, fatal.PausePrompt) || got.stdinReads != 0 {
		t.Errorf("stderr = %q, stdin reads = %d, want no pause in CLI mode", got.stderr, got.stdinReads)
	}
}

func TestRun_TUIEndsWithOutcomeCode_ExitsWithItWithoutReport(t *testing.T) {
	// Code 1 here is an in-TUI analysis-error screen (or an externally interrupted
	// TUI) that the frontend returns without an error; 2 is no data, 3 and 4 the
	// source screens; 0 a normal quit.
	for _, code := range []int{0, 1, 2, 3, 4} {
		got := callRun(t, wiringCase{
			args:   []string{"--tui"},
			goos:   "windows",
			runTUI: func(tui.Options) (int, error) { return code, nil },
		})

		assertExitedOnce(t, got, code)
		assertNoReport(t, got)
		if got.stderr != "" {
			t.Errorf("code %d: stderr = %q, want nothing", code, got.stderr)
		}
	}
}

func TestRun_TUIPanic_ReportsMessageAndStackWithFileAndPause(t *testing.T) {
	got := callRun(t, wiringCase{
		args:   []string{"--tui"},
		goos:   "windows",
		runTUI: func(tui.Options) (int, error) { panic("boom in the frontend") },
	})

	assertExitedOnce(t, got, 1)
	if !strings.Contains(got.stderr, "boom in the frontend") || !strings.Contains(got.stderr, "goroutine") {
		t.Errorf("stderr = %q, want the panic value and a stack", got.stderr)
	}
	_, content := onlyFailureFile(t, got.failureDir)
	if !strings.Contains(content, "Kind: panic") || !strings.Contains(content, "Stack:") {
		t.Errorf("failure file = %q, want a panic report with its stack", content)
	}
	if !strings.Contains(got.stderr, fatal.PausePrompt) || got.stdinReads == 0 {
		t.Errorf("stderr = %q, stdin reads = %d, want the Enter prompt and a wait", got.stderr, got.stdinReads)
	}
}

// panickingModel panics in Init, as a bug in a screen would.
type panickingModel struct{}

func (panickingModel) Init() tea.Cmd                       { panic("screen bug") }
func (panickingModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return panickingModel{}, nil }
func (panickingModel) View() string                        { return "" }

func TestRun_PanicInsideTUIProgram_IsReportedAsPanicNotAsTUIError(t *testing.T) {
	got := callRun(t, wiringCase{
		args: []string{"--tui"},
		goos: "windows",
		runTUI: func(opts tui.Options) (int, error) {
			if opts.NewProgram == nil {
				t.Error("the TUI frontend got no guarded program builder")
				return 1, nil
			}
			p := opts.NewProgram(panickingModel{}, tea.WithInput(nil), tea.WithOutput(io.Discard))
			_, err := p.Run()
			return 1, err
		},
	})

	assertExitedOnce(t, got, 1)
	if !strings.Contains(got.stderr, "panic: screen bug") {
		t.Errorf("stderr = %q, want the panic reported with its value", got.stderr)
	}
	_, content := onlyFailureFile(t, got.failureDir)
	if !strings.Contains(content, "Kind: panic") {
		t.Errorf("failure file = %q, want Kind: panic", content)
	}
}

func TestRun_NoArgsOnTerminals_StartsTheTUI(t *testing.T) {
	got := callRun(t, wiringCase{terminal: true, goos: "windows"})

	if got.runTUICalls != 1 {
		t.Errorf("TUI runs = %d, want 1 when stdin and stdout are terminals", got.runTUICalls)
	}
	assertExitedOnce(t, got, 0)
}

func TestRun_CLIUsageError_ExitsWithDocumentedCodeWithoutReport(t *testing.T) {
	got := callRun(t, wiringCase{args: []string{"no-such-command"}, goos: "windows"})

	assertExitedOnce(t, got, 3)
	assertNoReport(t, got)
	if got.runTUICalls != 0 {
		t.Errorf("the TUI ran for a CLI command")
	}
}
