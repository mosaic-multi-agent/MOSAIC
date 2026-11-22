package main

// Tests that the interactive composition seam hands the no-output timeout the
// user selected in the TUI to the opencode-v2 adapter it builds. The hanging
// fake CLI and helpers are those of adapters_timeouts_test.go.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

func seamV2DepsWithNoOutput(t *testing.T, noOutput time.Duration) session.Deps {
	t.Helper()
	t.Setenv("MOSAIC_TEST_HANG_CLI", "1")
	w := buildInteractiveWiring(testWiringInput(t, session.NewStopSignal(), resolvedTestIdentity(t)))
	cfg := runconfig.ConfigSelection{
		Harness:         commonharness.HarnessIDOpenCodeV2,
		ExecutablePath:  os.Args[0],
		Timeout:         longTimeout,
		HITLTimeout:     longTimeout,
		NoOutputTimeout: noOutput,
	}
	return w.NewDeps(testRunFolder(t, "20260101T000000Z-0004"), true, "", cfg)
}

func TestInteractiveWiring_OpenCodeV2_SelectedNoOutputTimeoutEndsSilentDispatch(t *testing.T) {
	deps := seamV2DepsWithNoOutput(t, 300*time.Millisecond)

	err, ctxErr := invokeHanging(t, deps.Harness, false, guardTimeout)

	if ctxErr != nil {
		t.Fatalf("the selected no-output timeout did not end the silent dispatch; err=%v", err)
	}
	if !errors.Is(err, commonharness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", err)
	}
}

func TestInteractiveWiring_OpenCodeV2_DisabledNoOutputTimeoutLeavesDispatchRunning(t *testing.T) {
	deps := seamV2DepsWithNoOutput(t, -1) // domain.NoOutputDisabled: "0" typed in the TUI

	err, ctxErr := invokeHanging(t, deps.Harness, false, probeTimeout)

	if errors.Is(err, commonharness.ErrNoOutput) {
		t.Fatalf("a disabled watchdog ended the dispatch: %v", err)
	}
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
	}
}
