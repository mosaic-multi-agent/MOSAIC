package session_test

// Tests for the failure kind a harness error is recorded with: every invocation
// that ends without a usable subagent response becomes a HARNESS_ERROR row
// whose Summary names the failure kind and its detail, on the engine-routed
// path (auto modes) and on the consultant-routed path (orchestrated mode).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// kindCase is one harness failure and the Summary it must be recorded with.
type kindCase struct {
	name   string
	err    error
	kind   string // the failure kind the Summary starts with
	detail string // a fragment the Summary must carry
}

// claimedStatusText is what a malformed response quotes of the agent's output.
// It must never reach a Summary.
const claimedStatusText = `{"status_code":"SUCCESS","status_message":"all good and accepted"`

func kindCases() []kindCase {
	return []kindCase{
		{"timeout", fmt.Errorf("invoke agent-a: %w", commonharness.ErrTimeout),
			"timeout", commonharness.ErrTimeout.Error()},
		{"non-zero exit", fmt.Errorf("%w: exit status 3", commonharness.ErrNonZeroExit),
			"process-failure", "exit status 3"},
		{"launch failure", &domain.HarnessLaunchError{
			Harness: "claude-code", Executable: "claude", Err: commonharness.ErrExecutableNotFound},
			"process-failure", "could not launch"},
		{"malformed JSON", fmt.Errorf("%w: %s", commonharness.ErrMalformedJSON, claimedStatusText),
			"unparseable-response", "not valid protocol JSON"},
		{"protocol not extractable", fmt.Errorf("%w: %s", commonharness.ErrProtocolNotExtractable, claimedStatusText),
			"unparseable-response", "not valid protocol JSON"},
		{"empty response", fmt.Errorf("agent output: %w", commonharness.ErrEmptyResponse),
			"empty-response", commonharness.ErrEmptyResponse.Error()},
		{"ghcp agent deselected", fmt.Errorf("%w (stderr: agent not found; exit code 1)", commonharness.ErrGHCPCLIAgentDeselected),
			"ghcp-agent-deselected", "agent not found"},
		{"ghcp session error", fmt.Errorf("%w: model unavailable", commonharness.ErrGHCPCLISessionError),
			"ghcp-session-error", "model unavailable"},
		{"ghcp stream error", fmt.Errorf("%w: rate limited", commonharness.ErrGHCPCLIStreamError),
			"stream-error", "rate limited"},
		{"ghcp stream incomplete", commonharness.ErrGHCPCLIStreamIncomplete,
			"stream-error", commonharness.ErrGHCPCLIStreamIncomplete.Error()},
		{"opencode stream error", fmt.Errorf("%w: provider down", commonharness.ErrOpenCodeStreamError),
			"stream-error", "provider down"},
		{"opencode stream incomplete", commonharness.ErrOpenCodeStreamIncomplete,
			"stream-error", commonharness.ErrOpenCodeStreamIncomplete.Error()},
		{"unclassified", errors.New("something unexpected broke"),
			"other", "something unexpected broke"},
	}
}

// newNoConsultantAutoSession builds an auto-mode session on the linear fixture
// without a routing consultant, so the first harness error ends the run as an
// unresolved deviation after its row is recorded.
func newNoConsultantAutoSession(t *testing.T) (session.Session, *harness.MockAdapter, *memStore, string) {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store := harness.NewMockAdapter(), &memStore{}
	ses := session.New(session.Deps{Harness: f, Store: store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})
	return ses, f, store, orchPath
}

// firstHarnessErrorSummary runs a session whose agent-a fails with err and
// returns the Summary of the first recorded HARNESS_ERROR row.
func firstHarnessErrorSummary(t *testing.T, err error) string {
	t.Helper()
	ses, f, store, orchPath := newNoConsultantAutoSession(t)
	f.Queue("agent-a", harness.ScriptedEntry{Err: err})

	got, startErr := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, startErr, domain.RunDeviationUnresolved)
	row := requireHarnessFailureStep(t, store, "engine path")
	return row.Summary
}

func TestSession_HarnessError_EnginePath_SummaryNamesFailureKindAndDetail(t *testing.T) {
	for _, tc := range kindCases() {
		t.Run(tc.name, func(t *testing.T) {
			summary := firstHarnessErrorSummary(t, tc.err)

			requireSummaryNamesKind(t, summary, tc.kind, tc.detail)
			if strings.ContainsAny(summary, "\r\n") {
				t.Errorf("want a single-line Summary, got %q", summary)
			}
			if strings.Contains(summary, "[error:") {
				t.Errorf("want no error-code marker on a HARNESS_ERROR Summary, got %q", summary)
			}
		})
	}
}

func TestSession_HarnessError_EnginePath_EveryKindHasADistinguishableSummary(t *testing.T) {
	seen := map[string]string{}
	for _, tc := range kindCases() {
		summary := firstHarnessErrorSummary(t, tc.err)
		kind := strings.SplitN(summary, ":", 2)[0]
		if prev, dup := seen[kind]; dup && prev != tc.kind {
			t.Errorf("want distinct kinds to read differently, but %q (%s) and %q share the prefix %q",
				tc.name, tc.kind, prev, kind)
		}
		seen[kind] = tc.kind
	}
	if len(seen) < 7 {
		t.Errorf("want at least 7 distinguishable Summary prefixes (timeout, process failure, unparseable, "+
			"empty, ghcp deselected, ghcp session error, stream error), got %d: %v", len(seen), seen)
	}
}

func TestSession_HarnessError_MultiLineDetail_IsFlattenedToOneLine(t *testing.T) {
	err := fmt.Errorf("%w: exit status 2\nstderr line one\r\nstderr line two", commonharness.ErrNonZeroExit)

	summary := firstHarnessErrorSummary(t, err)

	requireSummaryNamesKind(t, summary, "process-failure", "stderr line two")
	if strings.ContainsAny(summary, "\r\n") {
		t.Errorf("want every line break replaced, got %q", summary)
	}
}

// The agent's own words never appear as if accepted: the Summary says a
// response was received but was not valid protocol JSON, and does not quote
// the status the agent claimed.
func TestSession_HarnessError_UnparseableResponse_DoesNotQuoteTheClaimedStatus(t *testing.T) {
	for _, sentinel := range []error{commonharness.ErrMalformedJSON, commonharness.ErrProtocolNotExtractable} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			err := fmt.Errorf("%w: %s", sentinel, claimedStatusText)

			summary := firstHarnessErrorSummary(t, err)

			if !strings.Contains(summary, "received") || !strings.Contains(summary, "not valid protocol JSON") {
				t.Errorf("want the Summary to say the response was received but was not valid protocol JSON, got %q", summary)
			}
			for _, quoted := range []string{"SUCCESS", "all good and accepted", "status_code"} {
				if strings.Contains(summary, quoted) {
					t.Errorf("want the agent's claimed status kept out of the Summary, but %q appears in %q", quoted, summary)
				}
			}
			if !strings.Contains(summary, sentinel.Error()) {
				t.Errorf("want the Summary to name the sentinel %q, got %q", sentinel.Error(), summary)
			}
		})
	}
}

// The consultant-routed path records the same kind Summary and hands it to the
// next consultation as both the status message and the error reason.
func TestSession_HarnessError_ConsultPath_SummaryNamesFailureKindAndReachesTheConsultation(t *testing.T) {
	for _, tc := range kindCases() {
		t.Run(tc.name, func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueDispatch("agent-a", "try the work", 0)
			consultant.queueStop("done")
			ses, f, store, orchPath := newOrchestratedSession(t, consultant)
			// A raw-text-eligible error is retried once by the bypass; the
			// bypass fails the same way.
			queueRepeated(f, "agent-a", 2, harness.ScriptedEntry{Err: tc.err})

			ses.Start(context.Background(), baseOrchestratedConfig(orchPath)) //nolint:errcheck

			row := requireHarnessFailureStep(t, store, "consultant path")
			requireSummaryNamesKind(t, row.Summary, tc.kind, tc.detail)
			if len(consultant.Requests) < 2 {
				t.Fatalf("want a consultation after the harness error, got %d requests", len(consultant.Requests))
			}
			last := consultant.Requests[len(consultant.Requests)-1]
			if last.LastStatusMessage == nil || *last.LastStatusMessage != row.Summary {
				t.Errorf("want last_status_message equal to the row Summary %q, got %v", row.Summary, last.LastStatusMessage)
			}
			requireErrorReasonCarries(t, last, tc.detail)
			requireHarnessErrorDeviationIfPresent(t, last)
		})
	}
}

// requireHarnessErrorDeviationIfPresent checks the deviation of a consultation
// that carries one: a harness-error consultation reports a HARNESS_ERROR
// response, never a BLOCKED one.
func requireHarnessErrorDeviationIfPresent(t *testing.T, req domain.ConsultationRequest) {
	t.Helper()
	if req.Deviation == nil {
		return
	}
	if req.Deviation.Response.StatusCode == domain.StatusBLOCKED {
		t.Errorf("want no BLOCKED response for a harness failure, got BLOCKED/%s", req.Deviation.Response.ErrorCode)
	}
}
