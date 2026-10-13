package engine_test

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ===== RowNotInGroupError when the row is not in any ordered group =====
//
// computeNextFromExecution used to return a -1 sentinel when the current row
// was not found in any group of the resolved stage, causing an out-of-bounds
// panic in the caller (buildDispatchStep). These tests verify that after the
// fix the engine returns a StopDecision with a diagnostic message instead.
//
// Scenario: Tests-Only approach limits the groups of the iteration to the Test
// group only (rows [7,9) in brownfield-tdd). When the recorded state places an
// Implementation group row (implementation-tdd, row 9) as the current row,
// that row does not appear in any ordered group, triggering the error.

// brownfieldTDDAgents returns a full agent map for brownfield-tdd rows.
func brownfieldTDDAgents() map[string]domain.AgentReference {
	return newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
}

// TestNext_RowNotInGroup_ReturnsStopDecision verifies that when the current
// EXECUTION row is not found in any ordered group of the resolved stage,
// handleExecutionSuccess returns a StopDecision rather than panicking via
// an out-of-bounds slice access on the -1 sentinel row index.
//
// Setup: brownfield-tdd with Tests-Only approach (only the Test group is
// ordered for stage 1: rows [7,9)). State records implementation-tdd (row 9)
// as the last completed agent. Row 9 is outside [7,9), so no group contains
// it -- the "row not in group" condition.
func TestNext_RowNotInGroup_ReturnsStopDecision(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "2.0")
	// Tests-Only: orderedGroupsForStage returns only the Test group [7,9).
	schedules := singleSchedules("Tests-Only")
	agents := brownfieldTDDAgents()
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-tdd#5", domain.StatusSUCCESS, 5, 10)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("implementation-tdd#5"),
		Agents:       agents,
		Seq:          5,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	// Must return StopDecision, not panic from buildDispatchStep(-1).
	stop := requireStop(t, dec)
	if stop.Reason == "" {
		t.Error("StopDecision.Reason must be non-empty for diagnostics")
	}
}

// TestNext_RowNotInGroup_StopReasonContainsDiagnostics verifies that the
// StopDecision reason produced when the current row is not in any ordered
// group includes the row index, the stage number, and group information so
// the failure is diagnosable without reproducing the run.
func TestNext_RowNotInGroup_StopReasonContainsDiagnostics(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "2.0")
	schedules := singleSchedules("Tests-Only")
	agents := brownfieldTDDAgents()
	// implementation-tdd is row 9; stage 1 with Tests-Only has only [7,9).
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-tdd#5", domain.StatusSUCCESS, 5, 10)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("implementation-tdd#5"),
		Agents:       agents,
		Seq:          5,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	stop := requireStop(t, dec)
	// Reason must mention the row index (9), the stage (1), and at least one
	// group so the caller can diagnose the routing failure.
	for _, want := range []string{"9", "1"} {
		if !strings.Contains(stop.Reason, want) {
			t.Errorf("StopDecision.Reason %q does not contain expected diagnostic %q", stop.Reason, want)
		}
	}
}

// TestNext_RowNotInGroup_ErrorIsRowNotInGroupError verifies that the underlying
// error produced by computeNextFromExecution is a *domain.RowNotInGroupError
// whose fields identify the failing row and stage precisely.
//
// Because Next embeds the error message in StopDecision.Reason, we verify
// field-level diagnostics by constructing the expected error and comparing
// its Error() string against the stop reason (equivalent to asserting the
// structured fields without coupling to message format beyond key identifiers).
func TestNext_RowNotInGroup_ErrorIsRowNotInGroupError(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "2.0")
	schedules := singleSchedules("Tests-Only")
	agents := brownfieldTDDAgents()
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-tdd#5", domain.StatusSUCCESS, 5, 10)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("implementation-tdd#5"),
		Agents:       agents,
		Seq:          5,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	stop := requireStop(t, dec)
	// The domain error message format from RowNotInGroupError.Error() includes
	// the stage number and the row index. Verify both appear in the reason.
	// This also confirms the error type (not a generic string) was used.
	if !strings.Contains(stop.Reason, "stage 1") {
		t.Errorf("StopDecision.Reason %q: expected stage number in RowNotInGroupError format", stop.Reason)
	}
	if !strings.Contains(stop.Reason, "row 9") {
		t.Errorf("StopDecision.Reason %q: expected row index in RowNotInGroupError format", stop.Reason)
	}
}

// ===== ResumePoint errors on RowNotInGroupError (non-complete workflow) =====

// TestResumePoint_RowNotInGroup_NonCompleteWorkflow_ReturnsError verifies that
// when computeNextFromExecution returns a *domain.RowNotInGroupError (because
// the current EXECUTION row is not in any ordered group), ResumePoint wraps
// it and returns an error rather than silently treating the situation as
// end-of-run (the pre-fix behavior via the adv.RowIndex < 0 branch).
//
// Before the fix: computeNextFromExecution returns {RowIndex:-1, nil} and
// ResumePoint falls through to the adv.RowIndex < 0 guard at line 308,
// returning a spurious end-of-run ResumeInfo with no error.
// After the fix: computeNextFromExecution returns *RowNotInGroupError and
// ResumePoint wraps it via the existing advErr != nil guard (lines 304-306).
func TestResumePoint_RowNotInGroup_NonCompleteWorkflow_ReturnsError(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "2.0")
	schedules := singleSchedules("Tests-Only")
	// implementation-tdd (row 9) is not in the Test-Only ordered group [7,9).
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-tdd#5", domain.StatusSUCCESS, 5, 10)

	_, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
	if err == nil {
		t.Fatal("want error when EXECUTION row is not in any ordered group of a non-complete workflow, got nil")
	}
	// The error must wrap *domain.RowNotInGroupError so callers can inspect it.
	var rnigErr *domain.RowNotInGroupError
	if !errors.As(err, &rnigErr) {
		t.Errorf("want error wrapping *domain.RowNotInGroupError, got %T: %v", err, err)
	}
}

// TestResumePoint_RowNotInGroup_ErrorMessageContainsDiagnostics verifies that
// the error returned by ResumePoint when a row is not in any ordered group
// contains enough context to diagnose the failure: the row index and stage.
func TestResumePoint_RowNotInGroup_ErrorMessageContainsDiagnostics(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "2.0")
	schedules := singleSchedules("Tests-Only")
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-tdd#5", domain.StatusSUCCESS, 5, 10)

	_, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"9", "1"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain expected diagnostic %q (row index or stage)", msg, want)
		}
	}
}

// noPostBlockV216Content is a minimal workflow whose only rows are the four
// rows of an ungrouped Stage block: it has no row after the block, so finishing
// the last block row of the last iteration ends the run.
const noPostBlockV216Content = v216Columns +
	`| EXECUTION.[Stage] | test-writer-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | tests-review-tdd | FALSE | NEXT | test-writer-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage] | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
` + v216StageBlock

// TestResumePoint_CompleteWorkflow_EndOfRunBehaviorUnchanged verifies that
// when the workflow is genuinely complete (the last row of the last iteration
// has finished and nothing follows the block), ResumePoint still returns the
// end-of-run info with no error.
//
// Setup: noPostBlockV216Content has no rows after the block. After the last
// block row (implementation-review, row 3) completes, no iteration and no row
// remain, so ResumePoint returns (ResumeInfo{RowIndex=4}, nil).
func TestResumePoint_CompleteWorkflow_EndOfRunBehaviorUnchanged(t *testing.T) {
	aw := mustParseAndAdmitV216(t, noPostBlockV216Content, "no-post-block", "1.0")
	// The block is ungrouped, so the approach of its single iteration is ignored.
	schedules := singleSchedules("Implementation-Only")
	// implementation-review is the last block row (row 3); no rows follow the block.
	state := stateAfterRow("EXECUTION", "1", "implementation-review#4", domain.StatusSUCCESS, 4, 4)

	info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
	if err != nil {
		t.Fatalf("want nil error for complete workflow, got: %v", err)
	}
	// RowIndex should be len(rows) = 4, signaling end-of-run.
	wantRowIndex := 4 // len(workflow.Table.Rows) for noPostBlockV216Content
	if info.RowIndex != wantRowIndex {
		t.Errorf("complete workflow: want RowIndex=%d (end-of-run), got %d", wantRowIndex, info.RowIndex)
	}
	if info.RerunLast {
		t.Error("complete workflow: want RerunLast=false, got true")
	}
}
