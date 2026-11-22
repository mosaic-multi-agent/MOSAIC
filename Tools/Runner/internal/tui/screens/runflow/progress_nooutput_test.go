package runflow

// Test that a failed infrastructure step recorded for the no-output watchdog is
// shown with its own failure kind, so it reads differently from a wall-clock
// timeout (regression coverage: the screen renders the recorded reason verbatim).

import (
	"strings"
	"testing"
)

func TestProgressScreen_FailedInfrastructureRow_NoOutputWatchdogReasonIsShownDistinctFromTimeout(t *testing.T) {
	const reason = "no-output-timeout: harness: no output within the no-output timeout: no stdout output for 10m0s"

	view := progressViewOf(160, infraRow("HARNESS_ERROR", "", reason))

	want := "commit-manager-git#2  HARNESS_ERROR  infrastructure failed: " + reason
	if !strings.Contains(view, want) {
		t.Errorf("View() lacks %q:\n%s", want, view)
	}
	if strings.Contains(view, "failed: timeout") {
		t.Errorf("the watchdog end reads like a wall-clock timeout:\n%s", view)
	}
}
