package engine_test

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ===== Grouped block: approach-driven group ordering =====

// TestNext_Approach_TDD_DispatchesTestGroupFirst verifies that TDD approach
// dispatches the test group (test-writer-tdd) before the implementation group.
func TestNext_Approach_TDD_DispatchesTestGroupFirst(t *testing.T) {
	// brownfield-tdd: after contracts-review (row 6, last pre-execution row),
	// with TDD approach, the first EXECUTION dispatch should be test-writer-tdd (row 7).
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("TDD")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
	state := stateAfter("DESIGN", "", "contracts-review#7", domain.StatusSUCCESS, 7)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("contracts-review#7"),
		Agents:       agents,
		Seq:          7,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "test-writer-tdd" {
		t.Errorf("TDD approach: want first EXECUTION dispatch to test-writer-tdd, got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_Approach_ImplementationFirst_DispatchesImplGroupFirst verifies that
// Implementation-First approach dispatches the implementation group first.
func TestNext_Approach_ImplementationFirst_DispatchesImplGroupFirst(t *testing.T) {
	// brownfield-tdd: after contracts-review (row 6), with Implementation-First
	// approach, the first EXECUTION dispatch should be implementation-tdd (row 9),
	// not test-writer-tdd.
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("Implementation-First")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
	state := stateAfter("DESIGN", "", "contracts-review#7", domain.StatusSUCCESS, 7)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("contracts-review#7"),
		Agents:       agents,
		Seq:          7,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "implementation-tdd" {
		t.Errorf("Implementation-First approach: want first EXECUTION dispatch to implementation-tdd, got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_Approach_ImplementationOnly_SkipsTestGroup verifies that
// Implementation-Only approach never dispatches test group agents.
// After contracts-review, the first dispatch must be implementation-tdd.
func TestNext_Approach_ImplementationOnly_SkipsTestGroup(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("Implementation-Only")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
	state := stateAfter("DESIGN", "", "contracts-review#7", domain.StatusSUCCESS, 7)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("contracts-review#7"),
		Agents:       agents,
		Seq:          7,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	name := agentName(step.Request.AgentInstanceID)
	if name == "test-writer-tdd" || name == "tests-review-tdd" {
		t.Errorf("Implementation-Only approach: must not dispatch test agents, got %s", name)
	}
	if name != "implementation-tdd" {
		t.Errorf("Implementation-Only approach: want implementation-tdd, got %s", name)
	}
}

// TestNext_Approach_TestsOnly_SkipsImplGroup verifies that Tests-Only approach
// never dispatches implementation group agents.
// After contracts-review, the first dispatch must be test-writer-tdd.
func TestNext_Approach_TestsOnly_SkipsImplGroup(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("Tests-Only")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
	state := stateAfter("DESIGN", "", "contracts-review#7", domain.StatusSUCCESS, 7)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("contracts-review#7"),
		Agents:       agents,
		Seq:          7,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	name := agentName(step.Request.AgentInstanceID)
	if name == "implementation-tdd" || name == "implementation-review" {
		t.Errorf("Tests-Only approach: must not dispatch implementation agents, got %s", name)
	}
	if name != "test-writer-tdd" {
		t.Errorf("Tests-Only approach: want test-writer-tdd, got %s", name)
	}
}

// ===== Grouped block: iteration progression =====

// TestNext_Staged_TDD_AfterTestGroup_DispatchesImplGroup verifies that after
// the last row of the test group, the next dispatch is the first row of the
// implementation group (TDD approach).
func TestNext_Staged_TDD_AfterTestGroup_DispatchesImplGroup(t *testing.T) {
	// brownfield-tdd, TDD approach:
	// Test group: rows 7 (test-writer-tdd) and 8 (tests-review-tdd).
	// After tests-review-tdd succeeds (row 8), next must be implementation-tdd (row 9).
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("TDD")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#9", domain.StatusSUCCESS, 9, 9)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("tests-review-tdd#9"),
		Agents:       agents,
		Seq:          9,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "implementation-tdd" {
		t.Errorf("after last test-group row, want implementation-tdd (impl group), got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_Staged_TDD_AfterImplGroupLastStage_AdvancesToNextStage verifies that
// after the last row of the implementation group in stage N (with more schedules
// remaining), the engine starts stage N+1 with the test group (TDD approach).
func TestNext_Staged_TDD_AfterImplGroupLastStage_AdvancesToNextStage(t *testing.T) {
	// brownfield-tdd, TDD approach, two schedules:
	// After implementation-review (row 10) in Stage-1, advance to Stage-2 test group.
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := twoSchedules("TDD")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-review#11", domain.StatusSUCCESS, 11, 11)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("implementation-review#11"),
		Agents:       agents,
		Seq:          11,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	// Should start Stage-2, test group first (TDD) → test-writer-tdd.
	if agentName(step.Request.AgentInstanceID) != "test-writer-tdd" {
		t.Errorf("after iteration 1 last impl-group row, want test-writer-tdd in iteration 2, got %s",
			step.Request.AgentInstanceID)
	}
	if step.Stage != "2.Test" {
		t.Errorf("want Stage=2.Test, got %q", step.Stage)
	}
}

// TestNext_Staged_AfterLastStage_DispatchesPostExecutionRow verifies that after
// the last row of the last stage's last group, the engine dispatches the first
// post-execution row.
func TestNext_Staged_AfterLastStage_DispatchesPostExecutionRow(t *testing.T) {
	// brownfield-tdd, TDD approach, one stage:
	// After implementation-review (row 10, last impl-group row, only stage),
	// engine must dispatch test-runner (row 11, the post-execution REVIEW row).
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("TDD")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review",
		"test-runner",
	)
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-review#11", domain.StatusSUCCESS, 11, 11)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("implementation-review#11"),
		Agents:       agents,
		Seq:          11,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "test-runner" {
		t.Errorf("after last stage last group, want post-execution test-runner, got %s",
			step.Request.AgentInstanceID)
	}
	// Post-execution row has no Stage context.
	if step.Stage != "" {
		t.Errorf("want Stage=\"\" for post-execution row, got %q", step.Stage)
	}
}

// TestNext_Staged_AfterLastStage_NoPostExecution_ReturnsComplete verifies that
// when a workflow has no post-execution rows, the engine returns CompleteDecision
// after the last row of the last stage.
func TestNext_Staged_AfterLastStage_NoPostExecution_ReturnsComplete(t *testing.T) {
	// brownfield-tdd-build-verified has no post-execution rows (table ends at row 12).
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "build-review", "tests-review-tdd",
		"implementation-tdd", "implementation-review",
	)
	// implementation-review is row 12, the last EXECUTION row (and last row overall).
	state := stateAfterRow("EXECUTION", "1.Implementation", "implementation-review#13", domain.StatusSUCCESS, 13, 13)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("implementation-review#13"),
		Agents:       agents,
		Seq:          13,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	requireComplete(t, dec)
}
