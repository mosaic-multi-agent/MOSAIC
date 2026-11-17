package session_test

// Tests that every OpenCode V2 failure cause is recorded as a HARNESS_ERROR
// with a kind that matches its nature and a detail that names the cause: the
// V2 sentinels must not fall through to the generic "other" kind, and the
// causes must stay distinguishable from each other in the recorded Summary.

import (
	"fmt"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
)

// v2FailureCase is one V2 sentinel as the verdict produces it: the sentinel
// text, V2's own detail, and (for exit-code driven causes) ErrNonZeroExit.
type v2FailureCase struct {
	name     string
	sentinel error
	detail   string
	exit     bool   // the error also wraps ErrNonZeroExit, as the verdict does
	kind     string // the failure kind the Summary starts with
}

func v2FailureCases() []v2FailureCase {
	return []v2FailureCase{
		{"unknown agent", commonharness.ErrOpenCodeV2UnknownAgent, `Agent not found: "ghost"`, true, "stream-error"},
		{"unknown model", commonharness.ErrOpenCodeV2UnknownModel, "Model unavailable: openai/none", true, "stream-error"},
		{"run failed", commonharness.ErrOpenCodeV2RunFailed, "upstream exploded", true, "stream-error"},
		{"stream incomplete", commonharness.ErrOpenCodeV2StreamIncomplete, "last step finished with reason length", false, "stream-error"},
		{"usage error", commonharness.ErrOpenCodeV2UsageError, "Unknown argument: bogus-flag", true, "process-failure"},
		{"interrupted", commonharness.ErrOpenCodeV2Interrupted, "", true, "process-failure"},
		{"terminated", commonharness.ErrOpenCodeV2Terminated, "", true, "process-failure"},
	}
}

// err builds the error the way the V2 verdict reports it.
func (c v2FailureCase) err() error {
	text := ""
	if c.detail != "" {
		text = ": " + c.detail
	}
	if c.exit {
		return fmt.Errorf("%w%s (exit status 1): %w", c.sentinel, text, commonharness.ErrNonZeroExit)
	}
	return fmt.Errorf("%w%s", c.sentinel, text)
}

func TestSession_HarnessError_V2Failures_AreRecordedWithTheirKindAndNamedCause(t *testing.T) {
	for _, tc := range v2FailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			summary := firstHarnessErrorSummary(t, tc.err())

			requireSummaryNamesKind(t, summary, tc.kind, tc.sentinel.Error())
			if tc.detail != "" && !strings.Contains(summary, tc.detail) {
				t.Errorf("want V2's own message %q in the Summary, got %q", tc.detail, summary)
			}
			if strings.HasPrefix(summary, "other") {
				t.Errorf("a V2 failure must not be recorded as the generic kind, got %q", summary)
			}
		})
	}
}

func TestSession_HarnessError_V2Failures_EachCauseReadsDifferently(t *testing.T) {
	cases := v2FailureCases()
	summaries := make([]string, len(cases))
	for i, tc := range cases {
		summaries[i] = firstHarnessErrorSummary(t, tc.err())
	}

	for i, tc := range cases {
		for j, other := range cases {
			if i != j && strings.Contains(summaries[i], other.sentinel.Error()) {
				t.Errorf("the %s Summary %q also names the %s cause", tc.name, summaries[i], other.name)
			}
		}
	}
}

// The classification must not depend on the exit-code sentinel being present:
// each V2 cause keeps its kind when only the V2 sentinel is in the chain.
func TestSession_HarnessError_V2Failures_KeepTheirKindWithoutTheExitCodeSentinel(t *testing.T) {
	for _, tc := range v2FailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			bare := fmt.Errorf("invoke agent-a: %w", tc.sentinel)

			summary := firstHarnessErrorSummary(t, bare)

			requireSummaryNamesKind(t, summary, tc.kind, tc.sentinel.Error())
		})
	}
}
