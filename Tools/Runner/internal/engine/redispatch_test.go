package engine_test

// Tests for engine.UnacceptedAttemptRedispatch: the step that re-dispatches the
// trailing unaccepted workflow attempt of a resumed run.
//
// Coverage:
//
//   - The step is the attempt's own row and iteration with the Inputs recorded
//     on its log entry; every other field equals a fresh dispatch of that row.
//   - Staged (iteration in a block) and unstaged rows; a trailing
//     infrastructure entry after the unaccepted row; a row recorded without a
//     workflow row (agent+phase fallback).
//   - The state is never modified.
//   - ok is false exactly when ResumePointFromSchedules reports no rerun.
//   - Errors: unresolvable position, unresolvable recorded iteration.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

func brownfieldAgents() map[string]domain.AgentReference {
	return newTestAgents("codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review", "test-runner")
}

func quickFixAgents() map[string]domain.AgentReference {
	return newTestAgents("planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")
}

// redispatchFor calls UnacceptedAttemptRedispatch with the checkpoint
// infrastructure agent declared.
func redispatchFor(
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	agents map[string]domain.AgentReference,
	state domain.ArtifactState,
	seq int,
) (domain.DispatchStep, bool, error) {
	return engine.UnacceptedAttemptRedispatch(engine.RedispatchInput{
		Workflow:  aw,
		Schedules: schedules,
		State:     state,
		Infra:     domain.NewInfraAgentSet(declaredCheckpointInfraAgent()),
		Agents:    agents,
		Seq:       seq,
	})
}

// requireRedispatch requires ok and no error.
func requireRedispatch(t *testing.T, step domain.DispatchStep, ok bool, err error) domain.DispatchStep {
	t.Helper()
	if err != nil {
		t.Fatalf("UnacceptedAttemptRedispatch: unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("UnacceptedAttemptRedispatch: want ok=true for a trailing unaccepted attempt, got ok=false")
	}
	return step
}

// freshStepAfter returns the step engine.Next dispatches after prev finished
// with SUCCESS in auto-review mode.
func freshStepAfter(
	t *testing.T,
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	agents map[string]domain.AgentReference,
	prev domain.ArtifactState,
	prevAgentID string,
	seq int,
) domain.DispatchStep {
	t.Helper()
	return requireDispatch(t, engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        prev,
		LastResponse: successResponse(prevAgentID),
		Agents:       agents,
		Seq:          seq,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	}))
}

// requireEqualExceptInputs requires got to equal fresh in every field but
// Request.InputArtifacts.
func requireEqualExceptInputs(t *testing.T, got, fresh domain.DispatchStep) {
	t.Helper()
	got.Request.InputArtifacts = nil
	fresh.Request.InputArtifacts = nil
	if !reflect.DeepEqual(got, fresh) {
		t.Errorf("re-dispatch step differs from a fresh dispatch of the same row beyond its inputs:\n got: %+v\nwant: %+v", got, fresh)
	}
}

func requireInputs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("InputArtifacts: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("InputArtifacts: want %v (recorded order), got %v", want, got)
		}
	}
}

// ---- the incident shape: HITL-rejected planner after a review with findings ----

func TestUnacceptedAttemptRedispatch_RouteBackTarget_RedispatchesItsRowNotTheNextOne(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := singleSchedules("TDD")
	agents := brownfieldAgents()
	// plan-review (row 5) returned findings and is the accepted position. The
	// planner (row 4) was routed back to, returned PARTIALLY_DONE without
	// approval, and its row is not accepted into current_state.
	state := domain.ArtifactState{
		GlobalSequence: 6,
		CurrentState: domain.CurrentState{
			Phase: "PLANNING", LastStatus: domain.StatusCOMPLETED_NEEDS_ACTION, LastAgent: "plan-review#5",
		},
		ExecutionLog: []domain.ExecutionLogEntry{
			execLogEntry(4, "planner-tdd-soft#4", "PLANNING", "", domain.StatusSUCCESS, 4),
			execLogEntry(5, "plan-review#5", "PLANNING", "", domain.StatusCOMPLETED_NEEDS_ACTION, 5),
			{
				Seq: 6, Agent: "planner-tdd-soft#6", Phase: "PLANNING", WorkflowRow: domain.WorkflowRow(4),
				Status: domain.StatusPARTIALLY_DONE, Inputs: "Research.md, Requirements.md, plan-review.md",
			},
		},
	}
	before := state
	before.ExecutionLog = append([]domain.ExecutionLogEntry(nil), state.ExecutionLog...)

	step, ok, err := redispatchFor(aw, schedules, agents, state, 6)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != 3 || got.Agent.Identifier != "planner-tdd-soft" {
		t.Errorf("want the planner at row index 3, got agent %q row index %d", got.Agent.Identifier, got.RowIndex)
	}
	if got.Request.AgentInstanceID != "planner-tdd-soft#7" {
		t.Errorf("want instance planner-tdd-soft#7 (Seq+1), got %q", got.Request.AgentInstanceID)
	}
	requireInputs(t, got.Request.InputArtifacts, "Research.md", "Requirements.md", "plan-review.md")
	if !got.EffectiveHITL || !got.Request.HumanInTheLoop {
		t.Error("want the row's HITL default (true) on the re-dispatch")
	}
	if got.Retry != domain.RetryNone {
		t.Errorf("want RetryNone, got %v", got.Retry)
	}
	fresh := freshStepAfter(t, aw, schedules, agents,
		stateAfterRow("RESEARCH", "", "requirements-review#3", domain.StatusSUCCESS, 3, 3), "requirements-review#3", 6)
	requireEqualExceptInputs(t, got, fresh)
	if !reflect.DeepEqual(state, before) {
		t.Error("UnacceptedAttemptRedispatch modified the state it was given")
	}
}

// ---- unstaged row, recorded inputs replace the row defaults ----

func TestUnacceptedAttemptRedispatch_UnstagedRow_UsesRecordedInputsAndRowDefaults(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	agents := quickFixAgents()
	state := stateWithLog(2,
		execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1),
	)
	unaccepted := execLogEntry(2, "plan-review#2", "PLANNING", "", domain.StatusSUCCESS, 2)
	unaccepted.Inputs = "Plan.md, Stage-1/Plan.md, extra-context.md"
	state.ExecutionLog = append(state.ExecutionLog, unaccepted)

	step, ok, err := redispatchFor(aw, schedules, agents, state, 2)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != 1 || got.Phase != "PLANNING" || got.Stage != "" {
		t.Errorf("want row 1 in PLANNING without iteration, got row %d phase %q stage %q", got.RowIndex, got.Phase, got.Stage)
	}
	requireInputs(t, got.Request.InputArtifacts, "Plan.md", "Stage-1/Plan.md", "extra-context.md")
	fresh := freshStepAfter(t, aw, schedules, agents,
		stateAfterRow("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1, 1), "planner-tdd-soft#1", 2)
	requireEqualExceptInputs(t, got, fresh)
}

func TestUnacceptedAttemptRedispatch_EmptyRecordedInputs_GivesEmptyInputList(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	state := stateWithLog(2,
		execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1),
	)
	state.ExecutionLog = append(state.ExecutionLog,
		execLogEntry(2, "plan-review#2", "PLANNING", "", domain.StatusSUCCESS, 2))

	step, ok, err := redispatchFor(aw, singleSchedules("Implementation-Only"), quickFixAgents(), state, 2)

	got := requireRedispatch(t, step, ok, err)
	if len(got.Request.InputArtifacts) != 0 {
		t.Errorf("want no inputs (none recorded, row defaults do not apply), got %v", got.Request.InputArtifacts)
	}
}

// ---- staged row: the iteration comes from the recorded entry ----

func TestUnacceptedAttemptRedispatch_StagedRow_KeepsRecordedIterationAndGroup(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	schedules := twoSchedules("TDD")
	agents := brownfieldAgents()
	state := stateWithLog(9,
		execLogEntry(8, "test-writer-tdd#8", "EXECUTION", "2.Test", domain.StatusSUCCESS, 8),
	)
	unaccepted := execLogEntry(9, "tests-review-tdd#9", "EXECUTION", "2.Test", domain.StatusSUCCESS, 9)
	unaccepted.Inputs = "Stage-2/Plan.md, ContractsDesign.md"
	state.ExecutionLog = append(state.ExecutionLog, unaccepted)

	step, ok, err := redispatchFor(aw, schedules, agents, state, 9)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != 8 || got.Phase != "EXECUTION" || got.Stage != "2.Test" {
		t.Errorf("want row 8, EXECUTION, iteration 2.Test, got row %d phase %q stage %q", got.RowIndex, got.Phase, got.Stage)
	}
	if got.Request.AgentInstanceID != "tests-review-tdd#10" {
		t.Errorf("want tests-review-tdd#10, got %q", got.Request.AgentInstanceID)
	}
	requireInputs(t, got.Request.InputArtifacts, "Stage-2/Plan.md", "ContractsDesign.md")
	if !containsString(got.Request.OutputArtifacts, "Stage-2/tests-review-tdd.md") {
		t.Errorf("want output resolved for iteration 2, got %v", got.Request.OutputArtifacts)
	}
	fresh := freshStepAfter(t, aw, schedules, agents,
		stateAfterRow("EXECUTION", "2.Test", "test-writer-tdd#8", domain.StatusSUCCESS, 9, 8), "test-writer-tdd#8", 9)
	requireEqualExceptInputs(t, got, fresh)
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ---- trailing infrastructure entry ----

func TestUnacceptedAttemptRedispatch_TrailingInfrastructureEntry_RedispatchesWorkflowRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	state := stateWithLog(3,
		execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1),
	)
	state.ExecutionLog = append(state.ExecutionLog,
		execLogEntry(2, "plan-review#2", "PLANNING", "", domain.StatusCOMPLETED_NEEDS_ACTION, 2),
		execLogEntry(3, "checkpoint-manager-git#3", "PLANNING", "", domain.StatusSUCCESS, 0),
	)

	step, ok, err := redispatchFor(aw, singleSchedules("Implementation-Only"), quickFixAgents(), state, 3)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != 1 || got.Agent.Identifier != "plan-review" {
		t.Errorf("want plan-review at row index 1, got %q at %d", got.Agent.Identifier, got.RowIndex)
	}
	if got.Request.AgentInstanceID != "plan-review#4" {
		t.Errorf("want plan-review#4 (after the infrastructure sequence), got %q", got.Request.AgentInstanceID)
	}
}

func TestUnacceptedAttemptRedispatch_EntryWithoutWorkflowRow_ResolvesByAgentAndPhase(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	state := stateWithLog(2,
		execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1),
	)
	state.ExecutionLog = append(state.ExecutionLog,
		execLogEntry(2, "plan-review#2", "PLANNING", "", domain.StatusSUCCESS, 0))

	step, ok, err := redispatchFor(aw, singleSchedules("Implementation-Only"), quickFixAgents(), state, 2)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != 1 {
		t.Errorf("want row index 1 from agent+phase, got %d", got.RowIndex)
	}
}

// ---- ok mirrors the resume position ----

func TestUnacceptedAttemptRedispatch_OkIffResumePointReportsRerun(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	planner := execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1)
	cases := map[string]domain.ArtifactState{
		"empty log":     {},
		"clean last":    stateWithLog(1, planner),
		"unaccepted":    withTrailing(stateWithLog(1, planner), execLogEntry(2, "plan-review#2", "PLANNING", "", domain.StatusSUCCESS, 2)),
		"infra only":    {GlobalSequence: 1, ExecutionLog: []domain.ExecutionLogEntry{execLogEntry(1, "checkpoint-manager-git#1", "", "", domain.StatusSUCCESS, 0)}},
		"accepted PD":   stateWithLog(1, execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusPARTIALLY_DONE, 1)),
		"accepted E501": stateWithLog(1, execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusBLOCKED, 1)),
	}
	for name, state := range cases {
		t.Run(name, func(t *testing.T) {
			info, perr := engine.ResumePointFromSchedules(aw, schedules, state,
				domain.NewInfraAgentSet(declaredCheckpointInfraAgent()))
			if perr != nil {
				t.Fatalf("ResumePointFromSchedules: %v", perr)
			}

			step, ok, err := redispatchFor(aw, schedules, quickFixAgents(), state, state.GlobalSequence)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name == "unaccepted" && !info.RerunLast {
				t.Fatalf("fixture must describe an unaccepted attempt (RerunLast=true), got false")
			}
			if ok != info.RerunLast {
				t.Errorf("ok=%v, but ResumePointFromSchedules RerunLast=%v", ok, info.RerunLast)
			}
			if !ok && !reflect.DeepEqual(step, domain.DispatchStep{}) {
				t.Errorf("ok=false must come with a zero step, got %+v", step)
			}
		})
	}
}

func withTrailing(state domain.ArtifactState, entries ...domain.ExecutionLogEntry) domain.ArtifactState {
	state.ExecutionLog = append(state.ExecutionLog, entries...)
	state.GlobalSequence = entries[len(entries)-1].Seq
	return state
}

// ---- errors ----

func TestUnacceptedAttemptRedispatch_UnknownTrailingAgent_ReportsPositionUnresolved(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	state := stateWithLog(2,
		execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1),
	)
	state.ExecutionLog = append(state.ExecutionLog,
		execLogEntry(2, "mystery-agent#2", "PLANNING", "", domain.StatusSUCCESS, 0))

	step, ok, err := redispatchFor(aw, singleSchedules("Implementation-Only"), quickFixAgents(), state, 2)

	var perr *domain.PositionUnresolvedError
	if !errors.As(err, &perr) {
		t.Fatalf("want *PositionUnresolvedError, got %v", err)
	}
	if ok || !reflect.DeepEqual(step, domain.DispatchStep{}) {
		t.Errorf("on error want ok=false and a zero step, got ok=%v step=%+v", ok, step)
	}
}

func TestUnacceptedAttemptRedispatch_RecordedIterationNotInSchedule_ReportsError(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	state := stateWithLog(9,
		execLogEntry(8, "test-writer-tdd#8", "EXECUTION", "1.Test", domain.StatusSUCCESS, 8),
	)
	state.ExecutionLog = append(state.ExecutionLog,
		execLogEntry(9, "tests-review-tdd#9", "EXECUTION", "7.Test", domain.StatusSUCCESS, 9))

	step, ok, err := redispatchFor(aw, singleSchedules("TDD"), brownfieldAgents(), state, 9)

	if err == nil {
		t.Fatal("want an error for a recorded iteration that is not in the schedule, got nil")
	}
	if ok || !reflect.DeepEqual(step, domain.DispatchStep{}) {
		t.Errorf("on error want ok=false and a zero step, got ok=%v step=%+v", ok, step)
	}
}
