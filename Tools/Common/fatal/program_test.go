package fatal

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type pingMsg struct{}

// probeModel panics at the configured place; "none" quits at once.
type probeModel struct {
	at      string
	started chan struct{}
}

//go:noinline
func probeUpdatePanics() { panic("probe boom") }

func (m probeModel) Init() tea.Cmd {
	switch m.at {
	case "init":
		probeUpdatePanics()
	case "cmd":
		return func() tea.Msg { probeUpdatePanics(); return nil }
	case "batch":
		return tea.Batch(
			func() tea.Msg { return nil },
			tea.Batch(func() tea.Msg { probeUpdatePanics(); return nil }),
		)
	case "update":
		return func() tea.Msg { return pingMsg{} }
	case "block":
		return func() tea.Msg { close(m.started); return nil }
	case "none":
		return tea.Quit
	}
	return nil
}

func (m probeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(pingMsg); ok && m.at == "update" {
		probeUpdatePanics()
	}
	return m, nil
}

func (m probeModel) View() string {
	if m.at == "view" {
		probeUpdatePanics()
	}
	return "probe"
}

type runResult struct {
	final tea.Model
	err   error
}

// runGuarded builds the program through the reporter and runs it, failing the test
// on a panic or a hang.
func runGuarded(t *testing.T, r *Reporter, m tea.Model, out *bytes.Buffer) runResult {
	t.Helper()
	ch := make(chan runResult, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				ch <- runResult{err: nil, final: panicked{p}}
			}
		}()
		p := r.NewProgram(m, tea.WithInput(nil), tea.WithOutput(out), tea.WithoutSignals())
		f, err := p.Run()
		ch <- runResult{final: f, err: err}
	}()
	select {
	case res := <-ch:
		if p, ok := res.final.(panicked); ok {
			t.Fatalf("NewProgram/Run panicked instead of running: %v", p.v)
		}
		return res
	case <-time.After(10 * time.Second):
		t.Fatal("program did not end")
		return runResult{}
	}
}

// startedProgram is a program built and run through the reporter in a goroutine. A
// panic of NewProgram or Run is forwarded to the test instead of being swallowed.
type startedProgram struct {
	p     *tea.Program
	ready chan struct{}
	res   chan runResult
}

func startProgram(r *Reporter, m tea.Model, out io.Writer, opts ...tea.ProgramOption) *startedProgram {
	s := &startedProgram{ready: make(chan struct{}), res: make(chan runResult, 1)}
	go func() {
		defer func() {
			if v := recover(); v != nil {
				s.res <- runResult{final: panicked{v}}
				close(s.ready)
			}
		}()
		all := append([]tea.ProgramOption{tea.WithInput(nil), tea.WithOutput(out), tea.WithoutSignals()}, opts...)
		s.p = r.NewProgram(m, all...)
		close(s.ready)
		f, err := s.p.Run()
		s.res <- runResult{final: f, err: err}
	}()
	return s
}

// waitReady returns once NewProgram returned, failing at once when it panicked.
func (s *startedProgram) waitReady(t *testing.T) {
	t.Helper()
	select {
	case <-s.ready:
	case <-time.After(10 * time.Second):
		t.Fatal("NewProgram did not return")
	}
	if s.p == nil {
		s.wait(t)
	}
}

// wait returns the result of Run, failing at once when NewProgram or Run panicked.
func (s *startedProgram) wait(t *testing.T) runResult {
	t.Helper()
	select {
	case res := <-s.res:
		if p, ok := res.final.(panicked); ok {
			t.Fatalf("NewProgram/Run panicked instead of running: %v", p.v)
		}
		return res
	case <-time.After(10 * time.Second):
		t.Fatal("program did not end")
		return runResult{}
	}
}

type panicked struct{ v any }

func (panicked) Init() tea.Cmd                         { return nil }
func (p panicked) Update(tea.Msg) (tea.Model, tea.Cmd) { return p, nil }
func (panicked) View() string                          { return "" }

func TestProgram_WithoutPanicBehavesLikeTeaAndUnwrapsToToolModel(t *testing.T) {
	g := newRig(t, nil)
	var out bytes.Buffer

	res := runGuarded(t, g.r, probeModel{at: "none"}, &out)

	if res.err != nil {
		t.Fatalf("Run error = %v, want nil", res.err)
	}
	var final tea.Model
	guard(t, func() { final = UnwrapModel(res.final) })
	if pm, ok := final.(probeModel); !ok || pm.at != "none" {
		t.Fatalf("UnwrapModel returned %T, want the tool's own model", final)
	}
	guard(t, func() { g.r.Exit(0) })
	if g.stderr.String() != "" || len(failureFiles(t, g.dir)) != 0 {
		t.Fatalf("a clean TUI run must not report: %q", g.stderr.String())
	}
}

func TestUnwrapModel_ReturnsForeignModelUnchanged(t *testing.T) {
	m := probeModel{at: "none"}

	var got tea.Model
	guard(t, func() { got = UnwrapModel(m) })

	if pm, ok := got.(probeModel); !ok || pm.at != "none" {
		t.Fatalf("UnwrapModel changed a foreign model: %T", got)
	}
}

func TestProgram_PanicIsRecordedAndReportedByNextTerminatingCall(t *testing.T) {
	for _, at := range []string{"init", "cmd", "batch", "update", "view"} {
		t.Run(at, func(t *testing.T) {
			g := newRig(t, nil)
			var out bytes.Buffer
			stop := captureStdout(t)

			res := runGuarded(t, g.r, probeModel{at: at, started: make(chan struct{})}, &out)
			realStdout := stop()

			if res.err == nil {
				t.Fatalf("Run must return an error after a guarded panic")
			}
			if strings.Contains(realStdout, "Caught panic") || strings.Contains(out.String(), "Caught panic") {
				t.Fatalf("Bubble Tea's own panic output must not appear: stdout=%q out=%q", realStdout, out.String())
			}
			guard(t, func() { g.r.Fail("tui: " + res.err.Error()) })

			errOut := g.stderr.String()
			if !strings.Contains(errOut, "panic: probe boom") || !strings.Contains(errOut, "probeUpdatePanics") {
				t.Fatalf("report must carry the panic value and stack: %q", errOut)
			}
			if strings.Contains(errOut, "tui: ") {
				t.Fatalf("the tool's own message must be replaced by the panic report: %q", errOut)
			}
			_, file := onlyFailureFile(t, g.dir)
			if !strings.Contains(file, "Kind: panic") || !strings.Contains(file, "Stack:") {
				t.Fatalf("file must be a panic report:\n%s", file)
			}
			if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
				t.Fatalf("exit codes = %v, want [1]", codes)
			}
		})
	}
}

func TestProgram_RecordedPanicWinsOverSuccessExit(t *testing.T) {
	g := newRig(t, nil)
	var out bytes.Buffer
	runGuarded(t, g.r, probeModel{at: "update"}, &out)

	guard(t, func() { g.r.Exit(0) })

	if !strings.Contains(g.stderr.String(), "panic: probe boom") {
		t.Fatalf("Exit(0) after a recorded panic must report it: %q", g.stderr.String())
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
		t.Fatalf("exit codes = %v, want [1]", codes)
	}
}

func TestProgram_RecordedPanicWinsOverCLIOutcomeCode(t *testing.T) {
	g := newRig(t, nil)
	var out bytes.Buffer
	runGuarded(t, g.r, probeModel{at: "update"}, &out)

	guard(t, func() { g.r.ExitCLI(2) })

	if !strings.Contains(g.stderr.String(), "panic: probe boom") {
		t.Fatalf("ExitCLI(2) after a recorded panic must report it: %q", g.stderr.String())
	}
}

func TestProgram_WindowsTUIPanicPauses(t *testing.T) {
	g := newRig(t, func(c *Config) { c.Mode = ModeTUI; c.GOOS = "windows" })
	var out bytes.Buffer
	res := runGuarded(t, g.r, probeModel{at: "update"}, &out)

	guard(t, func() { g.r.Fail("tui: " + res.err.Error()) })

	if !strings.Contains(g.stderr.String(), PausePrompt) || g.stdin.Reads() == 0 {
		t.Fatalf("a panicking Windows TUI must show the panic and wait for Enter: %q", g.stderr.String())
	}
}

func TestProgram_RoutedPanicWhileProgramRunsStopsProgramAndReportsOnce(t *testing.T) {
	g := newRig(t, nil)
	var out bytes.Buffer
	started := make(chan struct{})
	sp := startProgram(g.r, probeModel{at: "block", started: started}, &out)
	sp.waitReady(t)
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("program did not start")
	}

	runPanickingGoroutine(t, panicInNamedWorker)

	sp.wait(t)
	if n := strings.Count(g.stderr.String(), "panic: worker boom"); n != 1 {
		t.Fatalf("want one panic report, got %d in %q", n, g.stderr.String())
	}
	if n := len(failureFiles(t, g.dir)); n != 1 {
		t.Fatalf("want one failure file, got %d", n)
	}
}
