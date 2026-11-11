package main

// Tests that buildAdapter applies both invocation timeouts to every CLI
// adapter: the non-HITL value for dispatches without human_in_the_loop and for
// raw invocations, the HITL value for human_in_the_loop dispatches.
//
// The test binary doubles as the hanging fake CLI: init() blocks forever when
// MOSAIC_TEST_HANG_CLI=1 is set, before any test machinery starts.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
)

func init() {
	if os.Getenv("MOSAIC_TEST_HANG_CLI") == "1" {
		time.Sleep(24 * time.Hour)
		os.Exit(0)
	}
}

// nonHITLTimeouts builds InvocationTimeouts carrying only the non-HITL value,
// for tests that do not care about the HITL timeout.
func nonHITLTimeouts(d time.Duration) domain.InvocationTimeouts {
	return domain.InvocationTimeouts{NonHITL: d}
}

const (
	shortTimeout = 150 * time.Millisecond
	longTimeout  = 30 * time.Second
	guardTimeout = 5 * time.Second
	probeTimeout = 900 * time.Millisecond
)

var cliHarnessIDs = []string{
	commonharness.HarnessIDClaudeCode,
	commonharness.HarnessIDOpenCode,
	commonharness.HarnessIDGHCPCLI,
}

func hangingAdapter(t *testing.T, harnessID string, timeouts domain.InvocationTimeouts) domain.HarnessAdapter {
	t.Helper()
	t.Setenv("MOSAIC_TEST_HANG_CLI", "1")
	return buildAdapter("", harnessID, os.Args[0], "blanket", timeouts, nil)
}

func timeoutAgentRef(t *testing.T) domain.AgentReference {
	t.Helper()
	def := filepath.Join(t.TempDir(), "agent.md")
	content := "---\nname: test-agent\nmodel: m\ntools: Read\n---\n\nBody.\n"
	if err := os.WriteFile(def, []byte(content), 0o644); err != nil {
		t.Fatalf("write agent definition: %v", err)
	}
	return domain.AgentReference{Identifier: "test-agent", DefinitionPath: def}
}

func invokeHanging(t *testing.T, h domain.HarnessAdapter, hitl bool, guard time.Duration) (err, ctxErr error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), guard)
	defer cancel()
	req := domain.ProtocolRequest{AgentInstanceID: "test-agent#1", TaskDescription: "x", HumanInTheLoop: hitl}
	_, err = h.Invoke(ctx, timeoutAgentRef(t), req)
	return err, ctx.Err()
}

func TestBuildAdapter_HITLDispatch_IsBoundedByHITLTimeout(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			h := hangingAdapter(t, id, domain.InvocationTimeouts{NonHITL: longTimeout, HITL: shortTimeout})

			err, ctxErr := invokeHanging(t, h, true, guardTimeout)

			if ctxErr != nil {
				t.Fatalf("HITL dispatch was not bounded by the HITL timeout; err=%v", err)
			}
			if !errors.Is(err, commonharness.ErrTimeout) {
				t.Fatalf("want ErrTimeout, got %v", err)
			}
		})
	}
}

func TestBuildAdapter_NonHITLDispatch_IsBoundedByNonHITLTimeout(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			h := hangingAdapter(t, id, domain.InvocationTimeouts{NonHITL: shortTimeout, HITL: longTimeout})

			err, ctxErr := invokeHanging(t, h, false, guardTimeout)

			if ctxErr != nil {
				t.Fatalf("non-HITL dispatch was not bounded by the non-HITL timeout; err=%v", err)
			}
			if !errors.Is(err, commonharness.ErrTimeout) {
				t.Fatalf("want ErrTimeout, got %v", err)
			}
		})
	}
}

func TestBuildAdapter_RawInvocation_UsesNonHITLTimeout(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			h := hangingAdapter(t, id, domain.InvocationTimeouts{NonHITL: shortTimeout, HITL: longTimeout})
			raw, ok := h.(domain.RawInvoker)
			if !ok {
				t.Fatalf("%T does not implement domain.RawInvoker", h)
			}
			ctx, cancel := context.WithTimeout(context.Background(), guardTimeout)
			defer cancel()

			_, err := raw.InvokeRaw(ctx, timeoutAgentRef(t), []byte(`{}`))

			if ctx.Err() != nil {
				t.Fatalf("raw invocation was not bounded by the non-HITL timeout; err=%v", err)
			}
			if !errors.Is(err, commonharness.ErrTimeout) {
				t.Fatalf("want ErrTimeout, got %v", err)
			}
		})
	}
}

func TestBuildAdapter_UnsetHITLTimeout_FallsBackToHITLDefault(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			// HITL is zero: the 4h default applies, not the short non-HITL value.
			h := hangingAdapter(t, id, domain.InvocationTimeouts{NonHITL: shortTimeout})

			err, ctxErr := invokeHanging(t, h, true, probeTimeout)

			if errors.Is(err, commonharness.ErrTimeout) {
				t.Fatalf("HITL dispatch hit a short timeout although the HITL timeout was unset: %v", err)
			}
			if !errors.Is(ctxErr, context.DeadlineExceeded) {
				t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
			}
		})
	}
}
