package engine_test

// Tests for the iteration-ID based engine API: a schedule whose IDs are not
// numbers (letters, digits, '-') runs every iteration in schedule order, with
// fill-ins, Stage-* expansion and recorded values carrying the IDs verbatim.

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// nonNumericBlock is the block of the fixtures in this file.
const nonNumericBlock domain.BlockName = "Stage"

// nonNumericSchedules returns one-block schedules (block Stage) with the IDs
// A, b-2 and C in that order, all with the given approach. HITL is set only
// for b-2.
func nonNumericSchedules(approach domain.Approach) domain.BlockSchedules {
	return domain.BlockSchedules{nonNumericBlock: domain.BlockSchedule{
		Block:   nonNumericBlock,
		Source:  "Plan.md",
		Section: "Stages",
		Iterations: []domain.ScheduleIteration{
			{ID: "A", Approach: approach},
			{ID: "b-2", HITL: true, Approach: approach},
			{ID: "C", Approach: approach},
		},
	}}
}

// walkedStep is one dispatched step observed while walking a run.
type walkedStep struct {
	agent  string
	stage  string
	inputs []string
	outs   []string
	hitl   bool
}

// walkRun drives Next with every dispatched step answered SUCCESS until the
// run completes, and returns the dispatched steps in order. It fails the test
// on any non-dispatch, non-complete decision or after a runaway number of steps.
func walkRun(t *testing.T, in engine.NextInput, firstSeq int) []walkedStep {
	t.Helper()
	var steps []walkedStep
	var log []domain.ExecutionLogEntry
	seq := firstSeq
	for i := 0; i < 40; i++ {
		dec := engine.Next(in)
		if dec.Complete != nil {
			return steps
		}
		step := requireDispatch(t, dec)
		steps = append(steps, walkedStep{
			agent:  agentName(step.Request.AgentInstanceID),
			stage:  step.Stage,
			inputs: step.Request.InputArtifacts,
			outs:   step.Request.OutputArtifacts,
			hitl:   step.EffectiveHITL,
		})
		seq++
		id := fmt.Sprintf("%s#%d", agentName(step.Request.AgentInstanceID), seq)
		// Iterations that completed are read from the log, so it accumulates.
		log = append(log, execLogEntry(seq, id, step.Phase, step.Stage, domain.StatusSUCCESS, step.RowIndex+1))
		in.State = stateWithLog(seq, log...)
		in.LastResponse = successResponse(id)
		in.Seq = seq
	}
	t.Fatalf("run did not complete within 40 steps; steps so far: %+v", steps)
	return nil
}

func walkInput(aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, agents ...string) engine.NextInput {
	return engine.NextInput{
		Workflow:  aw,
		Schedules: schedules,
		State:     emptyState(),
		Agents:    newTestAgents(agents...),
		Now:       fixedNow,
		Mode:      domain.ExecutionModeAuto,
	}
}

func TestNext_NonNumericIDs_UngroupedRunsEveryIterationInScheduleOrder(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	in := walkInput(aw, nonNumericSchedules("Implementation-Only"),
		"implementation-tdd", "implementation-review", "test-runner")

	steps := walkRun(t, in, 0)

	wantStages := []string{"A", "A", "b-2", "b-2", "C", "C", ""}
	wantAgents := []string{
		"implementation-tdd", "implementation-review",
		"implementation-tdd", "implementation-review",
		"implementation-tdd", "implementation-review",
		"test-runner",
	}
	if len(steps) != len(wantStages) {
		t.Fatalf("want %d steps, got %d: %+v", len(wantStages), len(steps), steps)
	}
	for i, s := range steps {
		if s.stage != wantStages[i] || s.agent != wantAgents[i] {
			t.Errorf("step %d: want %s at stage %q, got %s at stage %q",
				i, wantAgents[i], wantStages[i], s.agent, s.stage)
		}
	}
}

func TestNext_NonNumericIDs_FillInsCarryIDVerbatimAndHITLComesFromSchedule(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	in := walkInput(aw, nonNumericSchedules("Implementation-Only"),
		"implementation-tdd", "implementation-review", "test-runner")

	steps := walkRun(t, in, 0)

	// Step 2 is implementation-tdd of iteration b-2.
	got := steps[2]
	if !slices.Contains(got.inputs, "Stage-b-2/Plan.md") {
		t.Errorf("inputs of iteration b-2 = %v, want Stage-b-2/Plan.md", got.inputs)
	}
	if !slices.Contains(got.outs, "Stage-b-2/PlanProgress.md") {
		t.Errorf("outputs of iteration b-2 = %v, want Stage-b-2/PlanProgress.md", got.outs)
	}
	for i, s := range steps[:6] {
		wantHITL := s.stage == "b-2"
		if s.hitl != wantHITL {
			t.Errorf("step %d (stage %q): EffectiveHITL = %v, want %v", i, s.stage, s.hitl, wantHITL)
		}
	}
}

func TestNext_NonNumericIDs_GroupedRecordsGroupQualifiedIDsInScheduleOrder(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	in := walkInput(aw, nonNumericSchedules("TDD"),
		"codebase-research", "requirements-refinement", "requirements-review",
		"planner-tdd-soft", "plan-review", "contracts-designer", "contracts-review",
		"test-writer-tdd", "tests-review-tdd", "implementation-tdd",
		"implementation-review", "test-runner")
	in.State = stateAfter("DESIGN", "", "contracts-review#7", domain.StatusSUCCESS, 7)
	in.LastResponse = successResponse("contracts-review#7")
	in.Seq = 7

	steps := walkRun(t, in, 7)

	var staged []string
	for _, s := range steps {
		if s.stage == "" {
			continue
		}
		if len(staged) == 0 || staged[len(staged)-1] != s.stage {
			staged = append(staged, s.stage)
		}
	}
	want := []string{
		"A.Test", "A.Implementation",
		"b-2.Test", "b-2.Implementation",
		"C.Test", "C.Implementation",
	}
	if !slices.Equal(staged, want) {
		t.Errorf("recorded stage values in order = %v, want %v", staged, want)
	}
	if last := steps[len(steps)-1]; last.agent != "test-runner" || last.stage != "" {
		t.Errorf("last step = %+v, want test-runner outside the staged phase", last)
	}
}

func TestNext_NonNumericIDs_StageWildcardInputOf216Table_IsPassedThroughForTheSession(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	in := walkInput(aw, nonNumericSchedules("Implementation-Only"),
		"planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")
	in.State = stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)
	in.LastResponse = successResponse("planner-tdd-soft#1")
	in.Seq = 1

	step := requireDispatch(t, engine.Next(in))

	if agentName(step.Request.AgentInstanceID) != "plan-review" {
		t.Fatalf("want plan-review, got %s", step.Request.AgentInstanceID)
	}
	for _, want := range []string{"Stage-*/Plan.md", "Stage-*/PlanProgress.md"} {
		if !slices.Contains(step.Request.InputArtifacts, want) {
			t.Errorf("inputs %v: want %q passed through for the session to expand", step.Request.InputArtifacts, want)
		}
	}
	for _, p := range step.Request.InputArtifacts {
		if p == "Stage-A/Plan.md" || p == "Stage-b-2/Plan.md" || p == "Stage-C/Plan.md" {
			t.Errorf("inputs %v: the engine must not expand the wildcard over the schedule, found %q",
				step.Request.InputArtifacts, p)
		}
	}
}

func TestNext_TwoBlockSchedules_StageWildcardInputPassesThroughUnexpanded(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := nonNumericSchedules("Implementation-Only")
	schedules["Other"] = domain.BlockSchedule{
		Block:      "Other",
		Source:     "Other.md",
		Section:    "Others",
		Iterations: []domain.ScheduleIteration{{ID: "X"}},
	}
	in := walkInput(aw, schedules, "planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")
	in.State = stateAfter("PLANNING", "", "planner-tdd-soft#1", domain.StatusSUCCESS, 1)
	in.LastResponse = successResponse("planner-tdd-soft#1")
	in.Seq = 1

	step := requireDispatch(t, engine.Next(in))

	if !slices.Contains(step.Request.InputArtifacts, "Stage-*/Plan.md") {
		t.Errorf("inputs %v: want Stage-*/Plan.md passed through", step.Request.InputArtifacts)
	}
	for _, p := range step.Request.InputArtifacts {
		if p == "Stage-A/Plan.md" || p == "Stage-X/Plan.md" {
			t.Errorf("inputs %v: Stage-* must not be expanded for two blocks, found %q",
				step.Request.InputArtifacts, p)
		}
	}
}

// Every wildcard passes through for the session to expand; only fill-ins of
// the row's own block are replaced.
func TestResolveArtifactsForIteration_Wildcards(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: nonNumericSchedules("Implementation-Only")}
	arts := []string{"Stage-*/Plan.md", "*-Research.md", "*/Notes.md", "Stage-{Stage}/Out.md"}

	t.Run("input passes every wildcard through and replaces the fill-in", func(t *testing.T) {
		got, err := engine.ResolveArtifactsForIteration(arts, nonNumericBlock, "b-2", sc, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"Stage-*/Plan.md", "*-Research.md", "*/Notes.md", "Stage-b-2/Out.md"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("output passes Stage-* through", func(t *testing.T) {
		got, err := engine.ResolveArtifactsForIteration(arts, nonNumericBlock, "C", sc, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"Stage-*/Plan.md", "*-Research.md", "*/Notes.md", "Stage-C/Out.md"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("two blocks pass Stage-* through unexpanded", func(t *testing.T) {
		two := nonNumericSchedules("Implementation-Only")
		two["Other"] = domain.BlockSchedule{Block: "Other", Iterations: []domain.ScheduleIteration{{ID: "X"}}}
		got, err := engine.ResolveArtifactsForIteration(
			[]string{"Stage-*/Plan.md"}, "", "", engine.ScheduleContext{Schedules: two}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.Equal(got, []string{"Stage-*/Plan.md"}) {
			t.Errorf("got %v, want the entry passed through", got)
		}
	})

	t.Run("single block with no iterations passes Stage-* through", func(t *testing.T) {
		empty := domain.BlockSchedules{nonNumericBlock: {Block: nonNumericBlock}}
		got, err := engine.ResolveArtifactsForIteration(
			[]string{"Stage-*/Plan.md"}, "", "", engine.ScheduleContext{Schedules: empty}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.Equal(got, []string{"Stage-*/Plan.md"}) {
			t.Errorf("got %v, want the entry passed through", got)
		}
	})

	t.Run("no schedules at all passes Stage-* through", func(t *testing.T) {
		got, err := engine.ResolveArtifactsForIteration(
			[]string{"Stage-*/Plan.md"}, "", "", engine.ScheduleContext{}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.Equal(got, []string{"Stage-*/Plan.md"}) {
			t.Errorf("got %v, want the entry passed through", got)
		}
	})
}

func TestResolveRowDefaultsForIteration_NonNumericIDs(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	row := aw.Table.Rows[0]
	sc := engine.ScheduleContext{Schedules: nonNumericSchedules("Implementation-Only")}

	t.Run("HITL and paths come from the named iteration", func(t *testing.T) {
		got, err := engine.ResolveRowDefaultsForIteration(row, "b-2", sc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.HITL {
			t.Error("HITL = false, want true from iteration b-2")
		}
		if !slices.Contains(got.InputArtifacts, "Stage-b-2/Plan.md") {
			t.Errorf("inputs %v lack Stage-b-2/Plan.md", got.InputArtifacts)
		}
		if !slices.Contains(got.OutputArtifacts, "Stage-b-2/PlanProgress.md") {
			t.Errorf("outputs %v lack Stage-b-2/PlanProgress.md", got.OutputArtifacts)
		}
	})

	t.Run("iteration without HITL gives false", func(t *testing.T) {
		got, err := engine.ResolveRowDefaultsForIteration(row, "A", sc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.HITL {
			t.Error("HITL = true, want false for iteration A")
		}
	})

	t.Run("ID matches ignoring case and is written as passed", func(t *testing.T) {
		got, err := engine.ResolveRowDefaultsForIteration(row, "B-2", sc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.HITL {
			t.Error("HITL = false, want true: B-2 equals b-2 ignoring case")
		}
		if !slices.Contains(got.InputArtifacts, "Stage-B-2/Plan.md") {
			t.Errorf("inputs %v lack Stage-B-2/Plan.md (ID as passed)", got.InputArtifacts)
		}
	})

	t.Run("unknown ID is ErrRowDefaultsStageNotInSet", func(t *testing.T) {
		_, err := engine.ResolveRowDefaultsForIteration(row, "z", sc)
		if !errors.Is(err, engine.ErrRowDefaultsStageNotInSet) {
			t.Errorf("err = %v, want ErrRowDefaultsStageNotInSet", err)
		}
	})

	t.Run("block row without an ID is ErrRowDefaultsStageRequired", func(t *testing.T) {
		_, err := engine.ResolveRowDefaultsForIteration(row, "", sc)
		if !errors.Is(err, engine.ErrRowDefaultsStageRequired) {
			t.Errorf("err = %v, want ErrRowDefaultsStageRequired", err)
		}
	})

	t.Run("row outside every block with an ID is ErrRowDefaultsStageOnNonStagedRow", func(t *testing.T) {
		_, err := engine.ResolveRowDefaultsForIteration(aw.Table.Rows[2], "A", sc)
		if !errors.Is(err, engine.ErrRowDefaultsStageOnNonStagedRow) {
			t.Errorf("err = %v, want ErrRowDefaultsStageOnNonStagedRow", err)
		}
	})
}

func TestBoundaries_NonNumericIDs(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	schedules := nonNumericSchedules("Implementation-Only")
	steps := bndPlay(t, aw, domain.ExecutionModeAuto, schedules)

	iterations := bndIterationEnds(aw, schedules, domain.ExecutionModeAuto, steps)
	phases := bndPhaseEnds(aw, schedules, domain.ExecutionModeAuto, steps)

	// An iteration ends after its last block row, whatever its ID; the row
	// outside every block ends none. The phase ends after the last row of the
	// last iteration to run, where the next row is of another phase.
	requireEnds(t, "iteration end", iterations, []string{"1:A", "1:b-2", "1:C"})
	requireEnds(t, "phase end", phases, []string{"1:C", "2:"})
}

// resumeStateAfter is the artifact state after the steps ("row:ID" keys) ran,
// each logged with its recorded row.
func resumeStateAfter(t *testing.T, aw domain.AdmittedWorkflow, keys ...string) domain.ArtifactState {
	t.Helper()
	return stateWithLog(len(keys), bndLog(aw, bndSteps(t, keys...))...)
}

func TestResumePointFromSchedules_NonNumericIDs(t *testing.T) {
	aw := mustParseAndAdmitV216(t, implOnlyV216Content, "implementation-only", "3.1")
	schedules := nonNumericSchedules("Implementation-Only")

	t.Run("after the last row of an iteration resumes at the next ID in schedule order", func(t *testing.T) {
		state := resumeStateAfter(t, aw, "0:A", "1:A")
		info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.RowIndex != 0 || info.Iteration != "b-2" || info.Block != nonNumericBlock {
			t.Errorf("got row %d iteration %q block %q, want row 0 iteration b-2 block %q",
				info.RowIndex, info.Iteration, info.Block, nonNumericBlock)
		}
		if info.RerunLast {
			t.Error("RerunLast = true, want false")
		}
	})

	t.Run("after the last iteration resumes outside the block", func(t *testing.T) {
		state := resumeStateAfter(t, aw, "0:A", "1:A", "0:b-2", "1:b-2", "0:C", "1:C")
		info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.RowIndex != 2 || info.Iteration != "" {
			t.Errorf("got row %d iteration %q, want row 2 iteration \"\"", info.RowIndex, info.Iteration)
		}
	})

	t.Run("mid-iteration advance stays in the same ID", func(t *testing.T) {
		state := resumeStateAfter(t, aw, "0:A", "1:A", "0:b-2")
		info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.RowIndex != 1 || info.Iteration != "b-2" {
			t.Errorf("got row %d iteration %q, want row 1 iteration b-2", info.RowIndex, info.Iteration)
		}
	})

	t.Run("interrupted row reports the recorded ID verbatim", func(t *testing.T) {
		state := resumeStateAfter(t, aw, "0:A", "1:A", "0:b-2", "1:b-2")
		state.CurrentState.LastAgent = "implementation-tdd#3"
		info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.RerunLast || info.RowIndex != 1 || info.Iteration != "b-2" {
			t.Errorf("got rerun=%v row %d iteration %q, want rerun row 1 iteration b-2",
				info.RerunLast, info.RowIndex, info.Iteration)
		}
	})
}
