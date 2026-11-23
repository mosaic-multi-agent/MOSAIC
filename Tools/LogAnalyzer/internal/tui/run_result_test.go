package tui

// Run reports how the TUI ended: the in-TUI outcome code plus a non-nil error only
// when the TUI could not run or ended abnormally. An externally interrupted TUI and
// the screens the user sees and quits normally are not errors.
//
// Only the "cannot run" and "killed" tests specify new behaviour; the interrupt,
// analysis-error, outcome-screen and quit-before-result tests are guards that keep
// today's codes and non-errors from regressing.

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// blockingLogSource never resolves a source until released, so the real analysis
// started by Init stays pending and the test decides what the model receives.
type blockingLogSource struct {
	tuiFakeLogSource
	release chan struct{}
}

func (b *blockingLogSource) Default(wd string) domain.Source {
	<-b.release
	return b.tuiFakeLogSource.Default(wd)
}

func (b *blockingLogSource) Classify(p string) domain.Source {
	<-b.release
	return b.tuiFakeLogSource.Classify(p)
}

// runOutcome is what Run returned.
type runOutcome struct {
	code int
	err  error
}

// runWithDriver calls Run with a program built by the NewProgram seam (no terminal,
// no input). drive is called with the running program and must make it end; it runs
// in its own goroutine and may Send messages.
func runWithDriver(t *testing.T, build func() []tea.ProgramOption, drive func(p *tea.Program)) runOutcome {
	t.Helper()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	opts := Options{
		Service: app.New(app.Deps{
			Source:      &blockingLogSource{release: release},
			Reader:      &tuiFakeEventReader{},
			Pricing:     &tuiFakePricingStore{path: "/test/pricing.yaml"},
			Clock:       &tuiFakeClock{},
			Interaction: &tuiFakeInteraction{},
		}),
		Version: "test",
		NewProgram: func(m tea.Model, extra ...tea.ProgramOption) *tea.Program {
			programOpts := append([]tea.ProgramOption{tea.WithInput(nil), tea.WithOutput(io.Discard)}, extra...)
			programOpts = append(programOpts, build()...)
			p := tea.NewProgram(m, programOpts...)
			if drive != nil {
				go drive(p)
			}
			return p
		},
	}

	done := make(chan runOutcome, 1)
	go func() {
		code, err := Run(opts)
		done <- runOutcome{code: code, err: err}
	}()
	select {
	case out := <-done:
		return out
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within 10s")
		return runOutcome{}
	}
}

// quitAfter sends msgs to the program, then Ctrl+C, which ends any screen normally.
func quitAfter(msgs ...tea.Msg) func(p *tea.Program) {
	return func(p *tea.Program) {
		for _, m := range msgs {
			p.Send(m)
		}
		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	}
}

func TestRun_ProgramCannotRun_ReturnsItsError(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	got := runWithDriver(t, func() []tea.ProgramOption { return []tea.ProgramOption{tea.WithContext(cancelled)} }, nil)

	if got.err == nil {
		t.Fatalf("Run error = nil, want the program error (code %d)", got.code)
	}
	if errors.Is(got.err, tea.ErrInterrupted) {
		t.Errorf("a program that could not run must not look like an interrupt: %v", got.err)
	}
	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
}

func TestRun_ExternallyInterrupted_IsNotAnErrorAndKeepsExitCodeOne(t *testing.T) {
	got := runWithDriver(t, func() []tea.ProgramOption { return nil }, sendOnly(tea.InterruptMsg{}))

	if got.err != nil {
		t.Errorf("Run error = %v, want nil for an interrupted TUI", got.err)
	}
	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
}

// sendOnly sends msgs and nothing else (the message itself ends the program).
func sendOnly(msgs ...tea.Msg) func(p *tea.Program) {
	return func(p *tea.Program) {
		for _, m := range msgs {
			p.Send(m)
		}
	}
}

func TestRun_AnalysisErrorScreenQuitNormally_IsNotAnError(t *testing.T) {
	failed := analysisDoneMsg{err: errors.New("parse pricing file: bad document")}

	got := runWithDriver(t, func() []tea.ProgramOption { return nil }, quitAfter(failed))

	if got.err != nil {
		t.Errorf("Run error = %v, want nil: the user saw the error screen and quit", got.err)
	}
	if got.code != 1 {
		t.Errorf("exit code = %d, want 1 (today's code for an in-TUI analysis error)", got.code)
	}
}

func TestRun_OutcomeScreensQuitNormally_KeepTheirCodeWithoutError(t *testing.T) {
	cases := []struct {
		name    string
		outcome app.Outcome
		want    int
	}{
		{"no data", app.OutcomeNoData, 2},
		{"source not found", app.OutcomeSourceNotFound, 3},
		{"source unusable", app.OutcomeSourceUnusable, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := analysisDoneMsg{result: app.Result{Outcome: tc.outcome}}

			got := runWithDriver(t, func() []tea.ProgramOption { return nil }, quitAfter(msg))

			if got.err != nil {
				t.Errorf("Run error = %v, want nil", got.err)
			}
			if got.code != tc.want {
				t.Errorf("exit code = %d, want %d", got.code, tc.want)
			}
		})
	}
}

func TestRun_QuitBeforeAnyAnalysisResult_ReturnsZeroWithoutError(t *testing.T) {
	got := runWithDriver(t, func() []tea.ProgramOption { return nil }, quitAfter())

	if got.err != nil || got.code != 0 {
		t.Errorf("Run = (%d, %v), want (0, nil)", got.code, got.err)
	}
}

func TestRun_ProgramKilledAfterItStarted_ReturnsAnErrorThatIsNotAnInterrupt(t *testing.T) {
	kill := func(p *tea.Program) {
		p.Send(analysisDoneMsg{result: app.Result{Outcome: app.OutcomeNoData}})
		p.Kill()
	}

	got := runWithDriver(t, func() []tea.ProgramOption { return nil }, kill)

	if got.err == nil {
		t.Fatalf("Run error = nil, want an error for a killed program (code %d)", got.code)
	}
	if errors.Is(got.err, tea.ErrInterrupted) {
		t.Errorf("a killed program must not look like an external interrupt: %v", got.err)
	}
}
