package tui

// forcestop_control_test.go verifies the root model's handling of the
// progress screen's force-stop control: a confirmed force stop raises the shared
// force-stop request the session reads, the gate and the request are recorded
// in the debug log, and a force stop and a graceful stop can both be requested
// without interfering.

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runflow"
)

const (
	forceStopRequestedStatus = "Force stop requested: stopping the running invocation"
	forceStopNothingStatus   = "Nothing is running: force stop not applied"
	forceStopCancelStatus    = "Force stop cancelled"
)

// newForceStopModel returns a rootModel on the progress screen wired to the
// given recorder, graceful-stop signal and force-stop signal.
func newForceStopModel(rec *tuiRecordingLogger, stop *session.StopSignal, force *session.ForceStopSignal) *rootModel {
	sess := &stubNavSession{outcome: domain.RunOutcome{Status: domain.RunCompleted}}
	m := newRootModel(context.Background(), sess, Options{
		Theme:      tuicommon.DefaultTheme(),
		StopSignal: stop,
		ForceStop:  force,
		Debug:      rec,
	})
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	return m
}

// openWindow opens a force-stop window the way the session does around an
// invocation.
func openWindow(force *session.ForceStopSignal) (context.Context, func() bool) {
	return force.Watch(context.Background())
}

func TestForceStopControl_ConfirmedWhileRunning_CancelsOpenWindowAndAcknowledges(t *testing.T) {
	force := session.NewForceStopSignal()
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), force)
	wctx, release := openWindow(force)

	pressProgressRune(m, "x")
	pressProgressRune(m, "y")

	if wctx.Err() == nil {
		t.Error("the open invocation window was not cancelled by a confirmed force stop")
	}
	if !release() {
		t.Error("release() = false: the session would not classify the invocation as force-stopped")
	}
	if view := m.progressScreen.View(); !strings.Contains(view, forceStopRequestedStatus) {
		t.Errorf("View() lacks the acknowledgement %q:\n%s", forceStopRequestedStatus, view)
	}
}

func TestForceStopControl_ConfirmedWhileIdle_ReportsNothingRunning(t *testing.T) {
	force := session.NewForceStopSignal()
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), force)

	pressProgressRune(m, "x")
	pressProgressRune(m, "y")

	if view := m.progressScreen.View(); !strings.Contains(view, forceStopNothingStatus) {
		t.Errorf("View() lacks %q when no invocation was running:\n%s", forceStopNothingStatus, view)
	}
	if view := m.progressScreen.View(); strings.Contains(view, forceStopRequestedStatus) {
		t.Errorf("View() claims a request was applied when nothing was running:\n%s", view)
	}
}

func TestForceStopControl_ConfirmedWhileIdle_DoesNotKillALaterInvocation(t *testing.T) {
	force := session.NewForceStopSignal()
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), force)
	pressProgressRune(m, "x")
	pressProgressRune(m, "y")
	if view := m.progressScreen.View(); !strings.Contains(view, forceStopNothingStatus) {
		t.Fatalf("precondition: the confirmation was not acknowledged as a no-op:\n%s", view)
	}

	wctx, release := openWindow(force)
	m.Update(runflow.TickMsg(time.Now()))

	if wctx.Err() != nil {
		t.Error("a force stop confirmed between invocations killed the next invocation")
	}
	if release() {
		t.Error("release() = true for an invocation nobody force-stopped")
	}
}

func TestForceStopControl_Confirmation_IsOneRequestNotALatch(t *testing.T) {
	force := session.NewForceStopSignal()
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), force)
	_, firstRelease := openWindow(force)
	pressProgressRune(m, "x")
	pressProgressRune(m, "y")
	if !firstRelease() {
		t.Fatal("precondition: the first force stop was not applied")
	}

	wctx, release := openWindow(force)
	m.Update(runflow.TickMsg(time.Now()))
	pressProgressRune(m, "z")

	if wctx.Err() != nil || release() {
		t.Error("later updates re-raised the force-stop request against the next invocation")
	}
}

func TestForceStopControl_Cancelled_LeavesRunUntouched(t *testing.T) {
	rec := &tuiRecordingLogger{}
	force := session.NewForceStopSignal()
	m := newForceStopModel(rec, session.NewStopSignal(), force)
	wctx, release := openWindow(force)

	pressProgressRune(m, "x")
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if wctx.Err() != nil || release() {
		t.Error("cancelling the gate force-stopped the running invocation")
	}
	if got := rec.eventsNamed(domain.EventTUIForceStopRequested); len(got) != 0 {
		t.Errorf("recorded %d force-stop request entries for a cancelled gate", len(got))
	}
	if view := m.progressScreen.View(); !strings.Contains(view, forceStopCancelStatus) {
		t.Errorf("View() lacks the cancel acknowledgement %q:\n%s", forceStopCancelStatus, view)
	}
}

func TestForceStopControl_GateEvents_AreLogged(t *testing.T) {
	rec := &tuiRecordingLogger{}
	force := session.NewForceStopSignal()
	m := newForceStopModel(rec, session.NewStopSignal(), force)
	openWindow(force)

	pressProgressRune(m, "x")
	pressProgressRune(m, "y")

	entered := requireOneEntry(t, rec, domain.EventTUIForceStopGateEntered)
	if key := requireField(t, entered, "key"); key != "x" {
		t.Errorf("gate-entered key field = %q, want %q", key, "x")
	}
	resolved := requireOneEntry(t, rec, domain.EventTUIForceStopGateResolved)
	if key := requireField(t, resolved, "key"); key != "y" {
		t.Errorf("gate-resolved key field = %q, want %q", key, "y")
	}
	if c := requireField(t, resolved, "confirmed"); c != "true" {
		t.Errorf("gate-resolved confirmed field = %q, want %q", c, "true")
	}
	requested := requireOneEntry(t, rec, domain.EventTUIForceStopRequested)
	if a := requireField(t, requested, "applied"); a != "true" {
		t.Errorf("requested applied field = %q, want %q while an invocation was running", a, "true")
	}
	want := []string{
		domain.EventTUIForceStopGateEntered,
		domain.EventTUIForceStopGateResolved,
		domain.EventTUIForceStopRequested,
	}
	if got := rec.eventOrder(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("recorded event sequence = %v, want %v", got, want)
	}
}

func TestForceStopControl_RequestLog_RecordsNotAppliedWhenIdle(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newForceStopModel(rec, session.NewStopSignal(), session.NewForceStopSignal())

	pressProgressRune(m, "x")
	pressProgressRune(m, "y")

	requested := requireOneEntry(t, rec, domain.EventTUIForceStopRequested)
	if a := requireField(t, requested, "applied"); a != "false" {
		t.Errorf("requested applied field = %q, want %q when nothing was running", a, "false")
	}
}

func TestForceStopControl_CancelLog_RecordsUnconfirmedResolution(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newForceStopModel(rec, session.NewStopSignal(), session.NewForceStopSignal())

	pressProgressRune(m, "x")
	pressProgressRune(m, "n")

	resolved := requireOneEntry(t, rec, domain.EventTUIForceStopGateResolved)
	if c := requireField(t, resolved, "confirmed"); c != "false" {
		t.Errorf("gate-resolved confirmed field = %q, want %q", c, "false")
	}
}

func TestForceStopControl_IgnoredKeysWhileGateOpen_LogNothing(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newForceStopModel(rec, session.NewStopSignal(), session.NewForceStopSignal())
	pressProgressRune(m, "x")
	before := len(rec.snapshot())
	if before == 0 {
		t.Fatal("precondition: opening the gate recorded nothing")
	}

	for _, key := range []string{"s", "a", "z", "m", "h"} {
		pressProgressRune(m, key)
	}

	if after := len(rec.snapshot()); after != before {
		t.Errorf("ignored keys added %d log entries: %v", after-before, rec.eventOrder())
	}
}

func TestForceStopControl_ForceStop_DoesNotArmGracefulStop(t *testing.T) {
	rec := &tuiRecordingLogger{}
	stop := session.NewStopSignal()
	force := session.NewForceStopSignal()
	m := newForceStopModel(rec, stop, force)
	openWindow(force)

	pressProgressRune(m, "x")
	pressProgressRune(m, "y")

	if stop.Requested() {
		t.Error("a force stop armed the graceful-stop signal")
	}
	if got := rec.eventsNamed(domain.EventTUIStopSignalArmed); len(got) != 0 {
		t.Error("a force stop logged a graceful-stop arming")
	}
}

func TestForceStopControl_GracefulStop_DoesNotRaiseForceStop(t *testing.T) {
	force := session.NewForceStopSignal()
	stop := session.NewStopSignal()
	m := newForceStopModel(&tuiRecordingLogger{}, stop, force)
	wctx, release := openWindow(force)

	pressProgressRune(m, "s")
	pressProgressRune(m, "y")

	if !stop.Requested() {
		t.Fatal("precondition: the graceful stop was not armed")
	}
	if wctx.Err() != nil || release() {
		t.Error("a graceful stop force-stopped the running invocation")
	}
}

func TestForceStopControl_BothStopsCanBeRequested(t *testing.T) {
	orders := map[string][]string{
		"graceful then force": {"s", "y", "x", "y"},
		"force then graceful": {"x", "y", "s", "y"},
	}
	for name, keys := range orders {
		t.Run(name, func(t *testing.T) {
			stop := session.NewStopSignal()
			force := session.NewForceStopSignal()
			m := newForceStopModel(&tuiRecordingLogger{}, stop, force)
			wctx, release := openWindow(force)

			for _, k := range keys {
				pressProgressRune(m, k)
			}

			if !stop.Requested() {
				t.Error("the graceful stop is not armed")
			}
			if wctx.Err() == nil || !release() {
				t.Error("the force stop was not applied to the running invocation")
			}
		})
	}
}

func TestForceStopControl_CtrlC_WhileGateOpen_CancelsAndQuits(t *testing.T) {
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), session.NewForceStopSignal())
	pressProgressRune(m, "x")
	if !m.progressScreen.ForceStopConfirmPending() {
		t.Fatal("precondition: the force-stop gate must be open")
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if m.ctx.Err() == nil {
		t.Error("ctx.Err() = nil after ctrl+c with the force-stop gate open; force-quit must stay available")
	}
	if cmd == nil {
		t.Error("cmd = nil after ctrl+c with the force-stop gate open; want tea.Quit")
	}
}

func TestForceStopControl_RestartStopReset_ClosesTheGate(t *testing.T) {
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), session.NewForceStopSignal())
	pressProgressRune(m, "x")
	if !m.progressScreen.ForceStopConfirmPending() {
		t.Fatal("precondition: the force-stop gate must be open")
	}

	m.resetStopStateForRestart()

	if m.progressScreen.ForceStopConfirmPending() {
		t.Error("the force-stop gate stayed open across a restart")
	}
}

func TestForceStopControl_CancelByEsc_LogsTheEscKey(t *testing.T) {
	rec := &tuiRecordingLogger{}
	m := newForceStopModel(rec, session.NewStopSignal(), session.NewForceStopSignal())

	pressProgressRune(m, "x")
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	resolved := requireOneEntry(t, rec, domain.EventTUIForceStopGateResolved)
	if key := requireField(t, resolved, "key"); key != "esc" {
		t.Errorf("gate-resolved key field = %q, want %q", key, "esc")
	}
	if c := requireField(t, resolved, "confirmed"); c != "false" {
		t.Errorf("gate-resolved confirmed field = %q, want %q", c, "false")
	}
}

func TestForceStopControl_UppercaseConfirm_ArmsTheRequest(t *testing.T) {
	force := session.NewForceStopSignal()
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), force)
	wctx, release := openWindow(force)

	pressProgressRune(m, "x")
	pressProgressRune(m, "Y")

	if wctx.Err() == nil || !release() {
		t.Error("an uppercase Y did not confirm the force stop")
	}
}

// TestForceStopControl_GracefulGateOpen_ForceKeyDoesNotOpenASecondGate pins the
// reverse direction of the one-gate-at-a-time rule at the root level.
func TestForceStopControl_GracefulGateOpen_ForceKeyDoesNotOpenASecondGate(t *testing.T) {
	rec := &tuiRecordingLogger{}
	stop := session.NewStopSignal()
	force := session.NewForceStopSignal()
	m := newForceStopModel(rec, stop, force)
	wctx, release := openWindow(force)

	pressProgressRune(m, "s")
	pressProgressRune(m, "x")
	pressProgressRune(m, "y")

	if got := rec.eventsNamed(domain.EventTUIForceStopGateEntered); len(got) != 0 {
		t.Errorf("recorded %d force-stop gate entries while the graceful gate was open", len(got))
	}
	if !stop.Requested() {
		t.Error("the graceful stop was not armed by its own confirmation")
	}
	if wctx.Err() != nil || release() {
		t.Error("the invocation was force-stopped through the graceful gate")
	}
}

// TestForceStopControl_NoForceStopSignal_DoesNotPanic covers a model built
// without a force-stop request, as the non-wired constructors do.
func TestForceStopControl_NoForceStopSignal_DoesNotPanic(t *testing.T) {
	m := newForceStopModel(&tuiRecordingLogger{}, session.NewStopSignal(), nil)

	pressProgressRune(m, "x")
	pressProgressRune(m, "y")
	m.Update(runflow.TickMsg(time.Now()))
}
