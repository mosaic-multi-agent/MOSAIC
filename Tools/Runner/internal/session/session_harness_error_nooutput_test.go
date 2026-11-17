package session_test

// Tests that a no-output watchdog end is recorded as its own HARNESS_ERROR kind,
// distinct from the wall-clock timeout, while a plain timeout keeps its kind.

import (
	"fmt"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
)

// noOutputErr builds the error the watchdog reports: it wraps both sentinels.
func noOutputErr() error {
	return fmt.Errorf("%w: no stdout output for 10m0s: %w", commonharness.ErrNoOutput, commonharness.ErrTimeout)
}

func TestSession_HarnessError_NoOutputWatchdog_IsRecordedAsItsOwnKind(t *testing.T) {
	summary := firstHarnessErrorSummary(t, noOutputErr())

	requireSummaryNamesKind(t, summary, "no-output-timeout", commonharness.ErrNoOutput.Error())
	if strings.HasPrefix(summary, "timeout") {
		t.Errorf("the watchdog must not be recorded as a wall-clock timeout, got %q", summary)
	}
}

func TestSession_HarnessError_NoOutputWatchdog_KeepsKindWhenWrappedByTheInvoker(t *testing.T) {
	wrapped := fmt.Errorf("invoke agent-a: %w", noOutputErr())

	summary := firstHarnessErrorSummary(t, wrapped)

	requireSummaryNamesKind(t, summary, "no-output-timeout", commonharness.ErrNoOutput.Error())
}

func TestSession_HarnessError_WallClockTimeout_KeepsTimeoutKind(t *testing.T) {
	// regression: a timeout without the watchdog sentinel is still "timeout".
	summary := firstHarnessErrorSummary(t, fmt.Errorf("invoke agent-a: %w", commonharness.ErrTimeout))

	requireSummaryNamesKind(t, summary, "timeout", commonharness.ErrTimeout.Error())
	if strings.Contains(summary, "no-output") {
		t.Errorf("a wall-clock timeout must not name the watchdog, got %q", summary)
	}
}
