package resumecheck_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/resumecheck"
)

const noWorkflowText = "run " + fixtureRunID + " records no workflow, so there is nothing to resume it as; " +
	"its artifact may be missing, unreadable, or may record no workflow"

func asRefusal(t *testing.T, err error) *resumecheck.Refusal {
	t.Helper()
	var r *resumecheck.Refusal
	if !errors.As(err, &r) {
		t.Fatalf("error = %v (%T), want a *resumecheck.Refusal", err, err)
	}
	return r
}

func TestCheckArtifact_Readable_WithWorkflow_Passes(t *testing.T) {
	err := resumecheck.CheckArtifact(fixtureRunID, recordedState(), nil)
	if err != nil {
		t.Fatalf("CheckArtifact = %v, want nil", err)
	}
}

func TestCheckArtifact_WorkflowVersionMismatch_IsNotChecked(t *testing.T) {
	state := recordedState()
	state.WorkflowVersion = "0.0.1"
	if err := resumecheck.CheckArtifact(fixtureRunID, state, nil); err != nil {
		t.Fatalf("CheckArtifact = %v, want nil (version drift is not an early refusal)", err)
	}
}

func TestCheckArtifact_Pre30Layout_RefusesWithSessionText(t *testing.T) {
	readErr := fmt.Errorf("reading artifact: %w", artifact.ErrPre30Layout)

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	r := asRefusal(t, err)
	want := `session: refused "` + fixtureRunID + `": run ` + fixtureRunID +
		` was written in the pre-3.0 run artifact layout (Stage column or current_state.stage) and cannot be resumed`
	if r.Message != want || r.Error() != want {
		t.Errorf("refusal text = %q, want %q", r.Message, want)
	}
}

func TestCheckArtifact_Pre30CauseInsideRefusalError_UsesPre30Text(t *testing.T) {
	readErr := &domain.RefusalError{Component: "artifact", Resource: "x", Reason: "old layout", Cause: artifact.ErrPre30Layout}

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	r := asRefusal(t, err)
	if r.Message == readErr.Error() {
		t.Errorf("refusal text = %q, want the pre-3.0 layout text rather than the wrapped error's text", r.Message)
	}
}

func TestCheckArtifact_RefusalError_RefusesWithItsTextAndCause(t *testing.T) {
	readErr := &domain.RefusalError{Component: "artifact", Resource: "Orchestration.md", Reason: "invalid 'review_loop_limit' value"}

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, fmt.Errorf("read: %w", readErr))

	r := asRefusal(t, err)
	if r.Message != readErr.Error() {
		t.Errorf("refusal text = %q, want %q", r.Message, readErr.Error())
	}
	if !errors.Is(r, readErr) {
		t.Error("refusal does not unwrap to the underlying refusal error")
	}
}

func TestCheckArtifact_MissingArtifact_RefusesAsNoWorkflow(t *testing.T) {
	readErr := fmt.Errorf("open Orchestration.md: %w", os.ErrNotExist)

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	r := asRefusal(t, err)
	if r.Message != noWorkflowText {
		t.Errorf("refusal text = %q, want %q", r.Message, noWorkflowText)
	}
}

func TestCheckArtifact_UnreadableArtifact_ReturnsReadErrorUnchanged(t *testing.T) {
	readErr := errors.New("permission denied")

	err := resumecheck.CheckArtifact(fixtureRunID, domain.ArtifactState{}, readErr)

	if err != readErr {
		t.Fatalf("CheckArtifact = %v, want the read error itself", err)
	}
	var r *resumecheck.Refusal
	if errors.As(err, &r) {
		t.Error("an unreadable artifact must not be reported as a refusal")
	}
}

func TestCheckArtifact_RecordsNoWorkflow_Refuses(t *testing.T) {
	state := recordedState()
	state.Workflow = ""

	err := resumecheck.CheckArtifact(fixtureRunID, state, nil)

	r := asRefusal(t, err)
	if r.Message != noWorkflowText {
		t.Errorf("refusal text = %q, want %q", r.Message, noWorkflowText)
	}
}
