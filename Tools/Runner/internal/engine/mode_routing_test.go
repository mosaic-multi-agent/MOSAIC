package engine_test

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ===== Mode-gated routing =====

// TestNext_Mode_Auto_Success_RoutesNormally verifies that in auto mode a SUCCESS
// response routes to the On Success target exactly as before.
func TestNext_Mode_Auto_Success_RoutesNormally(t *testing.T) {
	// quick-fix row 0: planner-tdd-soft, OnSuccess="plan-review"
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := scopeAgents(aw)
	state := stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("planner-tdd-soft#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAuto,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "plan-review" {
		t.Errorf("auto mode SUCCESS: want On Success route to plan-review, got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_Mode_AutoReview_Success_RoutesNormally verifies that in auto-review
// mode SUCCESS routing is identical to auto — the auto-review extension applies
// only to CNA with an unambiguous On Findings hint.
func TestNext_Mode_AutoReview_Success_RoutesNormally(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := scopeAgents(aw)
	state := stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("planner-tdd-soft#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "plan-review" {
		t.Errorf("auto-review mode SUCCESS: want On Success route to plan-review, got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_Mode_Orchestrated_FirstCall_ReturnsConsultDecision verifies that in
// orchestrated mode the engine returns a ConsultDecision on the very first call
// (before any agent has run), with trigger ConsultTriggerOrchestratedMode.
func TestNext_Mode_Orchestrated_FirstCall_ReturnsConsultDecision(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := scopeAgents(aw)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        emptyState(),
		LastResponse: nil,
		Agents:       agents,
		Seq:          0,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeOrchestrated,
	})

	consult := requireConsult(t, dec)
	if consult.Trigger != domain.ConsultTriggerOrchestratedMode {
		t.Errorf("orchestrated mode first call: want trigger ConsultTriggerOrchestratedMode, got %q",
			consult.Trigger)
	}
	// First call: CurrentRow must be -1 (no row dispatched yet).
	if consult.CurrentRow != -1 {
		t.Errorf("orchestrated mode first call: want CurrentRow=-1 (no row yet), got %d",
			consult.CurrentRow)
	}
}

// TestNext_Mode_Orchestrated_AfterSuccess_ReturnsConsultDecision verifies that
// in orchestrated mode every call returns a ConsultDecision, not just the first.
// Even a SUCCESS response does not produce a routing decision from the engine.
func TestNext_Mode_Orchestrated_AfterSuccess_ReturnsConsultDecision(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := scopeAgents(aw)
	state := stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("planner-tdd-soft#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeOrchestrated,
	})

	consult := requireConsult(t, dec)
	if consult.Trigger != domain.ConsultTriggerOrchestratedMode {
		t.Errorf("orchestrated mode after SUCCESS: want trigger ConsultTriggerOrchestratedMode, got %q",
			consult.Trigger)
	}
	// After a PLANNING-phase SUCCESS the current phase is "PLANNING" and stage is "".
	if consult.CurrentPhase != "PLANNING" {
		t.Errorf("orchestrated mode after SUCCESS: want CurrentPhase=%q, got %q",
			"PLANNING", consult.CurrentPhase)
	}
	if consult.CurrentStage != "" {
		t.Errorf("orchestrated mode after SUCCESS: want CurrentStage=%q (empty for PLANNING phase), got %q",
			"", consult.CurrentStage)
	}
}

// TestNext_Mode_Orchestrated_AfterCNA_ReturnsConsultDecision verifies that
// in orchestrated mode even a CNA response does not produce a DeviationDecision
// or a DispatchDecision — it always produces ConsultDecision.
func TestNext_Mode_Orchestrated_AfterCNA_ReturnsConsultDecision(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")
	agents := scopeAgents(aw)
	// build-review names an On Findings target. In auto-review this dispatches;
	// in orchestrated it must still consult.
	state := stateAfterRow("EXECUTION", "1.Test", "build-review#9", domain.StatusCOMPLETED_NEEDS_ACTION, 9, 9)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: cnaResponse("build-review#9"),
		Agents:       agents,
		Seq:          9,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeOrchestrated,
	})

	consult := requireConsult(t, dec)
	if consult.Trigger != domain.ConsultTriggerOrchestratedMode {
		t.Errorf("orchestrated mode after CNA: want ConsultTriggerOrchestratedMode, got %q",
			consult.Trigger)
	}
	// After a CNA in EXECUTION the current phase and stage come from the state.
	if consult.CurrentPhase != "EXECUTION" {
		t.Errorf("orchestrated mode after CNA: want CurrentPhase=%q, got %q",
			"EXECUTION", consult.CurrentPhase)
	}
	if consult.CurrentStage != "1.Test" {
		t.Errorf("orchestrated mode after CNA: want CurrentStage=%q, got %q",
			"1.Test", consult.CurrentStage)
	}
}

// TestNext_Mode_Auto_NonCNA_NonSuccess_ReturnsDeviation verifies that in auto
// mode non-CNA non-SUCCESS statuses always yield Deviation regardless of On Findings.
func TestNext_Mode_Auto_NonCNA_NonSuccess_ReturnsDeviation(t *testing.T) {
	// tests-review-tdd has On Findings = test-writer-tdd, but the
	// status is NEEDS_CLARIFICATION (not CNA, not retried mechanically), so it
	// must deviate.
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("TDD")
	agents := scopeAgents(aw)
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#9", domain.StatusNEEDS_CLARIFICATION, 9, 9)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: nonSuccessResponse("tests-review-tdd#9", domain.StatusNEEDS_CLARIFICATION),
		Agents:       agents,
		Seq:          9,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAuto,
	})

	requireDeviation(t, dec)
}

// TestNext_Mode_AutoReview_NonCNA_NonSuccess_ReturnsDeviation verifies that in
// auto-review mode non-CNA non-SUCCESS statuses also yield Deviation, not Dispatch.
func TestNext_Mode_AutoReview_NonCNA_NonSuccess_ReturnsDeviation(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("TDD")
	agents := scopeAgents(aw)
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#9", domain.StatusBLOCKED, 9, 9)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: nonSuccessResponse("tests-review-tdd#9", domain.StatusBLOCKED),
		Agents:       agents,
		Seq:          9,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	requireDeviation(t, dec)
}

// ===== Unset mode =====

// TestNext_Mode_Unset_ReturnsDeviationAmbiguousRoute verifies that an engine
// call with no execution mode set routes nothing and deviates as an ambiguous
// route.
func TestNext_Mode_Unset_ReturnsDeviationAmbiguousRoute(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := scopeAgents(aw)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        emptyState(),
		LastResponse: nil,
		Agents:       agents,
		Seq:          0,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeUnset,
	})

	dev := requireDeviation(t, dec)
	if dev.Info.Kind != domain.DeviationAmbiguousRoute {
		t.Errorf("ExecutionModeUnset: want DeviationKind=%q, got %q",
			domain.DeviationAmbiguousRoute, dev.Info.Kind)
	}
}
