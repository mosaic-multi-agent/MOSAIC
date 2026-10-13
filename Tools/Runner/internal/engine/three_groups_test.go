package engine_test

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ===== Table-driven group ordering (three or more groups) =====
//
// threeGroupV216Content is a minimal grouped block with three named groups
// (Alpha, Beta, Gamma) and a four-row approach table. It is used to verify that
// the engine uses the approach table for ordering rather than the natural group
// declaration order or any hardcoded mapping.
//
// Group layout (1-based routing-table rows, no pre-block rows):
//
//	Row 1: EXECUTION.[Stage].Alpha  agent-alpha
//	Row 2: EXECUTION.[Stage].Beta   agent-beta
//	Row 3: EXECUTION.[Stage].Gamma  agent-gamma
//
// Approach table:
//
//	AlphaFirst       -> Alpha, Beta, Gamma
//	BetaFirst        -> Beta, Alpha, Gamma
//	GammaOnly        -> Gamma
//	AlphaBeta        -> Alpha, Beta
const threeGroupV216Content = v216Columns +
	`| EXECUTION.[Stage].Alpha | agent-alpha | FALSE | COMPLETE | - | - | - |
| EXECUTION.[Stage].Beta | agent-beta | FALSE | COMPLETE | - | - | - |
| EXECUTION.[Stage].Gamma | agent-gamma | FALSE | COMPLETE | - | - | - |
` + v216StageBlock + `
**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | AlphaFirst | Alpha, Beta, Gamma |
| Stage | BetaFirst | Beta, Alpha, Gamma |
| Stage | GammaOnly | Gamma |
| Stage | AlphaBeta | Alpha, Beta |
`

// TestNext_ThreeGroups_BetaFirst_FirstDispatchIsBeta verifies that when the workflow
// has three named groups and the stage uses the "BetaFirst" approach, the engine
// dispatches agent-beta first (Beta group appears before Alpha in that row).
//
// This test is RED with the current implementation, which uses orderedGroupsForApproach
// and falls back to the natural group order (Alpha first) when TwoGroup=false.
// The new orderedGroupsForStage implementation must look up the approach table to
// honour the row's group sequence.
func TestNext_ThreeGroups_BetaFirst_FirstDispatchIsBeta(t *testing.T) {
	aw := mustParseAndAdmitV216(t, threeGroupV216Content, "three-group", "1.0")
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "BetaFirst"},
	)
	agents := newTestAgents("agent-alpha", "agent-beta", "agent-gamma")

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        emptyState(),
		LastResponse: nil,
		Agents:       agents,
		Seq:          0,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "agent-beta" {
		t.Errorf("BetaFirst approach: want first dispatch to agent-beta (Beta group first in approach row), got %s",
			step.Request.AgentInstanceID)
	}
	if step.Stage != "1.Beta" {
		t.Errorf("want Beta.1, got %q", step.Stage)
	}
}

// TestNext_ThreeGroups_GammaOnly_SkipsAlphaAndBeta verifies that with the "GammaOnly"
// approach, only the Gamma group rows are dispatched. Alpha and Beta are omitted.
//
// This test is RED: the current implementation (TwoGroup=false) returns all groups
// and would dispatch agent-alpha first, not agent-gamma.
func TestNext_ThreeGroups_GammaOnly_SkipsAlphaAndBeta(t *testing.T) {
	aw := mustParseAndAdmitV216(t, threeGroupV216Content, "three-group", "1.0")
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "GammaOnly"},
	)
	agents := newTestAgents("agent-alpha", "agent-beta", "agent-gamma")

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        emptyState(),
		LastResponse: nil,
		Agents:       agents,
		Seq:          0,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "agent-gamma" {
		t.Errorf("GammaOnly approach: want first (only) dispatch to agent-gamma, got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_ThreeGroups_GammaOnly_AfterGamma_NextStageStartsWithGamma verifies
// that after agent-gamma completes stage 1 under the GammaOnly approach, stage 2
// begins with agent-gamma again (not agent-alpha). With the declared-order
// fallback the single implicit group covers all rows, so stage 2 would start at
// row 0 (agent-alpha); table-driven routing correctly starts at Gamma's row.
//
// RED: the current implementation (TwoGroup=false) uses a single implicit group
// covering all rows, so stage 2 starts with agent-alpha rather than agent-gamma.
func TestNext_ThreeGroups_GammaOnly_AfterGamma_NextStageStartsWithGamma(t *testing.T) {
	aw := mustParseAndAdmitV216(t, threeGroupV216Content, "three-group", "1.0")
	// Two schedules with GammaOnly: Gamma is the only dispatched group each stage.
	// After Gamma in stage 1 the engine must begin stage 2 with Gamma, not Alpha.
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "GammaOnly"},
		domain.ScheduleIteration{ID: "2", HITL: false, Approach: "GammaOnly"},
	)
	agents := newTestAgents("agent-alpha", "agent-beta", "agent-gamma")
	// agent-gamma is the only dispatched row (seq=1) under GammaOnly.
	state := stateAfterRow("EXECUTION", "1.Gamma", "agent-gamma#1", domain.StatusSUCCESS, 1, 3)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("agent-gamma#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if step.Stage != "2.Gamma" {
		t.Errorf("GammaOnly: after stage 1 Gamma, want Gamma.2, got %q", step.Stage)
	}
	if agentName(step.Request.AgentInstanceID) != "agent-gamma" {
		t.Errorf("GammaOnly stage 2: want agent-gamma (table-driven), got %s", step.Request.AgentInstanceID)
	}
}

// TestNext_ThreeGroups_AlphaBeta_SkipsGamma verifies that with the "AlphaBeta"
// approach, Gamma is skipped. After agent-alpha and agent-beta complete the stage,
// the engine should advance to the next stage (or complete), not dispatch agent-gamma.
//
// This test is RED: the current implementation (TwoGroup=false) returns all groups
// and would dispatch agent-gamma after agent-beta.
func TestNext_ThreeGroups_AlphaBeta_SkipsGamma(t *testing.T) {
	aw := mustParseAndAdmitV216(t, threeGroupV216Content, "three-group", "1.0")
	// Two schedules: stage 1 AlphaBeta, stage 2 AlphaBeta.
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "AlphaBeta"},
		domain.ScheduleIteration{ID: "2", HITL: false, Approach: "AlphaBeta"},
	)
	agents := newTestAgents("agent-alpha", "agent-beta", "agent-gamma")
	// agent-beta is row 1 (last row in the Alpha+Beta sequence). After it completes
	// in stage 1, the engine should advance to stage 2 Alpha (row 0), not dispatch
	// Gamma (which is skipped by AlphaBeta).
	state := stateAfterRow("EXECUTION", "1.Beta", "agent-beta#2", domain.StatusSUCCESS, 2, 2)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("agent-beta#2"),
		Agents:       agents,
		Seq:          2,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	// Should advance to stage 2, starting with Alpha (first group in AlphaBeta order).
	if agentName(step.Request.AgentInstanceID) != "agent-alpha" {
		t.Errorf("AlphaBeta: after agent-beta in stage 1, want agent-alpha in stage 2, got %s",
			step.Request.AgentInstanceID)
	}
	if step.Stage != "2.Alpha" {
		t.Errorf("AlphaBeta: want Alpha.2 after Alpha+Beta complete in stage 1, got %q", step.Stage)
	}
}

// TestNext_ThreeGroups_BetaFirst_FullSequence verifies that with "BetaFirst"
// approach, all three groups are dispatched in the table-driven order (Beta,
// Alpha, Gamma) — which diverges from the declared row order (Alpha, Beta, Gamma).
// This test fails without table-driven routing because the current fallback runs
// groups in declaration order (Alpha first), whereas the table says Beta first.
//
// RED: the current implementation (TwoGroup=false) uses declared order and
// dispatches agent-alpha first rather than agent-beta.
func TestNext_ThreeGroups_BetaFirst_FullSequence(t *testing.T) {
	aw := mustParseAndAdmitV216(t, threeGroupV216Content, "three-group", "1.0")
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "BetaFirst"},
	)
	agents := newTestAgents("agent-alpha", "agent-beta", "agent-gamma")

	// BetaFirst → Beta (row 1), Alpha (row 0), Gamma (row 2): the order diverges
	// from declared row order, so it cannot coincide with the declared-order fallback.
	wantSequence := []string{"agent-beta", "agent-alpha", "agent-gamma"}
	states := []domain.ArtifactState{
		emptyState(),
		stateAfterRow("EXECUTION", "1.Beta", "agent-beta#1", domain.StatusSUCCESS, 1, 2),
		stateAfterRow("EXECUTION", "1.Alpha", "agent-alpha#2", domain.StatusSUCCESS, 2, 1),
	}
	responses := []*domain.ProtocolResponse{
		nil,
		successResponse("agent-beta#1"),
		successResponse("agent-alpha#2"),
	}

	for i, want := range wantSequence {
		dec := engine.Next(engine.NextInput{
			Workflow:     aw,
			Schedules:    schedules,
			State:        states[i],
			LastResponse: responses[i],
			Agents:       agents,
			Seq:          i,
			Now:          fixedNow,
			Mode:         domain.ExecutionModeAutoReview,
		})
		step := requireDispatch(t, dec)
		if agentName(step.Request.AgentInstanceID) != want {
			t.Errorf("BetaFirst step %d: want %s, got %s", i+1, want, step.Request.AgentInstanceID)
		}
	}
}

// TestNext_ThreeGroups_GroupInEveryApproachRow_RunsInEveryStage verifies that a
// group present in an approach row is dispatched in every stage that uses that
// approach. Both schedules use BetaFirst (→ Beta, Alpha, Gamma), so after Gamma
// completes stage 1 the engine must begin stage 2 with Beta, not with Alpha.
// This fails without table-driven routing because the declared-order fallback
// (single implicit group, all rows) always starts the next stage at row 0 (Alpha).
//
// RED: the current implementation uses declared row order and dispatches
// agent-alpha at the start of stage 2 rather than agent-beta.
func TestNext_ThreeGroups_GroupInEveryApproachRow_RunsInEveryStage(t *testing.T) {
	// BetaFirst: Beta (row 1), Alpha (row 0), Gamma (row 2). Gamma is last.
	// After Gamma finishes stage 1 the engine must pick up BetaFirst again for
	// stage 2, dispatching Beta — not Alpha (row 0, which declared-order would use).
	aw := mustParseAndAdmitV216(t, threeGroupV216Content, "three-group", "1.0")
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "BetaFirst"},
		domain.ScheduleIteration{ID: "2", HITL: false, Approach: "BetaFirst"},
	)
	agents := newTestAgents("agent-alpha", "agent-beta", "agent-gamma")

	// Stage 1 BetaFirst: Beta=seq1, Alpha=seq2, Gamma=seq3. After Gamma (#3) the
	// engine advances to stage 2 and must dispatch Beta first.
	state := stateAfterRow("EXECUTION", "1.Gamma", "agent-gamma#3", domain.StatusSUCCESS, 3, 3)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("agent-gamma#3"),
		Agents:       agents,
		Seq:          3,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if step.Stage != "2.Beta" {
		t.Errorf("after Gamma in stage 1, want Beta.2, got %q", step.Stage)
	}
	if agentName(step.Request.AgentInstanceID) != "agent-beta" {
		t.Errorf("stage 2 BetaFirst: want agent-beta first (table-driven), got %s",
			step.Request.AgentInstanceID)
	}
}

// ===== Unresolvable approach at the routing decision =====
//
// These tests verify that when a stage's Approach value has no matching row in
// the workflow's Groups table, the engine surfaces a StopDecision (or
// error from ResumePoint) rather than falling back to a default order.
// All tests are RED with the current implementation, which falls back to ApproachTDD.

// TestNext_UnresolvableApproach_InitialDispatch_ReturnsStop verifies that when the
// very first EXECUTION dispatch has an approach not in the workflow's table, the
// engine returns a StopDecision naming the unresolvable value.
//
// RED: current implementation falls back to ApproachTDD and dispatches normally.
