package main

// Tests that buildAdapter hands the no-output watchdog duration to the
// opencode-v2 adapter only: it ends silent non-HITL invocations (dispatch and
// raw), never HITL dispatches, and no other harness's invocations are affected.

import (
	"context"
	"errors"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
)

func v2Timeouts(noOutput time.Duration) domain.InvocationTimeouts {
	return domain.InvocationTimeouts{NonHITL: longTimeout, HITL: longTimeout, NoOutput: noOutput}
}

func TestBuildAdapter_OpenCodeV2_NoOutputWatchdogEndsSilentDispatch(t *testing.T) {
	h := hangingAdapter(t, commonharness.HarnessIDOpenCodeV2, v2Timeouts(300*time.Millisecond))

	err, ctxErr := invokeHanging(t, h, false, guardTimeout)

	if ctxErr != nil {
		t.Fatalf("the watchdog did not end the silent invocation; err=%v", err)
	}
	if !errors.Is(err, commonharness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", err)
	}
}

func TestBuildAdapter_OpenCodeV2_NoOutputWatchdogEndsSilentRawInvocation(t *testing.T) {
	h := hangingAdapter(t, commonharness.HarnessIDOpenCodeV2, v2Timeouts(300*time.Millisecond))
	raw, ok := h.(domain.RawInvoker)
	if !ok {
		t.Fatalf("%T does not implement domain.RawInvoker", h)
	}
	ctx, cancel := context.WithTimeout(context.Background(), guardTimeout)
	defer cancel()

	_, err := raw.InvokeRaw(ctx, timeoutAgentRef(t), []byte(`{}`))

	if ctx.Err() != nil {
		t.Fatalf("the watchdog did not end the silent raw invocation; err=%v", err)
	}
	if !errors.Is(err, commonharness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", err)
	}
}

func TestBuildAdapter_OpenCodeV2_NoOutputWatchdogSkipsHITLDispatch(t *testing.T) {
	h := hangingAdapter(t, commonharness.HarnessIDOpenCodeV2, v2Timeouts(300*time.Millisecond))

	err, ctxErr := invokeHanging(t, h, true, probeTimeout)

	if errors.Is(err, commonharness.ErrNoOutput) {
		t.Fatalf("a HITL dispatch must not be ended by the watchdog: %v", err)
	}
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
	}
}

func TestBuildAdapter_OpenCodeV2_NoOutputUnsetOrDisabledIsNotShort(t *testing.T) {
	// Unset (0) means the 10 minute default; disabled means no watchdog. Neither
	// ends a hang within the probe window.
	for name, d := range map[string]time.Duration{"unset": 0, "disabled": domain.NoOutputDisabled} {
		t.Run(name, func(t *testing.T) {
			h := hangingAdapter(t, commonharness.HarnessIDOpenCodeV2, v2Timeouts(d))

			err, ctxErr := invokeHanging(t, h, false, probeTimeout)

			if errors.Is(err, commonharness.ErrNoOutput) {
				t.Fatalf("no short watchdog expected: %v", err)
			}
			if !errors.Is(ctxErr, context.DeadlineExceeded) {
				t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
			}
		})
	}
}

func TestBuildAdapter_OtherHarnesses_IgnoreNoOutputTimeout(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			h := hangingAdapter(t, id, v2Timeouts(300*time.Millisecond))

			err, ctxErr := invokeHanging(t, h, false, probeTimeout)

			if errors.Is(err, commonharness.ErrNoOutput) {
				t.Fatalf("only opencode-v2 has a watchdog, got %v", err)
			}
			if !errors.Is(ctxErr, context.DeadlineExceeded) {
				t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
			}
		})
	}
}
