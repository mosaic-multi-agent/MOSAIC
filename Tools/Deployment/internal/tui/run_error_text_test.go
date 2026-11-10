package tui

// run_error_text_test.go verifies that the result screen shows a service error's full text
// unchanged, so the TUI and the CLI report the same content for resolution failures (searched
// catalogs, removed-layer hint).

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRunError_ResolutionErrorText_IsShownUnchanged(t *testing.T) {
	cases := map[string]error{
		"unknown workflow with removed-layer hint": fmt.Errorf("%w; it may have come from a catalog layer that is no longer in the stack",
			errors.New(`workflow not found: "team-review"; searched catalogs: team-extras, mosaic-default`)),
		"unknown requested agent": errors.New(`requested subagent "team-linter" not found; searched catalogs: team-extras, mosaic-default`),
	}
	for name, runErr := range cases {
		t.Run(name, func(t *testing.T) {
			m := newRestartModel()

			m.Update(runErrorMsg{err: runErr})

			if m.screen != screenDone {
				t.Fatalf("screen = %v, want screenDone", m.screen)
			}
			if m.errMsg != runErr.Error() {
				t.Errorf("errMsg = %q, want the service error text %q", m.errMsg, runErr.Error())
			}
			if !strings.Contains(m.View(), "Deployment failed") {
				t.Errorf("result screen does not report the failure: %q", m.View())
			}
		})
	}
}
