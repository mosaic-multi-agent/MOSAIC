package engine_test

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// creatorInjectionV216Content is a minimal EXECUTION-only workflow used to
// exercise the creator-artifact injection path. test-writer-tdd's output
// artifact (Stage-{Stage}/PlanProgress.md) is intentionally absent from its
// input artifact list so that creator-artifact injection produces a genuinely
// new entry that would not be present without it.
const creatorInjectionV216Content = v216Columns +
	`| EXECUTION.[Stage].Test | test-writer-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, ContractsDesign.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage].Test | tests-review-tdd | FALSE | COMPLETE | test-writer-tdd | Stage-{Stage}/PlanProgress.md | Stage-{Stage}/tests-review-tdd.md |
| EXECUTION.[Stage].Implementation | implementation-tdd | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md | Stage-{Stage}/impl.md |
` + v216StageBlock + `
**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | TDD | Test, Implementation |
`

func TestNext_CreatorArtifactInjection_RegistryPresent_InjectsOutputArtifact(t *testing.T) {
	aw := mustParseAndAdmitV216(t, creatorInjectionV216Content, "creator-injection", "1.0")
	schedules := singleSchedules("TDD")
	agents := newTestAgents("test-writer-tdd", "tests-review-tdd", "implementation-tdd")
	// tests-review-tdd (row 1) completed with CNA; OnFindings=test-writer-tdd.
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#5", domain.StatusCOMPLETED_NEEDS_ACTION, 5, 2)

	// Registry entry: test-writer-tdd's prior output, bare path after StripRunPrefix.
	registry := []domain.ArtifactRegistryEntry{
		{Artifact: "Stage-1/PlanProgress.md", CreatedIn: "EXECUTION.1.Test", CreatedBy: "test-writer-tdd#1"},
	}
	// Review agent's last output artifacts (injected by existing review logic).
	reviewOutputs := []string{"Stage-1/tests-review-tdd.md"}

	dec := engine.Next(engine.NextInput{
		Workflow:            aw,
		Schedules:           schedules,
		State:               state,
		LastResponse:        cnaResponse("tests-review-tdd#5"),
		LastOutputArtifacts: reviewOutputs,
		Agents:              agents,
		Seq:                 5,
		Now:                 fixedNow,
		Mode:                domain.ExecutionModeAutoReview,
		ArtifactRegistry:    registry,
	})

	step := requireDispatch(t, dec)
	// The creator's prior output artifact must appear in the dispatched
	// test-writer-tdd step's InputArtifacts (it is not a table row entry,
	// so deduplication does not suppress it).
	found := false
	for _, a := range step.Request.InputArtifacts {
		if a == "Stage-1/PlanProgress.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("creator-artifact injection: want %q in dispatched InputArtifacts %v",
			"Stage-1/PlanProgress.md", step.Request.InputArtifacts)
	}
}

// TestNext_CreatorArtifactInjection_RegistryAbsent_NoInjection verifies that
// when ArtifactRegistry is nil (first invocation of the creator agent), no
// creator-artifact injection occurs -- the dispatched step's InputArtifacts
// contain only the table row entries and the review agent's LastOutputArtifacts.
func TestNext_CreatorArtifactInjection_RegistryAbsent_NoInjection(t *testing.T) {
	aw := mustParseAndAdmitV216(t, creatorInjectionV216Content, "creator-injection", "1.0")
	schedules := singleSchedules("TDD")
	agents := newTestAgents("test-writer-tdd", "tests-review-tdd", "implementation-tdd")
	state := stateAfterRow("EXECUTION", "1.Test", "tests-review-tdd#5", domain.StatusCOMPLETED_NEEDS_ACTION, 5, 2)
	reviewOutputs := []string{"Stage-1/tests-review-tdd.md"}

	dec := engine.Next(engine.NextInput{
		Workflow:            aw,
		Schedules:           schedules,
		State:               state,
		LastResponse:        cnaResponse("tests-review-tdd#5"),
		LastOutputArtifacts: reviewOutputs,
		Agents:              agents,
		Seq:                 5,
		Now:                 fixedNow,
		Mode:                domain.ExecutionModeAutoReview,
		ArtifactRegistry:    nil, // first invocation: no prior outputs in registry
	})

	step := requireDispatch(t, dec)
	// Stage-1/PlanProgress.md is NOT in the table input for test-writer-tdd
	// and is NOT in reviewOutputs. With no registry, it must not appear.
	for _, a := range step.Request.InputArtifacts {
		if a == "Stage-1/PlanProgress.md" {
			t.Errorf("creator-artifact injection: no registry supplied, but %q appeared in InputArtifacts %v (spurious injection)",
				"Stage-1/PlanProgress.md", step.Request.InputArtifacts)
		}
	}
}

// TestNext_CreatorArtifactInjection_TemplatedOutput_MatchesResolvedPath verifies
// that creator-artifact injection fires correctly when the target row's declared
// output artifact contains a {Stage} fill-in. The comparison must use the
// resolved path (Stage-2/PlanProgress.md) rather than the raw template
// (Stage-{Stage}/PlanProgress.md), so a registry entry with the bare resolved
// path triggers injection.
func TestNext_CreatorArtifactInjection_TemplatedOutput_MatchesResolvedPath(t *testing.T) {
	aw := mustParseAndAdmitV216(t, creatorInjectionV216Content, "creator-injection", "1.0")
	// Stage 2: resolved output would be Stage-2/PlanProgress.md.
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "TDD"},
		domain.ScheduleIteration{ID: "2", HITL: false, Approach: "TDD"},
	)
	agents := newTestAgents("test-writer-tdd", "tests-review-tdd", "implementation-tdd")
	// State: tests-review-tdd completed for stage 2 (CNA).
	state := stateAfterRow("EXECUTION", "2.Test", "tests-review-tdd#7", domain.StatusCOMPLETED_NEEDS_ACTION, 7, 2)
	// Registry carries the bare resolved path for stage 2 (as session provides
	// after StripRunPrefix -- the resolved path, not the raw template).
	registry := []domain.ArtifactRegistryEntry{
		{Artifact: "Stage-2/PlanProgress.md", CreatedIn: "EXECUTION.2.Test", CreatedBy: "test-writer-tdd#5"},
	}
	reviewOutputs := []string{"Stage-2/tests-review-tdd.md"}

	dec := engine.Next(engine.NextInput{
		Workflow:            aw,
		Schedules:           schedules,
		State:               state,
		LastResponse:        cnaResponse("tests-review-tdd#7"),
		LastOutputArtifacts: reviewOutputs,
		Agents:              agents,
		Seq:                 7,
		Now:                 fixedNow,
		Mode:                domain.ExecutionModeAutoReview,
		ArtifactRegistry:    registry,
	})

	step := requireDispatch(t, dec)
	// Stage-2/PlanProgress.md must be injected (resolved path match).
	found := false
	for _, a := range step.Request.InputArtifacts {
		if a == "Stage-2/PlanProgress.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("templated-output creator injection: want %q in dispatched InputArtifacts %v",
			"Stage-2/PlanProgress.md", step.Request.InputArtifacts)
	}
}

// ===== "next" keyword routing (handleNonExecutionSuccess) =====

// nextKeywordNonExecContent is a two-row run-scope workflow where both rows
// carry On Success = NEXT. Tests derive two behaviors from this fixture:
//   - row 0 success with NEXT -> advance to row 1 (DispatchDecision, agent-b).
//   - row 1 success with NEXT -> past the last row -> CompleteDecision.
const nextKeywordNonExecContent = v216Columns +
	`| PLANNING | agent-a | FALSE | NEXT | - | - | out.md |
| PLANNING | agent-b | FALSE | NEXT | - | - | final.md |
`

// nextIntoExecutionContent is a workflow with a single PLANNING row using
// On Success = NEXT followed by a block. Used to verify that NEXT enters the
// block when the next row begins it, and that it returns StopDecision when no
// schedule is available (same behavior as naming the block with no schedule).
const nextIntoExecutionContent = v216Columns +
	`| PLANNING | planner | FALSE | NEXT | - | - | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| EXECUTION.[Stage] | implementation-tdd | FALSE | NEXT | - | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/PlanProgress.md |
| EXECUTION.[Stage] | implementation-review | FALSE | COMPLETE | implementation-tdd | Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md | Stage-{Stage}/implementation-review.md |
` + v216StageBlock

// TestNext_NextKeyword_AdvancesToNextRow verifies that a SUCCESS response from a
// run-scope row with On Success = NEXT dispatches the immediately following row
// by index rather than by agent-name lookup.
func TestNext_NextKeyword_AdvancesToNextRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, nextKeywordNonExecContent, "next-keyword-non-exec", "1.0")
	agents := newTestAgents("agent-a", "agent-b")
	// State: agent-a (row 0) just completed successfully.
	state := stateAfter("PLANNING", "", "agent-a#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		State:        state,
		LastResponse: successResponse("agent-a#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "agent-b" {
		t.Errorf("next keyword: want agent-b (row 1 by index), got %s", step.Request.AgentInstanceID)
	}
	if step.RowIndex != 1 {
		t.Errorf("next keyword: want RowIndex=1 (next row), got %d", step.RowIndex)
	}
}

// TestNext_NextKeyword_PastLastRow_ReturnsComplete verifies that a SUCCESS
// response from the last row in the workflow when On Success = NEXT returns
// CompleteDecision (same as On Success = COMPLETE).
func TestNext_NextKeyword_PastLastRow_ReturnsComplete(t *testing.T) {
	aw := mustParseAndAdmitV216(t, nextKeywordNonExecContent, "next-keyword-non-exec", "1.0")
	agents := newTestAgents("agent-a", "agent-b")
	// State: agent-b (row 1, the last row) just completed successfully.
	state := stateAfter("PLANNING", "", "agent-b#2", domain.StatusSUCCESS, 2)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		State:        state,
		LastResponse: successResponse("agent-b#2"),
		Agents:       agents,
		Seq:          2,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	requireComplete(t, dec)
}

// TestNext_NextKeyword_IntoExecutionRow_TriggersStageResolution verifies that
// On Success = NEXT from a run-scope row whose successor begins a block enters
// the block (same dispatch path as naming the block with [Stage]). The
// dispatched agent must be the first row of the block's first iteration.
func TestNext_NextKeyword_IntoExecutionRow_TriggersStageResolution(t *testing.T) {
	aw := mustParseAndAdmitV216(t, nextIntoExecutionContent, "next-into-execution", "1.0")
	schedules := singleSchedules("Implementation-Only")
	agents := newTestAgents("planner", "implementation-tdd", "implementation-review")
	// State: planner (row 0, PLANNING) just completed successfully.
	state := stateAfter("PLANNING", "", "planner#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: successResponse("planner#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	step := requireDispatch(t, dec)
	if agentName(step.Request.AgentInstanceID) != "implementation-tdd" {
		t.Errorf("next into EXECUTION: want implementation-tdd (first EXECUTION group row), got %s",
			step.Request.AgentInstanceID)
	}
	if step.Stage == "" {
		t.Error("next into EXECUTION: want non-empty Stage (stage/group context resolved), got empty")
	}
}

// TestNext_NextKeyword_IntoExecutionRow_NoSchedules_ReturnsStop verifies that
// On Success = NEXT into a block with no available schedule returns
// StopDecision with a non-empty reason. This is consistent with naming the
// block with [Stage] when no schedule is read: both yield StopDecision.
func TestNext_NextKeyword_IntoExecutionRow_NoSchedules_ReturnsStop(t *testing.T) {
	aw := mustParseAndAdmitV216(t, nextIntoExecutionContent, "next-into-execution", "1.0")
	agents := newTestAgents("planner", "implementation-tdd", "implementation-review")
	state := stateAfter("PLANNING", "", "planner#1", domain.StatusSUCCESS, 1)

	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		State:        state,
		LastResponse: successResponse("planner#1"),
		Agents:       agents,
		Seq:          1,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})

	stop := requireStop(t, dec)
	if stop.Reason == "" {
		t.Error("next into EXECUTION with nil schedules: want non-empty StopDecision.Reason, got empty")
	}
}

// ===== ResolveArtifactsForIteration: template expansion =====
//
// These tests live in the engine_test package (black-box) and cover the
// template-expansion behaviour of the exported artifact-path resolver.

// TestResolveArtifacts_FillIn_Substitution_InputArtifact verifies that
// {Stage} in an input artifact path is replaced with the current iteration ID.
func TestResolveArtifacts_FillIn_Substitution_InputArtifact(t *testing.T) {
	arts := []string{"Stage-{Stage}/Plan.md", "ContractsDesign.md"}
	schedules := singleSchedules("TDD")
	got, err := engine.ResolveArtifactsForIteration(arts, "Stage", "2", engine.ScheduleContext{Schedules: schedules}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Stage-2/Plan.md", "ContractsDesign.md"}
	if !stringSlicesEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveArtifacts_FillIn_Substitution_OutputArtifact verifies that
// {Stage} in an output artifact path is also replaced with the current
// iteration ID (same substitution rule for both directions).
func TestResolveArtifacts_FillIn_Substitution_OutputArtifact(t *testing.T) {
	arts := []string{"Stage-{Stage}/PlanProgress.md"}
	schedules := singleSchedules("TDD")
	got, err := engine.ResolveArtifactsForIteration(arts, "Stage", "3", engine.ScheduleContext{Schedules: schedules}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Stage-3/PlanProgress.md"}
	if !stringSlicesEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveArtifacts_StageWildcard_Input_PassedThroughForTheSession verifies
// that Stage-* in an input artifact path is passed through unexpanded whatever
// the schedules hold: the session expands wildcards, never the engine.
func TestResolveArtifacts_StageWildcard_Input_PassedThroughForTheSession(t *testing.T) {
	arts := []string{"Stage-*/Plan.md"}
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "TDD"},
		domain.ScheduleIteration{ID: "2", HITL: false, Approach: "TDD"},
		domain.ScheduleIteration{ID: "3", HITL: false, Approach: "TDD"},
	)
	got, err := engine.ResolveArtifactsForIteration(arts, "", "", engine.ScheduleContext{Schedules: schedules}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Stage-*/Plan.md"}
	if !stringSlicesEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveArtifacts_StageWildcard_Output_PassThrough verifies that Stage-*
// in an output artifact path is passed through unexpanded (output artifacts
// use the wildcard as a literal declaration, not a glob expansion).
func TestResolveArtifacts_StageWildcard_Output_PassThrough(t *testing.T) {
	arts := []string{"Stage-*/PlanProgress.md"}
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", HITL: false, Approach: "TDD"},
		domain.ScheduleIteration{ID: "2", HITL: false, Approach: "TDD"},
	)
	got, err := engine.ResolveArtifactsForIteration(arts, "Stage", "1", engine.ScheduleContext{Schedules: schedules}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Stage-*/PlanProgress.md"}
	if !stringSlicesEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveArtifacts_UnresolvablePlaceholder_ReturnsError verifies that
// {Stage} in an artifact path of a block row without an iteration returns an
// error rather than leaving the placeholder literal.
func TestResolveArtifacts_UnresolvablePlaceholder_ReturnsError(t *testing.T) {
	arts := []string{"Stage-{Stage}/Plan.md"}
	schedules := singleSchedules("TDD")
	// A block row with no iteration.
	_, err := engine.ResolveArtifactsForIteration(arts, "Stage", "", engine.ScheduleContext{Schedules: schedules}, true)
	if err == nil {
		t.Fatal("want error for {Stage} without an iteration, got nil")
	}
	if !strings.Contains(err.Error(), "{Stage}") {
		t.Errorf("error %q: want mention of the unresolvable placeholder {Stage}", err.Error())
	}
}

// stringSlicesEqual returns true when a and b have the same length and equal
// elements in the same order.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
