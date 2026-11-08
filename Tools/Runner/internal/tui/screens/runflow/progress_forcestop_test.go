package runflow

// progress_forcestop_test.go verifies the progress screen's force-stop control:
// the 'x' key opens its own sticky confirmation gate, only an explicit confirm or
// cancel key resolves it, every resolution is recorded for the root model to
// act on and log, and the graceful-stop gate and all other keys keep their
// behaviour.

import (
	"strings"
	"testing"
)

const forceStopBannerText = "Force-stop the running invocation now? (y/n)"

// forceStopGateScreen returns a screen with the force-stop gate open and its
// entry event already drained.
func forceStopGateScreen(t *testing.T) *ProgressScreen {
	t.Helper()
	s := NewProgressScreen(80, 24, progressStyles())
	pressProgressKey(s, "x")
	if !s.ForceStopConfirmPending() {
		t.Fatal("precondition: ForceStopConfirmPending() = false after 'x'")
	}
	s.TakeForceStopGateEvents()
	return s
}

func TestProgressScreen_ForceStopKey_OpensItsOwnGate(t *testing.T) {
	for _, key := range []string{"x", "X"} {
		t.Run(key, func(t *testing.T) {
			s := NewProgressScreen(80, 24, progressStyles())

			pressProgressKey(s, key)

			if !s.ForceStopConfirmPending() {
				t.Fatalf("ForceStopConfirmPending() = false after %q, want true", key)
			}
			if s.ConfirmPending() || s.GracefulStop() {
				t.Errorf("the graceful-stop gate changed after %q; the two gates are separate", key)
			}
			events := s.TakeForceStopGateEvents()
			if len(events) != 1 || events[0].Kind != ForceStopGateEntered || events[0].Key != key {
				t.Errorf("events = %+v, want one ForceStopGateEntered with Key %q", events, key)
			}
		})
	}
}

func TestProgressScreen_ForceStopGate_ShowsBannerWhileOpen(t *testing.T) {
	s := forceStopGateScreen(t)

	if view := s.View(); !strings.Contains(view, forceStopBannerText) {
		t.Errorf("View() lacks the force-stop prompt %q while the gate is open:\n%s", forceStopBannerText, view)
	}
	if view := s.View(); strings.Contains(view, confirmPromptLabel) {
		t.Errorf("View() shows the graceful-stop prompt while only the force-stop gate is open:\n%s", view)
	}
}

func TestProgressScreen_ForceStopGate_BannerGoneOnceResolved(t *testing.T) {
	s := forceStopGateScreen(t)

	pressProgressKey(s, "n")

	if view := s.View(); strings.Contains(view, forceStopBannerText) {
		t.Errorf("View() still shows the force-stop prompt after cancel:\n%s", view)
	}
}

func TestProgressScreen_ForceStopGate_ConfirmKeysResolveAsConfirmed(t *testing.T) {
	for _, key := range []string{"y", "Y"} {
		t.Run(key, func(t *testing.T) {
			s := forceStopGateScreen(t)

			pressProgressKey(s, key)

			if s.ForceStopConfirmPending() {
				t.Errorf("gate still open after %q", key)
			}
			events := s.TakeForceStopGateEvents()
			if len(events) != 1 || events[0].Kind != ForceStopGateResolved ||
				!events[0].Confirmed || events[0].Key != key {
				t.Errorf("events = %+v, want one confirmed ForceStopGateResolved with Key %q", events, key)
			}
			if s.GracefulStop() {
				t.Error("a confirmed force stop latched the graceful stop; it must not")
			}
		})
	}
}

func TestProgressScreen_ForceStopGate_CancelKeysResolveAsCancelledWithAcknowledgement(t *testing.T) {
	cancels := map[string]func(*ProgressScreen){
		"n":   func(s *ProgressScreen) { pressProgressKey(s, "n") },
		"N":   func(s *ProgressScreen) { pressProgressKey(s, "N") },
		"esc": func(s *ProgressScreen) { s.Update(escKey()) },
	}
	for key, press := range cancels {
		t.Run(key, func(t *testing.T) {
			s := forceStopGateScreen(t)

			press(s)

			if s.ForceStopConfirmPending() {
				t.Errorf("gate still open after %q", key)
			}
			events := s.TakeForceStopGateEvents()
			if len(events) != 1 || events[0].Kind != ForceStopGateResolved ||
				events[0].Confirmed || events[0].Key != key {
				t.Errorf("events = %+v, want one unconfirmed ForceStopGateResolved with Key %q", events, key)
			}
			if view := s.View(); !strings.Contains(view, "Force stop cancelled") {
				t.Errorf("View() lacks the cancel acknowledgement:\n%s", view)
			}
		})
	}
}

func TestProgressScreen_ForceStopGate_IsStickyAgainstOtherKeys(t *testing.T) {
	for _, key := range []string{"s", "a", "i", "e", "g", "h", "m", "l", "z", "x", "1"} {
		t.Run(key, func(t *testing.T) {
			s := forceStopGateScreen(t)

			pressProgressKey(s, key)

			if !s.ForceStopConfirmPending() {
				t.Errorf("gate closed by %q; only confirm and cancel keys may resolve it", key)
			}
			if events := s.TakeForceStopGateEvents(); len(events) != 0 {
				t.Errorf("ignored key %q recorded force-stop events %+v", key, events)
			}
			if s.ConfirmPending() || s.GracefulStop() || s.ArtifactViewRequested() {
				t.Errorf("key %q acted on the screen while the force-stop gate was open", key)
			}
			if events := s.TakeRequestKeyEvents(); len(events) != 0 {
				t.Errorf("key %q reached the in-run request keys: %+v", key, events)
			}
		})
	}
}

func TestProgressScreen_ForceStopGate_ReopensAfterResolution(t *testing.T) {
	s := forceStopGateScreen(t)
	pressProgressKey(s, "n")
	s.TakeForceStopGateEvents()

	pressProgressKey(s, "x")

	if !s.ForceStopConfirmPending() {
		t.Error("gate did not reopen after a cancel; each confirmation must be a fresh request")
	}
}

func TestProgressScreen_ForceStopKey_OpensGateAfterGracefulStopArmed(t *testing.T) {
	s := armedProgressScreen()

	pressProgressKey(s, "x")

	if !s.ForceStopConfirmPending() {
		t.Error("force-stop gate did not open after a graceful stop was armed")
	}
	if !s.GracefulStop() {
		t.Error("opening the force-stop gate cleared the armed graceful stop")
	}
}

func TestProgressScreen_ForceStopKey_IgnoredWhileGracefulGateOpen(t *testing.T) {
	s := pendingProgressScreen(t)

	pressProgressKey(s, "x")

	if s.ForceStopConfirmPending() {
		t.Error("'x' opened the force-stop gate while the graceful-stop gate was open; only one gate may be open")
	}
	if !s.ConfirmPending() {
		t.Error("'x' resolved the graceful-stop gate")
	}
	if events := s.TakeForceStopGateEvents(); len(events) != 0 {
		t.Errorf("ignored 'x' recorded force-stop events %+v", events)
	}
}

func TestProgressScreen_GracefulKey_IgnoredWhileForceStopGateOpen(t *testing.T) {
	s := forceStopGateScreen(t)

	pressProgressKey(s, "s")

	if s.ConfirmPending() {
		t.Error("'s' opened the graceful-stop gate while the force-stop gate was open")
	}
	if events := s.TakeStopGateEvents(); len(events) != 0 {
		t.Errorf("ignored 's' recorded graceful gate events %+v", events)
	}
}

func TestProgressScreen_ForceStopConfirm_LeavesGracefulStopWorking(t *testing.T) {
	s := forceStopGateScreen(t)
	pressProgressKey(s, "y")

	pressProgressKey(s, "s")
	pressProgressKey(s, "y")

	if !s.GracefulStop() {
		t.Error("graceful stop could not be confirmed after a force-stop confirmation")
	}
}

func TestProgressScreen_ResetStopState_ClosesForceStopGateAndDiscardsEvents(t *testing.T) {
	s := NewProgressScreen(80, 24, progressStyles())
	pressProgressKey(s, "x")
	if !s.ForceStopConfirmPending() {
		t.Fatal("precondition: the force-stop gate must be open before the reset")
	}

	s.ResetStopState()

	if s.ForceStopConfirmPending() {
		t.Error("ResetStopState left the force-stop gate open")
	}
	if events := s.TakeForceStopGateEvents(); len(events) != 0 {
		t.Errorf("ResetStopState left undrained force-stop events %+v", events)
	}
}

func TestProgressScreen_HelpBar_ListsForceStopControl(t *testing.T) {
	s := NewProgressScreen(200, 30, progressStyles())

	if view := s.View(); !strings.Contains(view, "x force-stop") {
		t.Errorf("View() help bar lacks %q:\n%s", "x force-stop", view)
	}
}
