package engine_test

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ===== Implementation-First approach: test group runs after implementation group =====

// TestNext_Approach_ImplementationFirst_AfterImplGroup_DispatchesTestGroup verifies
// that with Implementation-First approach, after the last row of the implementation
// group, the engine dispatches the first row of the test group.
func TestNext_Approach_ImplementationFirst_AfterImplGroup_DispatchesTestGroup(t *testing.T) {
	// brownfield-tdd, Implementation-First: impl group = rows 9-10, test group = rows 7-8.
	// After implementation-review (row 10) in Stage-1, the next dispatch must be
	// test-writer-tdd (row 7, first test group row).
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("Implementation-First")
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
	if agentName(step.Request.AgentInstanceID) != "test-writer-tdd" {
		t.Errorf("Implementation-First: after impl group, want test-writer-tdd (first test-group row), got %s",
			step.Request.AgentInstanceID)
	}
}

// TestNext_Approach_TestsOnly_AfterTestGroup_AdvancesToNextStage verifies that
// with Tests-Only approach, after the last row of the test group, the engine
// advances to the next stage (not the implementation group, which is skipped).
func TestNext_Approach_TestsOnly_AfterTestGroup_AdvancesToNextStage(t *testing.T) {
	// brownfield-tdd, Tests-Only, 2 stages:
	// After tests-review-tdd (row 8, last test-group row) in Stage-1,
	// advance to Stage-2 test-writer-tdd (no implementation group).
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := twoSchedules("Tests-Only")
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
	if agentName(step.Request.AgentInstanceID) != "test-writer-tdd" {
		t.Errorf("Tests-Only after last test-group row: want test-writer-tdd in Stage-2, got %s",
			step.Request.AgentInstanceID)
	}
	if step.Stage != "2.Test" {
		t.Errorf("Tests-Only: want 2.Test, got %q", step.Stage)
	}
}

// ===== Build-verified workflow: duplicate agent (build-review appears twice) =====

// TestNext_BuildVerified_TDD_TestGroupSequence verifies the complete test-group
// sequence in brownfield-tdd-build-verified with TDD approach:
// test-writer-tdd → build-review → tests-review-tdd.
func TestNext_BuildVerified_TDD_TestGroupSequence(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "build-review", "tests-review-tdd",
		"implementation-tdd", "implementation-review",
	)

	tests := []struct {
		name      string
		state     domain.ArtifactState
		response  *domain.ProtocolResponse
		seq       int
		wantAgent string
	}{
		{
			name:      "after contracts-review, dispatches test-writer-tdd",
			state:     stateAfter("DESIGN", "", "contracts-review#7", domain.StatusSUCCESS, 7),
			response:  successResponse("contracts-review#7"),
			seq:       7,
			wantAgent: "test-writer-tdd",
		},
		{
			name:      "after test-writer-tdd, dispatches build-review (test build)",
			state:     stateAfterRow("EXECUTION", "1.Test", "test-writer-tdd#8", domain.StatusSUCCESS, 8, 8),
			response:  successResponse("test-writer-tdd#8"),
			seq:       8,
			wantAgent: "build-review",
		},
		{
			name:      "after build-review (test build), dispatches tests-review-tdd",
			state:     stateAfterRow("EXECUTION", "1.Test", "build-review#9", domain.StatusSUCCESS, 9, 9),
			response:  successResponse("build-review#9"),
			seq:       9,
			wantAgent: "tests-review-tdd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := engine.Next(engine.NextInput{
				Workflow:     aw,
				Schedules:    schedules,
				State:        tc.state,
				LastResponse: tc.response,
				Agents:       agents,
				Seq:          tc.seq,
				Now:          fixedNow,
				Mode:         domain.ExecutionModeAutoReview,
			})
			step := requireDispatch(t, dec)
			if agentName(step.Request.AgentInstanceID) != tc.wantAgent {
				t.Errorf("want %s, got %s", tc.wantAgent, step.Request.AgentInstanceID)
			}
		})
	}
}

// TestNext_BuildVerified_TDD_ImplGroupSequence verifies the complete impl-group
// sequence in brownfield-tdd-build-verified with TDD approach:
// implementation-tdd → build-review → implementation-review.
func TestNext_BuildVerified_TDD_ImplGroupSequence(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")
	agents := newTestAgents(
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "build-review", "tests-review-tdd",
		"implementation-tdd", "implementation-review",
	)

	tests := []struct {
		name      string
		state     domain.ArtifactState
		response  *domain.ProtocolResponse
		seq       int
		wantAgent string
	}{
		{
			name:      "after tests-review-tdd (last test-group row), dispatches implementation-tdd",
			state:     stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#10", domain.StatusSUCCESS, 10, 10),
			response:  successResponse("tests-review-tdd#10"),
			seq:       10,
			wantAgent: "implementation-tdd",
		},
		{
			name:      "after implementation-tdd, dispatches build-review (impl build)",
			state:     stateAfterRow("EXECUTION", "1.Implementation", "implementation-tdd#11", domain.StatusSUCCESS, 11, 11),
			response:  successResponse("implementation-tdd#11"),
			seq:       11,
			wantAgent: "build-review",
		},
		{
			name:      "after build-review (impl build), dispatches implementation-review",
			state:     stateAfterRow("EXECUTION", "1.Implementation", "build-review#12", domain.StatusSUCCESS, 12, 12),
			response:  successResponse("build-review#12"),
			seq:       12,
			wantAgent: "implementation-review",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := engine.Next(engine.NextInput{
				Workflow:     aw,
				Schedules:    schedules,
				State:        tc.state,
				LastResponse: tc.response,
				Agents:       agents,
				Seq:          tc.seq,
				Now:          fixedNow,
				Mode:         domain.ExecutionModeAutoReview,
			})
			step := requireDispatch(t, dec)
			if agentName(step.Request.AgentInstanceID) != tc.wantAgent {
				t.Errorf("want %s, got %s", tc.wantAgent, step.Request.AgentInstanceID)
			}
		})
	}
}

// ===== DispatchStep fields =====

// TestNext_DispatchStep_PhaseAndSchedule verifies that the dispatched step
// carries the correct Phase and Stage values for recording in the artifact.
func TestNext_DispatchStep_PhaseAndSchedule(t *testing.T) {
	// quick-fix: dispatching implementation-tdd (row 3, EXECUTION.[Stage]) in iteration 1.
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := newTestAgents("planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")
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
	// The bare phase name and the plain stage number are recorded, not the
	// routing table's qualified phase string or a "Stage-N" form.
	if step.Phase != "EXECUTION" {
		t.Errorf("want Phase=EXECUTION, got %q", step.Phase)
	}
	if step.Stage != "1" {
		t.Errorf("want Stage=1, got %q", step.Stage)
	}
}

// TestNext_DispatchStep_RowIndex verifies that RowIndex matches the routing table.
func TestNext_DispatchStep_RowIndex(t *testing.T) {
	// quick-fix: row 0 = planner-tdd-soft.
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := newTestAgents("planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")

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
	if step.RowIndex != 0 {
		t.Errorf("want RowIndex=0, got %d", step.RowIndex)
	}
}

// TestNext_DispatchStep_AgentIdentifier verifies that DispatchStep.Agent.Identifier
// matches the routing table's agent field.
func TestNext_DispatchStep_AgentIdentifier(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := newTestAgents("planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")

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
	if step.Agent.Identifier != "planner-tdd-soft" {
		t.Errorf("want Agent.Identifier=planner-tdd-soft, got %q", step.Agent.Identifier)
	}
}
