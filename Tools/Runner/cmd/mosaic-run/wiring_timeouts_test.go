package main

// wiring_timeouts_test.go verifies that the interactive composition seam hands
// both timeouts the user selected to the harness adapter it builds: the
// non-HITL value bounds dispatches without human_in_the_loop, the HITL value
// bounds human_in_the_loop dispatches. The hanging fake CLI and the helpers are
// the ones of adapters_timeouts_test.go.

import (
	"errors"
	"os"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

// seamDepsWithTimeouts builds the session.Deps the TUI seam produces for a
// claude-code selection carrying the given timeouts, with the hanging fake CLI
// as the executable.
func seamDepsWithTimeouts(t *testing.T, nonHITL, hitl time.Duration) session.Deps {
	t.Helper()
	t.Setenv("MOSAIC_TEST_HANG_CLI", "1")
	w := buildInteractiveWiring(testWiringInput(t, session.NewStopSignal(), resolvedTestIdentity(t)))
	cfg := runconfig.ConfigSelection{
		Harness:        "claude-code",
		ExecutablePath: os.Args[0],
		Timeout:        nonHITL,
		HITLTimeout:    hitl,
	}
	return w.NewDeps(testRunFolder(t, "20260101T000000Z-0003"), true, "", cfg)
}

func TestInteractiveWiring_HITLDispatch_IsBoundedBySelectedHITLTimeout(t *testing.T) {
	deps := seamDepsWithTimeouts(t, longTimeout, shortTimeout)

	err, ctxErr := invokeHanging(t, deps.Harness, true, guardTimeout)

	if ctxErr != nil {
		t.Fatalf("the HITL dispatch was not bounded by the selected HITL timeout; err=%v", err)
	}
	if !errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func TestInteractiveWiring_NonHITLDispatch_IsBoundedBySelectedNonHITLTimeout(t *testing.T) {
	deps := seamDepsWithTimeouts(t, shortTimeout, longTimeout)

	err, ctxErr := invokeHanging(t, deps.Harness, false, guardTimeout)

	if ctxErr != nil {
		t.Fatalf("the non-HITL dispatch was not bounded by the selected timeout; err=%v", err)
	}
	if !errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func TestInteractiveWiring_HITLDispatch_IsNotBoundedByTheShortNonHITLTimeout(t *testing.T) {
	deps := seamDepsWithTimeouts(t, shortTimeout, longTimeout)

	err, ctxErr := invokeHanging(t, deps.Harness, true, probeTimeout)

	if errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("the HITL dispatch hit the short non-HITL timeout: %v", err)
	}
	if ctxErr == nil {
		t.Fatalf("want the probe context to expire first, err=%v", err)
	}
}
