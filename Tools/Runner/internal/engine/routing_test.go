package engine_test

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// nextForRecordedStep calls engine.Next in the given mode (auto-review is the mode in which the
// engine routes findings) for the step recorded in state, answered by resp.
func nextForRecordedStep(
	aw domain.AdmittedWorkflow,
	state domain.ArtifactState,
	resp *domain.ProtocolResponse,
	mode domain.ExecutionMode,
) domain.EngineDecision {
	return engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    singleSchedules("TDD"),
		State:        state,
		LastResponse: resp,
		Agents:       scopeAgents(aw),
		Seq:          state.GlobalSequence,
		Now:          fixedNow,
		Mode:         mode,
	})
}

func TestNext_PreExecution_OnSuccess_AdvancesToNamedAgent(t *testing.T) {
	// quick-fix row 0: planner-tdd-soft, On Success = plan-review
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	state := stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)

	dec := nextForRecordedStep(aw, state, successResponse("planner-tdd-soft#1"), domain.ExecutionModeAutoReview)

	step := requireDispatch(t, dec)
	// On Success of row 0 = "plan-review" -> dispatch row 1.
	if agentName(step.Request.AgentInstanceID) != "plan-review" {
		t.Errorf("want agent plan-review (from On Success), got %s", step.Request.AgentInstanceID)
	}
	if step.RowIndex != 1 {
		t.Errorf("want RowIndex=1 (plan-review), got %d", step.RowIndex)
	}
}

// TestNext_PostExecution_CompleteTarget_ReturnsComplete verifies that a SUCCESS
// response from the last run-scope row whose On Success is COMPLETE causes the
// engine to return a CompleteDecision.
func TestNext_PostExecution_CompleteTarget_ReturnsComplete(t *testing.T) {
	// quick-fix row 3: test-runner, On Success = COMPLETE
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	state := stateAfter("REVIEW", "", "test-runner#4", domain.StatusSUCCESS, 4)

	dec := nextForRecordedStep(aw, state, successResponse("test-runner#4"), domain.ExecutionModeAutoReview)

	requireComplete(t, dec)
}

// ===== On Findings hint-based auto-routing =====

// TestNext_OnFindings_AutoReview_NamedRow_CNA_ReturnsDispatch verifies that in
// auto-review mode a COMPLETED_NEEDS_ACTION response from a row whose On
// Findings names a row causes the engine to return a DispatchDecision for
// that row (loop-back), not a Deviation.
func TestNext_OnFindings_AutoReview_NamedRow_CNA_ReturnsDispatch(t *testing.T) {
	// build-verified row 9: build-review(tests) in the Test group, On Findings = test-writer-tdd.
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	state := stateAfterRow("EXECUTION", "1.Test", "build-review#9", domain.StatusCOMPLETED_NEEDS_ACTION, 9, 9)

	dec := nextForRecordedStep(aw, state, cnaResponse("build-review#9"), domain.ExecutionModeAutoReview)

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "test-writer-tdd" {
		t.Errorf("auto-review+CNA+On Findings: want loop-back Dispatch to test-writer-tdd, got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_OnFindings_Auto_NamedRow_CNA_ReturnsDeviation verifies that in auto
// mode a COMPLETED_NEEDS_ACTION response from a row whose On Findings names a
// row returns a DeviationDecision, not a Dispatch. The On-Findings auto-route
// fires only in auto-review mode.
func TestNext_OnFindings_Auto_NamedRow_CNA_ReturnsDeviation(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	state := stateAfterRow("EXECUTION", "1.Test", "build-review#9", domain.StatusCOMPLETED_NEEDS_ACTION, 9, 9)

	dec := nextForRecordedStep(aw, state, cnaResponse("build-review#9"), domain.ExecutionModeAuto)

	requireDeviation(t, dec)
}

// TestNext_OnFindings_AutoReview_NamedRow_CNA_InImplementationGroup_ReturnsDispatch
// verifies that in auto-review mode the On Findings auto-routing works in the
// Implementation group too.
func TestNext_OnFindings_AutoReview_NamedRow_CNA_InImplementationGroup_ReturnsDispatch(t *testing.T) {
	// build-verified row 12: build-review(impl) in the Implementation group, On Findings = implementation-tdd.
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	state := stateAfterRow("EXECUTION", "1.Implementation", "build-review#12", domain.StatusCOMPLETED_NEEDS_ACTION, 12, 12)

	dec := nextForRecordedStep(aw, state, cnaResponse("build-review#12"), domain.ExecutionModeAutoReview)

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "implementation-tdd" {
		t.Errorf("auto-review+CNA+On Findings in a group: want loop-back to implementation-tdd, got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_OnFindings_Auto_NamedRow_CNA_InImplementationGroup_ReturnsDeviation
// verifies that in auto mode a CNA response with an On Findings target in the
// Implementation group also returns Deviation, not Dispatch.
func TestNext_OnFindings_Auto_NamedRow_CNA_InImplementationGroup_ReturnsDeviation(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	state := stateAfterRow("EXECUTION", "1.Implementation", "build-review#12", domain.StatusCOMPLETED_NEEDS_ACTION, 12, 12)

	dec := nextForRecordedStep(aw, state, cnaResponse("build-review#12"), domain.ExecutionModeAuto)

	requireDeviation(t, dec)
}

// TestNext_OnFindings_EmptyValue_CNA_ReturnsDeviation verifies that a CNA
// response from a row whose On Findings is "-" yields a DeviationDecision.
func TestNext_OnFindings_EmptyValue_CNA_ReturnsDeviation(t *testing.T) {
	// brownfield-tdd row 8: test-writer-tdd has On Findings = "-".
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	state := stateAfterRow("EXECUTION", "1.Test", "test-writer-tdd#8", domain.StatusCOMPLETED_NEEDS_ACTION, 8, 8)

	dec := nextForRecordedStep(aw, state, cnaResponse("test-writer-tdd#8"), domain.ExecutionModeAutoReview)

	requireDeviation(t, dec)
}

// TestNext_OnFindings_NonCNA_NonSuccess_ReturnsDeviation verifies that for
// non-CNA non-SUCCESS statuses, the engine always returns Deviation regardless
// of whether On Findings names a row. On Findings auto-routing is only for CNA.
func TestNext_OnFindings_NonCNA_NonSuccess_ReturnsDeviation(t *testing.T) {
	// brownfield-tdd row 9: tests-review-tdd has On Findings = test-writer-tdd.
	// If it returns NEEDS_CLARIFICATION (not CNA), the engine must return Deviation,
	// even though On Findings names a row.
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#9", domain.StatusNEEDS_CLARIFICATION, 9, 9)

	dec := nextForRecordedStep(aw, state, nonSuccessResponse("tests-review-tdd#9", domain.StatusNEEDS_CLARIFICATION),
		domain.ExecutionModeAutoReview)

	requireDeviation(t, dec)
}

// TestNext_Deviation_Kind_IsNonSuccess verifies the DeviationKind in the
// deviation info is set to DeviationNonSuccess for a non-SUCCESS response.
func TestNext_Deviation_Kind_IsNonSuccess(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#9", domain.StatusBLOCKED, 9, 9)

	dec := nextForRecordedStep(aw, state, nonSuccessResponse("tests-review-tdd#9", domain.StatusBLOCKED),
		domain.ExecutionModeAutoReview)

	dev := requireDeviation(t, dec)
	if dev.Info.Kind != domain.DeviationNonSuccess {
		t.Errorf("want DeviationKind=%q, got %q", domain.DeviationNonSuccess, dev.Info.Kind)
	}
}

// TestNext_Deviation_IncludesCurrentRowAndPhase verifies that DeviationInfo
// carries the current row index and phase for the resolver's context.
func TestNext_Deviation_IncludesCurrentRowAndPhase(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	// tests-review-tdd is row 8 (zero-based) in brownfield-tdd.
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#9", domain.StatusBLOCKED, 9, 9)

	dec := nextForRecordedStep(aw, state, nonSuccessResponse("tests-review-tdd#9", domain.StatusBLOCKED),
		domain.ExecutionModeAutoReview)

	dev := requireDeviation(t, dec)
	if dev.Info.CurrentRow != 8 {
		t.Errorf("want CurrentRow=8 (tests-review-tdd), got %d", dev.Info.CurrentRow)
	}
	if dev.Info.CurrentPhase != "EXECUTION.[Stage].Test" {
		t.Errorf("want CurrentPhase=EXECUTION.[Stage].Test, got %q", dev.Info.CurrentPhase)
	}
	if dev.Info.CurrentStage != "1.Test" {
		t.Errorf("want CurrentStage=Test.1, got %q", dev.Info.CurrentStage)
	}
}
