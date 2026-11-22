package codex_test

// Tests that the Codex adapter bounds each invocation by the timeout matching
// the HITL flag of the request (the non-HITL timeout for plain and raw
// invocations, the HITL timeout for HITL requests) and that cancelling the
// context ends the invocation with the context error.

import (
	"context"
	"errors"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/harness/codex"
)

const (
	shortTimeout = 150 * time.Millisecond
	longTimeout  = 30 * time.Second
	// ctxGuard bounds a test whose adapter timeout is expected to fire; if a
	// wrong (long) timeout applies, the guard fires first and the test fails.
	ctxGuard = 5 * time.Second
	// ctxProbe is far above shortTimeout: an invocation still running when it
	// expires was not bounded by shortTimeout.
	ctxProbe = 900 * time.Millisecond
)

// invokeHang runs the hanging fake CLI under a context with the given deadline.
func invokeHang(t *testing.T, adapter *codex.CodexAdapter, hitl, raw bool, guard time.Duration) (err, ctxErr error) {
	t.Helper()
	setHelperEnv(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), guard)
	defer cancel()
	if raw {
		_, err = adapter.InvokeRaw(ctx, agentRef(t), rawPayload)
	} else {
		req := request("test-agent#1")
		req.HumanInTheLoop = hitl
		_, err = adapter.Invoke(ctx, agentRef(t), req)
	}
	return err, ctx.Err()
}

func assertAdapterTimedOut(t *testing.T, err, ctxErr error) {
	t.Helper()
	if ctxErr != nil {
		t.Fatalf("not bounded by the expected timeout (context guard fired first); err=%v", err)
	}
	if !errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func assertNotAdapterTimeout(t *testing.T, err, ctxErr error) {
	t.Helper()
	if errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("hit the adapter timeout, but a longer timeout should apply: %v", err)
	}
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
	}
}

func TestCodexAdapter_Invoke_Timeout_ReturnsErrTimeout(t *testing.T) {
	adapter := codex.NewCodexAdapter(helperExe(t), shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestCodexAdapter_InvokeRaw_Timeout_ReturnsErrTimeout(t *testing.T) {
	adapter := codex.NewCodexAdapter(helperExe(t), shortTimeout).WithHITLTimeout(longTimeout)

	err, ctxErr := invokeHang(t, adapter, false, true, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestCodexAdapter_Invoke_HITLTrue_UsesHITLTimeout(t *testing.T) {
	adapter := codex.NewCodexAdapter(helperExe(t), longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, true, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestCodexAdapter_Invoke_HITLFalse_UsesNonHITLTimeout(t *testing.T) {
	adapter := codex.NewCodexAdapter(helperExe(t), shortTimeout).WithHITLTimeout(longTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestCodexAdapter_Invoke_HITLFalse_IgnoresShortHITLTimeout(t *testing.T) {
	adapter := codex.NewCodexAdapter(helperExe(t), longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}

func TestCodexAdapter_InvokeRaw_IgnoresShortHITLTimeout(t *testing.T) {
	adapter := codex.NewCodexAdapter(helperExe(t), longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, true, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}

func TestCodexAdapter_Invoke_HITLTrue_NonPositiveHITLTimeoutFallsBackToDefault(t *testing.T) {
	cases := map[string]func() *codex.CodexAdapter{
		"never configured": func() *codex.CodexAdapter { return codex.NewCodexAdapter(helperExe(t), shortTimeout) },
		"zero":             func() *codex.CodexAdapter { return codex.NewCodexAdapter(helperExe(t), shortTimeout).WithHITLTimeout(0) },
		"negative": func() *codex.CodexAdapter {
			return codex.NewCodexAdapter(helperExe(t), shortTimeout).WithHITLTimeout(-time.Second)
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			err, ctxErr := invokeHang(t, build(), true, false, ctxProbe)

			assertNotAdapterTimeout(t, err, ctxErr)
		})
	}
}

func TestCodexAdapter_Invoke_ZeroConstructorTimeoutIsDefault(t *testing.T) {
	adapter := codex.NewCodexAdapter(helperExe(t), 0).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}

func TestCodexAdapter_Cancellation_ReturnsTheContextError(t *testing.T) {
	for _, raw := range []bool{false, true} {
		setHelperEnv(t, "hang")
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(300*time.Millisecond, cancel)
		adapter := codex.NewCodexAdapter(helperExe(t), longTimeout)

		var err error
		if raw {
			_, err = adapter.InvokeRaw(ctx, agentRef(t), rawPayload)
		} else {
			_, err = adapter.Invoke(ctx, agentRef(t), request("test-agent#1"))
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("raw=%v: want context.Canceled, got %v", raw, err)
		}
		if errors.Is(err, commonharness.ErrTimeout) {
			t.Errorf("raw=%v: cancellation must not read as a timeout: %v", raw, err)
		}
	}
}
