package fatal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const leaveAltScreen = "\x1b[?1049l"

// newSharedOutputRig returns a rig whose reporter stderr is the very buffer the
// program renders into, so the order of terminal restore and report is observable.
func newSharedOutputRig(t *testing.T) (*rig, *lockedBuffer) {
	t.Helper()
	shared := &lockedBuffer{}
	g := newRig(t, func(c *Config) { c.Stderr = shared })
	return g, shared
}

func assertRestoredBeforeReport(t *testing.T, all, report string) {
	t.Helper()
	iRestore := strings.Index(all, leaveAltScreen)
	iReport := strings.Index(all, report)
	if iRestore < 0 || iReport < 0 {
		t.Fatalf("want both the screen restore and %q in the output, got %q", report, all)
	}
	if iRestore > iReport {
		t.Fatalf("the terminal must be restored before the report is printed: %q", all)
	}
}

func TestProgram_GuardedPanicRestoresTerminalBeforeReport(t *testing.T) {
	g, shared := newSharedOutputRig(t)
	sp := startProgram(g.r, probeModel{at: "update"}, shared, tea.WithAltScreen())

	res := sp.wait(t)
	if res.err == nil {
		t.Fatalf("Run must return an error after a guarded panic")
	}
	guard(t, func() { g.r.Fail("tui: " + res.err.Error()) })

	assertRestoredBeforeReport(t, shared.String(), "panic: probe boom")
}

func TestProgram_RoutedPanicRestoresTerminalBeforeReport(t *testing.T) {
	g, shared := newSharedOutputRig(t)
	started := make(chan struct{})
	sp := startProgram(g.r, probeModel{at: "block", started: started}, shared, tea.WithAltScreen())
	sp.waitReady(t)
	<-started

	runPanickingGoroutine(t, panicInNamedWorker)
	sp.wait(t)

	assertRestoredBeforeReport(t, shared.String(), "panic: worker boom")
}
