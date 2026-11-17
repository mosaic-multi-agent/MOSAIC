package opencodev2_test

// Tests that each invocation is bounded by the timeout matching its kind: the
// non-HITL timeout for ordinary dispatches and for orchestrator consultations,
// the HITL timeout only for dispatches whose request has HumanInTheLoop set.
// Uses the "hang" scenario with short durations.

import (
	"context"
	"errors"
	"testing"
	"time"

	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/harness/opencodev2"
)

const (
	shortTimeout = 150 * time.Millisecond
	longTimeout  = 30 * time.Second
	// ctxGuard bounds a test whose adapter timeout is expected to fire; if the
	// wrong (long) timeout applies, the guard fires first and the test fails.
	ctxGuard = 5 * time.Second
	// ctxProbe is far above shortTimeout: an invocation still running when it
	// expires was not bounded by shortTimeout.
	ctxProbe = 900 * time.Millisecond
)

// invokeHang runs the hanging helper under a context with the given deadline.
func invokeHang(t *testing.T, adapter *opencodev2.OpenCodeV2Adapter, hitl, raw bool, guard time.Duration) (err, ctxErr error) {
	t.Helper()
	setHelperEnv(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), guard)
	defer cancel()
	if raw {
		_, err = adapter.InvokeRaw(ctx, orchestratorRef(), rawPayload)
	} else {
		req := minimalRequest("test-agent#1")
		req.HumanInTheLoop = hitl
		_, err = adapter.Invoke(ctx, ordinaryAgentRef(), req)
	}
	return err, ctx.Err()
}

func assertAdapterTimedOut(t *testing.T, err, ctxErr error) {
	t.Helper()
	if ctxErr != nil {
		t.Fatalf("the invocation was not bounded by the expected timeout (context guard fired first); err=%v", err)
	}
	if !errors.Is(err, cliexec.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func assertNotAdapterTimeout(t *testing.T, err, ctxErr error) {
	t.Helper()
	if errors.Is(err, cliexec.ErrTimeout) {
		t.Fatalf("invocation hit the adapter timeout, but a longer timeout was expected: %v", err)
	}
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
	}
}

func TestInvoke_HITLTrue_UsesHITLTimeout(t *testing.T) {
	adapter := newAdapter(t, longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, true, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestInvoke_HITLFalse_UsesNonHITLTimeout(t *testing.T) {
	adapter := newAdapter(t, shortTimeout).WithHITLTimeout(longTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestInvokeRaw_UsesNonHITLTimeout(t *testing.T) {
	adapter := newAdapter(t, shortTimeout).WithHITLTimeout(longTimeout)

	err, ctxErr := invokeHang(t, adapter, false, true, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestInvokeRaw_IgnoresShortHITLTimeout(t *testing.T) {
	adapter := newAdapter(t, longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, true, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}

func TestInvoke_HITLFalse_IgnoresShortHITLTimeout(t *testing.T) {
	adapter := newAdapter(t, longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}

func TestInvoke_HITLTrue_UnsetOrNonPositiveHITLTimeoutFallsBackToDefault(t *testing.T) {
	cases := map[string]func() *opencodev2.OpenCodeV2Adapter{
		"never configured": func() *opencodev2.OpenCodeV2Adapter { return newAdapter(t, shortTimeout) },
		"zero":             func() *opencodev2.OpenCodeV2Adapter { return newAdapter(t, shortTimeout).WithHITLTimeout(0) },
		"negative":         func() *opencodev2.OpenCodeV2Adapter { return newAdapter(t, shortTimeout).WithHITLTimeout(-time.Second) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			// The 4h default applies, not the (short) non-HITL timeout.
			err, ctxErr := invokeHang(t, build(), true, false, ctxProbe)

			assertNotAdapterTimeout(t, err, ctxErr)
		})
	}
}

func TestInvoke_HITLFalse_ZeroConstructorTimeoutIsTheDefault(t *testing.T) {
	adapter := newAdapter(t, 0).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}
