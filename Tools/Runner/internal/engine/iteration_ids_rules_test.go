package engine_test

// Tests for the rules around iteration IDs in the engine: no selection of a
// schedule by block name, new schedule fields winning over the old stage-set
// fields, ID matching ignoring case, findings routing and error identification
// inside a non-numeric iteration.

import (
	"errors"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// namedBlockSchedules returns a one-block schedules value under the given
// block name with the given IDs, all with the same approach.
func namedBlockSchedules(block domain.BlockName, approach domain.Approach, ids ...domain.IterationID) domain.BlockSchedules {
	var its []domain.ScheduleIteration
	for _, id := range ids {
		its = append(its, domain.ScheduleIteration{ID: id, Approach: approach})
	}
	return domain.BlockSchedules{block: domain.BlockSchedule{
		Block: block, Source: "Plan.md", Section: "Stages", Iterations: its,
	}}
}

// planReviewInputs returns the input artifacts of the plan-review step of the
// quick-fix workflow, reached after planner-tdd-soft succeeded.
func planReviewInputs(t *testing.T, in engine.NextInput) []string {
	t.Helper()
	in.State = stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)
	in.LastResponse = successResponse("planner-tdd-soft#1")
	in.Seq = 1
	step := requireDispatch(t, engine.Next(in))
	if agentName(step.Request.AgentInstanceID) != "plan-review" {
		t.Fatalf("want plan-review, got %s", step.Request.AgentInstanceID)
	}
	return step.Request.InputArtifacts
}

func TestResolveArtifactsForIteration_SingleBlockUnderAnotherName_PassesStageWildcardThrough(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: namedBlockSchedules("Feature", "TDD", "A", "b-2", "C")}

	t.Run("row outside every block", func(t *testing.T) {
		got, err := engine.ResolveArtifactsForIteration([]string{"Stage-*/Plan.md"}, "", "", sc, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"Stage-*/Plan.md"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("row inside the block", func(t *testing.T) {
		got, err := engine.ResolveArtifactsForIteration(
			[]string{"Stage-*/Plan.md", "Stage-{Feature}/Out.md"}, "Feature", "b-2", sc, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"Stage-*/Plan.md", "Stage-b-2/Out.md"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestNext_SingleBlockUnderAnotherName_StageWildcardInputOf216Table_IsPassedThrough(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	in := walkInput(aw, namedBlockSchedules("Feature", "Implementation-Only", "A", "b-2", "C"),
		"planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")

	inputs := planReviewInputs(t, in)

	if !slices.Contains(inputs, "Stage-*/Plan.md") {
		t.Errorf("inputs %v: want the Stage-* entry passed through for the session to expand", inputs)
	}
	for _, unwanted := range []string{"Stage-A/Plan.md", "Stage-b-2/Plan.md", "Stage-C/Plan.md"} {
		if slices.Contains(inputs, unwanted) {
			t.Errorf("inputs %v: the engine must not expand the wildcard over the schedule, found %q", inputs, unwanted)
		}
	}
}

func TestNext_IDMatchingIgnoresCaseAndWritesTheScheduleSpelling(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	in := walkInput(aw, nonNumericSchedules("Implementation-Only"),
		"implementation-tdd", "implementation-review", "test-runner")
	// The log records the ID as "B-2"; the schedule spells it "b-2".
	in.State = resumeStateAfter(t, aw, "0:A", "1:A", "0:B-2", "1:B-2")
	in.LastResponse = successResponse("implementation-review#4")
	in.Seq = 4

	step := requireDispatch(t, engine.Next(in))

	if agentName(step.Request.AgentInstanceID) != "implementation-tdd" || step.Stage != "C" {
		t.Errorf("got %s at stage %q, want implementation-tdd at stage \"C\"",
			step.Request.AgentInstanceID, step.Stage)
	}
}

func TestResumePointFromSchedules_RecordedIDDiffersInCaseFromSchedule(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	schedules := nonNumericSchedules("Implementation-Only")

	t.Run("advanced row reports the schedule spelling", func(t *testing.T) {
		state := resumeStateAfter(t, aw, "0:A", "1:A", "0:B-2")
		info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.RowIndex != 1 || info.Iteration != "b-2" {
			t.Errorf("got row %d iteration %q, want row 1 iteration \"b-2\"", info.RowIndex, info.Iteration)
		}
	})

	t.Run("interrupted row reports the recorded spelling", func(t *testing.T) {
		state := resumeStateAfter(t, aw, "0:A", "1:A", "0:B-2", "1:B-2")
		state.CurrentState.LastAgent = "implementation-tdd#3"
		info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.RerunLast || info.RowIndex != 1 || info.Iteration != "B-2" {
			t.Errorf("got rerun=%v row %d iteration %q, want rerun row 1 iteration \"B-2\"",
				info.RerunLast, info.RowIndex, info.Iteration)
		}
	})
}

func TestNext_FindingsRouteBackInsideNonNumericIteration(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	in := walkInput(aw, nonNumericSchedules("Implementation-Only"),
		"implementation-tdd", "implementation-review", "test-runner")
	in.Mode = domain.ExecutionModeAutoReview
	in.State = stateAfterRow("EXECUTION", "b-2", "implementation-review#4",
		domain.StatusCOMPLETED_NEEDS_ACTION, 4, 2)
	in.LastResponse = cnaResponse("implementation-review#4")
	in.Seq = 4

	step := requireDispatch(t, engine.Next(in))

	if agentName(step.Request.AgentInstanceID) != "implementation-tdd" || step.Stage != "b-2" {
		t.Errorf("got %s at stage %q, want implementation-tdd at stage \"b-2\"",
			step.Request.AgentInstanceID, step.Stage)
	}
	if !slices.Contains(step.Request.InputArtifacts, "Stage-b-2/Plan.md") {
		t.Errorf("inputs %v lack Stage-b-2/Plan.md", step.Request.InputArtifacts)
	}
}

func TestResumePointFromSchedules_UnresolvableApproachErrorNamesTheNonNumericIteration(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.1")
	state := stateWithLog(9,
		execLogEntry(8, "test-writer-tdd#8", "EXECUTION", "b-2.Test", domain.StatusSUCCESS, bvRowTestWriter),
		execLogEntry(9, "build-review#9", "EXECUTION", "b-2.Test", domain.StatusSUCCESS, bvRowTestBuild),
	)

	_, err := engine.ResumePointFromSchedules(aw,
		namedBlockSchedules(nonNumericBlock, "UnknownApproach", "b-2"), state, nil)

	var uae *domain.UnresolvableApproachError
	if !errors.As(err, &uae) {
		t.Fatalf("err = %v, want an error wrapping *domain.UnresolvableApproachError", err)
	}
	if uae.Iteration != "b-2" {
		t.Errorf("Iteration = %q, want \"b-2\"", uae.Iteration)
	}
}

func TestResumePointFromSchedules_RowNotInGroupErrorNamesTheNonNumericIteration(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	// implementation-tdd is outside the only ordered group of a Tests-Only iteration.
	state := stateAfterRow("EXECUTION", "b-2.Implementation", "implementation-tdd#5", domain.StatusSUCCESS, 5, 10)

	_, err := engine.ResumePointFromSchedules(aw,
		namedBlockSchedules(nonNumericBlock, "Tests-Only", "b-2"), state, nil)

	var rnig *domain.RowNotInGroupError
	if !errors.As(err, &rnig) {
		t.Fatalf("err = %v, want an error wrapping *domain.RowNotInGroupError", err)
	}
	if rnig.Iteration != "b-2" {
		t.Errorf("Iteration = %q, want \"b-2\"", rnig.Iteration)
	}
}
