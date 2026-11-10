package claudecode_test

// Tests that ClaudeCodeAdapter bounds each invocation by the timeout matching
// the HITL flag of the request: the non-HITL timeout for HumanInTheLoop false
// and for raw invocations, the HITL timeout for HumanInTheLoop true.
// Uses the "hang" scenario of the fake-CLI helper process with short durations.

import (
	"context"
	"errors"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/claudecode"
	"mosaic-run/internal/harness/cliexec"
)

const (
	shortTimeout = 150 * time.Millisecond
	longTimeout  = 30 * time.Second
	// ctxGuard bounds a test whose adapter timeout is expected to fire; if the
	// wrong (long) timeout is applied, the guard fires first and the test fails.
	ctxGuard = 5 * time.Second
	// ctxProbe is far above shortTimeout: an invocation still running when it
	// expires was not bounded by shortTimeout.
	ctxProbe = 900 * time.Millisecond
)

func hitlRequest(hitl bool) domain.ProtocolRequest {
	req := minimalClaudeRequest("test-agent#1")
	req.HumanInTheLoop = hitl
	return req
}

// invokeHang runs the hanging helper under a context with the given deadline.
func invokeHang(t *testing.T, adapter *claudecode.ClaudeCodeAdapter, hitl bool, raw bool, guard time.Duration) (err error, ctxErr error) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("GO_HELPER_CMD", "hang")
	ctx, cancel := context.WithTimeout(context.Background(), guard)
	defer cancel()
	if raw {
		_, err = adapter.InvokeRaw(ctx, ccOrchestratorRef(t), rawPayload)
	} else {
		_, err = adapter.Invoke(ctx, ordinaryAgentRefCC(t), hitlRequest(hitl))
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
		t.Fatalf("invocation hit the adapter timeout, but a longer timeout was expected to apply: %v", err)
	}
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
	}
}

func TestClaudeCodeAdapter_Invoke_HITLTrue_UsesHITLTimeout(t *testing.T) {
	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, true, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestClaudeCodeAdapter_Invoke_HITLFalse_UsesNonHITLTimeout(t *testing.T) {
	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), shortTimeout).WithHITLTimeout(longTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestClaudeCodeAdapter_InvokeRaw_UsesNonHITLTimeout(t *testing.T) {
	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), shortTimeout).WithHITLTimeout(longTimeout)

	err, ctxErr := invokeHang(t, adapter, false, true, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
}

func TestClaudeCodeAdapter_InvokeRaw_IgnoresShortHITLTimeout(t *testing.T) {
	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, true, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}

func TestClaudeCodeAdapter_Invoke_HITLFalse_IgnoresShortHITLTimeout(t *testing.T) {
	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), longTimeout).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}

func TestClaudeCodeAdapter_Invoke_HITLTrue_UnsetOrNonPositiveHITLTimeoutFallsBackToDefault(t *testing.T) {
	cases := map[string]func() *claudecode.ClaudeCodeAdapter{
		"never configured": func() *claudecode.ClaudeCodeAdapter {
			return claudecode.NewClaudeCodeAdapter(helperExe(t), shortTimeout)
		},
		"zero": func() *claudecode.ClaudeCodeAdapter {
			return claudecode.NewClaudeCodeAdapter(helperExe(t), shortTimeout).WithHITLTimeout(0)
		},
		"negative": func() *claudecode.ClaudeCodeAdapter {
			return claudecode.NewClaudeCodeAdapter(helperExe(t), shortTimeout).WithHITLTimeout(-time.Second)
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			// The 4h default applies, not the (short) non-HITL timeout.
			err, ctxErr := invokeHang(t, build(), true, false, ctxProbe)

			assertNotAdapterTimeout(t, err, ctxErr)
		})
	}
}

func TestClaudeCodeAdapter_Invoke_HITLFalse_ZeroConstructorTimeoutIsDefault(t *testing.T) {
	adapter := claudecode.NewClaudeCodeAdapter(helperExe(t), 0).WithHITLTimeout(shortTimeout)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxProbe)

	assertNotAdapterTimeout(t, err, ctxErr)
}
