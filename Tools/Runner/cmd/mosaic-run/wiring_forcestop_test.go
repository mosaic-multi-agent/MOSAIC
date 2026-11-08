package main

// wiring_forcestop_test.go verifies that the force-stop request is shared: the
// interactive wiring hands the same instance to the TUI and to every session its
// factory builds, so a confirmed force stop in the TUI reaches the invocation
// the session is running. The non-interactive wiring has no force-stop control
// and leaves the session dependency unset.

import (
	"testing"

	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

// forceStopWiringInput returns a complete wiring input carrying a fresh shared
// force-stop request.
func forceStopWiringInput(t *testing.T) (interactiveWiringInput, *session.ForceStopSignal) {
	t.Helper()
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	force := session.NewForceStopSignal()
	in.ForceStop = force
	return in, force
}

func TestInteractiveWiring_ForceStop_TheTUIReceivesTheSameInstance(t *testing.T) {
	in, force := forceStopWiringInput(t)

	w := buildInteractiveWiring(in)

	if w.Options.ForceStop != force {
		t.Errorf("Options.ForceStop = %p, want the instance passed in (%p): the TUI would raise requests no session watches",
			w.Options.ForceStop, force)
	}
}

func TestInteractiveWiring_ForceStop_TheSessionWatchesTheTUIInstance(t *testing.T) {
	in, force := forceStopWiringInput(t)
	deps := buildInteractiveWiring(in).NewDeps(testRunFolder(t, "20260101T000000Z-0031"), true, "", runconfig.ConfigSelection{})

	if deps.ForceStop == nil {
		t.Fatal("Deps.ForceStop is nil: the session would never open a force-stop window")
	}
	wctx, release := deps.ForceStop.Watch(t.Context())
	applied := force.Request()

	if !applied {
		t.Error("Request() = false while the session's window was open: the session watches another object")
	}
	if wctx.Err() == nil {
		t.Error("the session's invocation context was not cancelled by the TUI's request")
	}
	if !release() {
		t.Error("release() = false: the session would not classify the invocation as force-stopped")
	}
}

func TestInteractiveWiring_ForceStop_EverySessionBuiltSharesTheOneObject(t *testing.T) {
	in, force := forceStopWiringInput(t)
	w := buildInteractiveWiring(in)
	first := w.NewDeps(testRunFolder(t, "20260101T000000Z-0032"), true, "", runconfig.ConfigSelection{})
	second := w.NewDeps(testRunFolder(t, "20260101T000000Z-0033"), false, "", runconfig.ConfigSelection{Harness: "claude-code"})

	if first.ForceStop != second.ForceStop {
		t.Error("two sessions built by the wiring watch different force-stop objects")
	}
	if got, ok := first.ForceStop.(*session.ForceStopSignal); !ok || got != force {
		t.Errorf("Deps.ForceStop = %v, want the shared instance %p", first.ForceStop, force)
	}
}

// TestBuildDeps_CLI_DoesNotWireForceStop pins that the non-interactive wiring
// has no force-stop control.
func TestBuildDeps_CLI_DoesNotWireForceStop(t *testing.T) {
	deps := buildDepsFromTransports(&fakeRawInvoker{}, &mainTestNoopInteraction{}, nil, nil)

	if deps.ForceStop != nil {
		t.Error("buildDeps wired ForceStop: the CLI must not offer a force-stop control")
	}
}
