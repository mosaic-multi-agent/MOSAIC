package tui

// The progress-screen key switches the anti-loop guard off and on for the
// session through the shared waiver, shows the state permanently, and logs
// every switch.

import (
	"reflect"
	"testing"
)

func TestGuardKey_SwitchesTheGuardOffAndBackOn(t *testing.T) {
	fx := newGuardFixture(t)

	pressRune(fx.m, "g")

	if !fx.waiver.Waived() {
		t.Fatal("waiver.Waived() = false after g, want true")
	}
	if !fx.m.progressScreen.GuardWaived() {
		t.Error("progress screen does not show the guard off after g")
	}
	if view := fx.m.View(); !containsStr(view, guardIndicatorLine) {
		t.Errorf("the model view lacks the guard-off indicator after g:\n%s", view)
	}

	pressRune(fx.m, "g")

	if fx.waiver.Waived() {
		t.Error("waiver.Waived() = true after the second g, want false (the key toggles)")
	}
	if fx.m.progressScreen.GuardWaived() {
		t.Error("progress screen still shows the guard off after the second g")
	}
	if view := fx.m.View(); containsStr(view, guardIndicatorLine) {
		t.Error("the guard-off indicator is still shown after the guard was switched back on")
	}
}

func TestGuardKey_EverySwitchIsLogged(t *testing.T) {
	fx := newGuardFixture(t)

	pressRune(fx.m, "g")
	pressRune(fx.m, "g")

	want := [][2]string{{"true", guardToggleTrigger}, {"false", guardToggleTrigger}}
	if got := fx.switchEvents(); !reflect.DeepEqual(got, want) {
		t.Errorf("tui.guard.switched entries (waived, trigger) = %v, want %v", got, want)
	}
}

func TestGuardKey_IgnoredWhileTheStopGateIsPending(t *testing.T) {
	fx := newGuardFixture(t)
	pressRune(fx.m, "s")
	if !fx.m.progressScreen.ConfirmPending() {
		t.Fatal("precondition: the stop gate is not pending after s")
	}

	pressRune(fx.m, "g")

	if fx.waiver.Waived() {
		t.Error("the guard was switched off while the stop gate was pending")
	}
	if n := len(fx.switchEvents()); n != 0 {
		t.Errorf("logged %d guard switches for a key the gate ignores, want 0", n)
	}
}

func TestGuardKey_IgnoredAfterAStopIsConfirmed(t *testing.T) {
	fx := newGuardFixture(t)
	pressRune(fx.m, "s")
	pressRune(fx.m, "y")
	if !fx.m.progressScreen.GracefulStop() {
		t.Fatal("precondition: the stop is not confirmed")
	}

	pressRune(fx.m, "g")

	if fx.waiver.Waived() {
		t.Error("the guard was switched off after a stop was confirmed")
	}
	if n := len(fx.switchEvents()); n != 0 {
		t.Errorf("logged %d guard switches after a confirmed stop, want 0", n)
	}
}
