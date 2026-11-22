package opencodev2_test

// Tests for the no-output watchdog of the adapter: it is enabled with the
// configured duration for ordinary dispatches and consultations, never for
// human-in-the-loop dispatches, disabled when unset or not positive, and a
// watchdog end reports one activity item naming the watchdog.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/testutil"
)

const watchdogDuration = 300 * time.Millisecond

func assertWatchdogEnded(t *testing.T, err, ctxErr error) {
	t.Helper()
	if ctxErr != nil {
		t.Fatalf("the watchdog did not end the invocation before the context guard; err=%v", err)
	}
	if !errors.Is(err, commonharness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", err)
	}
	if !errors.Is(err, cliexec.ErrTimeout) {
		t.Errorf("want the watchdog error to also satisfy ErrTimeout, got %v", err)
	}
}

func assertNoWatchdog(t *testing.T, err, ctxErr error) {
	t.Helper()
	if errors.Is(err, commonharness.ErrNoOutput) {
		t.Fatalf("no watchdog was expected here: %v", err)
	}
	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("want the probe context to expire first, ctx.Err()=%v err=%v", ctxErr, err)
	}
}

func TestInvoke_NoOutputTimeout_EndsSilentInvocation(t *testing.T) {
	adapter := newAdapter(t, longTimeout).WithHITLTimeout(longTimeout).WithNoOutputTimeout(watchdogDuration)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxGuard)

	assertWatchdogEnded(t, err, ctxErr)
}

func TestInvokeRaw_NoOutputTimeout_EndsSilentInvocation(t *testing.T) {
	adapter := newAdapter(t, longTimeout).WithNoOutputTimeout(watchdogDuration)

	err, ctxErr := invokeHang(t, adapter, false, true, ctxGuard)

	assertWatchdogEnded(t, err, ctxErr)
}

func TestInvoke_NoOutputTimeout_EndsInvocationThatWentSilentAfterOutput(t *testing.T) {
	adapter := newAdapter(t, longTimeout).WithNoOutputTimeout(watchdogDuration)
	setHelperEnv(t, "start-hang")
	ctx, cancel := context.WithTimeout(context.Background(), ctxGuard)
	defer cancel()

	_, err := adapter.Invoke(ctx, ordinaryAgentRef(), minimalRequest("test-agent#1"))

	assertWatchdogEnded(t, err, ctx.Err())
}

func TestInvoke_NoOutputTimeout_DoesNotApplyToHITLDispatches(t *testing.T) {
	adapter := newAdapter(t, longTimeout).WithHITLTimeout(longTimeout).WithNoOutputTimeout(watchdogDuration)

	err, ctxErr := invokeHang(t, adapter, true, false, ctxProbe)

	assertNoWatchdog(t, err, ctxErr)
}

func TestInvoke_NoOutputTimeout_UnsetZeroOrNegativeDisablesWatchdog(t *testing.T) {
	for name, d := range map[string]time.Duration{"zero": 0, "negative": -time.Second} {
		t.Run(name, func(t *testing.T) {
			adapter := newAdapter(t, longTimeout).WithNoOutputTimeout(d)

			err, ctxErr := invokeHang(t, adapter, false, false, ctxProbe)

			assertNoWatchdog(t, err, ctxErr)
		})
	}
	t.Run("never configured", func(t *testing.T) {
		err, ctxErr := invokeHang(t, newAdapter(t, longTimeout), false, false, ctxProbe)

		assertNoWatchdog(t, err, ctxErr)
	})
}

func TestInvoke_WallClockTimeoutFirst_IsNotReportedAsWatchdog(t *testing.T) {
	adapter := newAdapter(t, shortTimeout).WithNoOutputTimeout(time.Hour)

	err, ctxErr := invokeHang(t, adapter, false, false, ctxGuard)

	assertAdapterTimedOut(t, err, ctxErr)
	if errors.Is(err, commonharness.ErrNoOutput) {
		t.Errorf("the wall-clock timeout came first; want no ErrNoOutput, got %v", err)
	}
}

func TestInvoke_NoOutputEnd_ReportsOneWatchdogActivityItemBeforeEnding(t *testing.T) {
	obs := &testutil.Observer{}
	adapter := newActivityAdapter(t, obs).WithNoOutputTimeout(watchdogDuration)
	setHelperEnv(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), ctxGuard)
	defer cancel()

	_, err := adapter.Invoke(ctx, ordinaryAgentRef(), minimalRequest("test-agent#1"))

	assertWatchdogEnded(t, err, ctx.Err())
	_, items, events, ends := obs.Snapshot()
	testutil.AssertLifecycle(t, events, ends)
	var texts []domain.ActivityItem
	for _, it := range items {
		if it.Kind == domain.ActivityText {
			texts = append(texts, it)
		}
	}
	if len(texts) != 1 {
		t.Fatalf("want exactly one text item naming the watchdog, got %+v", items)
	}
	if s := strings.ToLower(texts[0].Summary); !strings.Contains(s, "no output") || !strings.Contains(s, "watchdog") {
		t.Errorf("want the item to name the no-output watchdog, got %q", texts[0].Summary)
	}
	if events[len(events)-1] != "end" {
		t.Errorf("want the watchdog item reported before the invocation ends, got events %v", events)
	}
}

func TestInvoke_WallClockTimeout_ReportsNoWatchdogActivityItem(t *testing.T) {
	obs := &testutil.Observer{}
	adapter := newAdapter(t, shortTimeout).WithActivityObserver(obs).WithNoOutputTimeout(time.Hour)
	setHelperEnv(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), ctxGuard)
	defer cancel()

	_, err := adapter.Invoke(ctx, ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if !errors.Is(err, cliexec.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	_, items, _, _ := obs.Snapshot()
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Summary), "watchdog") {
			t.Errorf("a wall-clock timeout must not report the watchdog, got %+v", it)
		}
	}
}
