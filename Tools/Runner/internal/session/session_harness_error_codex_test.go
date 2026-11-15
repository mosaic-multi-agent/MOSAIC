package session_test

// Tests that each Codex failure sentinel is recorded with the intended
// existing failure kind: stream failures as stream-error, an unreadable agent
// file as other, and the process and timeout kinds keep their precedence.

import (
	"fmt"
	"testing"

	commonharness "mosaic-common/harness"
)

// codexKindCases lists the Codex sentinel shapes and their intended kind.
// Cases marked "regression" use pre-existing sentinels or the deliberate
// default kind and are precedence/regression pins only; the stream-error
// cases are the ones that require the new Codex classification.
func codexKindCases() []kindCase {
	return []kindCase{
		{"codex stream error", fmt.Errorf("%w: quota exceeded", commonharness.ErrCodexStreamError),
			"stream-error", "quota exceeded"},
		{"codex stream incomplete", commonharness.ErrCodexStreamIncomplete,
			"stream-error", commonharness.ErrCodexStreamIncomplete.Error()},
		// regression: `other` is the deliberate kind for an unreadable agent
		// file; it is the classifier default and is pinned, not new behavior.
		{"regression codex agent unreadable", fmt.Errorf("%w: agent file changed during the run", commonharness.ErrCodexAgentUnreadable),
			"other", "agent file changed during the run"},
		{"codex non-zero exit with stream error",
			fmt.Errorf("%w: exit status 1: %w: turn failed", commonharness.ErrNonZeroExit, commonharness.ErrCodexStreamError),
			"stream-error", "turn failed"},
		{"regression codex non-zero exit without stream error", fmt.Errorf("%w: exit status 1", commonharness.ErrNonZeroExit),
			"process-failure", "exit status 1"},
		{"regression codex timeout wins over a stream error",
			fmt.Errorf("%w: %w", commonharness.ErrTimeout, commonharness.ErrCodexStreamError),
			"timeout", commonharness.ErrTimeout.Error()},
		{"regression codex empty response", fmt.Errorf("%w: no events", commonharness.ErrEmptyResponse),
			"empty-response", commonharness.ErrEmptyResponse.Error()},
	}
}

func TestSession_HarnessError_CodexSentinels_ClassifyOntoExistingKinds(t *testing.T) {
	for _, tc := range codexKindCases() {
		t.Run(tc.name, func(t *testing.T) {
			summary := firstHarnessErrorSummary(t, tc.err)

			requireSummaryNamesKind(t, summary, tc.kind, tc.detail)
		})
	}
}
