package fatal

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// captureStdout swaps os.Stdout for a pipe and returns a stop function that restores
// it and returns everything written meanwhile. Bubble Tea prints its own "Caught
// panic" text to the real os.Stdout, not to the program output.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	orig := os.Stdout
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = wr
	collected := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(rd)
		collected <- string(b)
	}()
	stopped := false
	stop := func() string {
		if stopped {
			return ""
		}
		stopped = true
		os.Stdout = orig
		_ = wr.Close()
		s := <-collected
		_ = rd.Close()
		return s
	}
	t.Cleanup(func() { stop() })
	return stop
}

// twoPanicModel starts two commands that panic with different values; the second one
// only after release is closed, so "first boom" is recorded first.
type twoPanicModel struct{ release, secondRan chan struct{} }

func (m twoPanicModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { panic("first boom") },
		func() tea.Msg {
			<-m.release
			defer close(m.secondRan) // runs while the second panic unwinds
			panic("second boom")
		},
	)
}
func (m twoPanicModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m twoPanicModel) View() string                        { return "two" }

func TestProgram_OnlyFirstPanicIsRecorded(t *testing.T) {
	g := newRig(t, nil)
	var out bytes.Buffer
	release := make(chan struct{})
	secondRan := make(chan struct{})

	runGuarded(t, g.r, twoPanicModel{release: release, secondRan: secondRan}, &out)
	close(release)
	select {
	case <-secondRan:
	case <-time.After(10 * time.Second):
		t.Fatal("the second panicking command never ran")
	}
	time.Sleep(50 * time.Millisecond) // grace for the guard to finish recording the second panic
	guard(t, func() { g.r.Fail("tui: killed") })

	got := g.stderr.String()
	if !strings.Contains(got, "panic: first boom") || strings.Contains(got, "second boom") {
		t.Fatalf("want only the first panic reported, got %q", got)
	}
}

func TestProgram_PanicDoesNotLetBubbleTeaPrintItsOwnOutput(t *testing.T) {
	for _, at := range []string{"init", "cmd", "update", "view"} {
		t.Run(at, func(t *testing.T) {
			g := newRig(t, nil)
			var out bytes.Buffer
			stop := captureStdout(t)

			runGuarded(t, g.r, probeModel{at: at, started: make(chan struct{})}, &out)
			realStdout := stop()

			if strings.Contains(realStdout, "Caught panic") || strings.Contains(out.String(), "Caught panic") {
				t.Fatalf("Bubble Tea's own panic output must not appear: stdout=%q out=%q", realStdout, out.String())
			}
		})
	}
}

// counterModel counts incMsg messages and renders the count.
type incMsg struct{}

type counterModel struct{ n int }

func (m counterModel) Init() tea.Cmd { return nil }
func (m counterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(incMsg); ok {
		m.n++
	}
	return m, nil
}
func (m counterModel) View() string { return "count: " + string(rune('0'+m.n)) }

func TestProgram_SendAndQuitWorkUnchanged(t *testing.T) {
	g := newRig(t, nil)
	var out bytes.Buffer
	sp := startProgram(g.r, counterModel{}, &out)
	sp.waitReady(t)

	sp.p.Send(incMsg{})
	sp.p.Send(incMsg{})
	sp.p.Quit()

	res := sp.wait(t)
	if res.err != nil {
		t.Fatalf("Run error = %v, want nil", res.err)
	}
	var final tea.Model
	guard(t, func() { final = UnwrapModel(res.final) })
	if cm, ok := final.(counterModel); !ok || cm.n != 2 {
		t.Fatalf("final model = %#v, want counterModel with n=2", final)
	}
	if !strings.Contains(out.String(), "count: 2") {
		t.Fatalf("the tool's View must be rendered, got %q", out.String())
	}
}
