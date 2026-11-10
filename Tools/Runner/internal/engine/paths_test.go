package engine_test

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

func TestNext_Paths_FillIn_SubstitutedInInput(t *testing.T) {
	// quick-fix row 2: implementation-tdd has input "Stage-{Stage}/Plan.md".
	// When dispatched in Stage-1, it must resolve to "Stage-1/Plan.md".
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := scopeAgents(aw)
	state := stateAfter("PLANNING", "", "plan-review#2", domain.StatusSUCCESS, 2)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("plan-review#2"),
		Agents:       agents,
		Seq:          2,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	// Should have resolved Stage-{Stage}/Plan.md → Stage-1/Plan.md.
	found := false
	for _, a := range step.Request.InputArtifacts {
		if a == "Stage-1/Plan.md" {
			found = true
		}
		if strings.Contains(a, "{Stage}") {
			t.Errorf("unresolved {Stage} found in input artifact: %s", a)
		}
	}
	if !found {
		t.Errorf("want Stage-1/Plan.md in InputArtifacts, got %v", step.Request.InputArtifacts)
	}
}

// TestNext_Paths_FillIn_SubstitutedInOutput verifies {Stage} is
// replaced in output artifact paths too.
func TestNext_Paths_FillIn_SubstitutedInOutput(t *testing.T) {
	// quick-fix row 2: implementation-tdd output is "Stage-{Stage}/PlanProgress.md".
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := scopeAgents(aw)
	state := stateAfter("PLANNING", "", "plan-review#2", domain.StatusSUCCESS, 2)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("plan-review#2"),
		Agents:       agents,
		Seq:          2,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	found := false
	for _, a := range step.Request.OutputArtifacts {
		if a == "Stage-1/PlanProgress.md" {
			found = true
		}
		if strings.Contains(a, "{Stage}") {
			t.Errorf("unresolved {Stage} found in output artifact: %s", a)
		}
	}
	if !found {
		t.Errorf("want Stage-1/PlanProgress.md in OutputArtifacts, got %v", step.Request.OutputArtifacts)
	}
}

// TestNext_Paths_StageStar_InputPassedThroughOn216Table verifies that Stage-*
// in input artifacts of a 2.16 table is not matched by the engine, whatever
// the schedule holds: the entry reaches the session unchanged and the session
// lists the existing artifacts.
func TestNext_Paths_StageStar_InputPassedThroughOn216Table(t *testing.T) {
	// quick-fix row 1 (plan-review) has input "Stage-*/Plan.md, Stage-*/PlanProgress.md".
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	threeSchedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "Implementation-Only"},
		domain.ScheduleIteration{ID: "2", HITL: false, Approach: "Implementation-Only"},
		domain.ScheduleIteration{ID: "3", HITL: false, Approach: "Implementation-Only"},
	)
	agents := scopeAgents(aw)
	state := stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    threeSchedules,
		State:        state,
		LastResponse: successResponse("planner-tdd-soft#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)

	requireStageStarPassedThrough(t, step.Request.InputArtifacts)
}

// requireStageStarPassedThrough asserts the inputs hold the unexpanded
// Stage-*/Plan.md entry and no per-iteration Stage-N/Plan.md path.
func requireStageStarPassedThrough(t *testing.T, inputs []string) {
	t.Helper()
	found := false
	for _, a := range inputs {
		if a == "Stage-*/Plan.md" {
			found = true
			continue
		}
		if strings.HasPrefix(a, "Stage-") && strings.HasSuffix(a, "/Plan.md") {
			t.Errorf("Stage-N/Plan.md path %q: the engine must not expand a 2.16 wildcard; inputs %v", a, inputs)
		}
	}
	if !found {
		t.Errorf("want Stage-*/Plan.md passed through unchanged; inputs %v", inputs)
	}
}

// TestNext_Paths_StageStar_OutputPassedThrough verifies that Stage-* in output
// artifacts is dispatched as-is (not expanded).
func TestNext_Paths_StageStar_OutputPassedThrough(t *testing.T) {
	// quick-fix row 0 (planner-tdd-soft) output includes "Stage-*/Plan.md" and
	// "Stage-*/PlanProgress.md". These must pass through to the subagent unchanged.
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
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	found := false
	for _, a := range step.Request.OutputArtifacts {
		if a == "Stage-*/Plan.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("want Stage-*/Plan.md unexpanded in OutputArtifacts, got %v", step.Request.OutputArtifacts)
	}
}

// ===== Non-EXECUTION Stage-* resolution =====

// TestNext_Paths_NonExecution_StageStar_NoSchedulesIsNotAnError verifies that
// a 2.16 table's Stage-* input needs no schedule at all: the engine passes the
// entry through, so a row outside every block is dispatched.
func TestNext_Paths_NonExecution_StageStar_NoSchedulesIsNotAnError(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	agents := scopeAgents(aw)
	state := stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		State:        state,
		LastResponse: successResponse("planner-tdd-soft#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)

	requireStageStarPassedThrough(t, step.Request.InputArtifacts)
}

// ===== DispatchStep carries artifact paths exactly =====

// TestNext_Artifacts_PreExecutionRow_NoTemplateExpansion verifies that
// pre-execution artifact paths with no template variables are passed through
// unchanged.
func TestNext_Artifacts_PreExecutionRow_NoTemplateExpansion(t *testing.T) {
	// quick-fix row 0: planner-tdd-soft output = "Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md"
	// Plan.md has no template — must appear unchanged.
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
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	found := false
	for _, a := range step.Request.OutputArtifacts {
		if a == "Plan.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("want Plan.md in OutputArtifacts unchanged, got %v", step.Request.OutputArtifacts)
	}
}

// TestNext_Artifacts_InputArtifactsMatchRow verifies that InputArtifacts in the
// ProtocolRequest are populated from the routing row (after template resolution).
func TestNext_Artifacts_InputArtifactsMatchRow(t *testing.T) {
	// quick-fix row 1 (plan-review): inputs are Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md.
	// The Stage-* entries are passed through for the session to expand.
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
	if len(step.Request.InputArtifacts) == 0 {
		t.Error("want non-empty InputArtifacts for plan-review row, got empty")
	}
	// "Plan.md" (no template) must appear.
	foundPlanMd := false
	for _, a := range step.Request.InputArtifacts {
		if a == "Plan.md" {
			foundPlanMd = true
		}
	}
	if !foundPlanMd {
		t.Errorf("want Plan.md in InputArtifacts, got %v", step.Request.InputArtifacts)
	}
}
