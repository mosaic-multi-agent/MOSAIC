package engine_test

// Iteration selection and block re-entry on WorkflowFormat 2.16 tables in
// Modes 2 and 3. The next iteration to run is the first one in schedule order
// that has not completed and whose Depends On iterations have all completed.
// Iterations run one at a time. Entering a block again runs only the
// iterations that have not completed; completion is derived from the
// Execution Log the run has so far.
//
// The dependency-order schedules are built as values: the schedule reader
// refuses a Depends On naming a later row, so they cannot be read from a file.

import (
	"fmt"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ---- Fixtures ----

// itBuildRows is a planner, a two-row ungrouped block and a final review:
//
//	0 planner  1 worker  2 checker (findings -> worker)  3 final-review (findings -> planner)
func itBuildRows() []scopeRow {
	return []scopeRow{
		bsPlanner(),
		bsBuild("worker", ""),
		{phase: "BUILD.[Stage]", agent: "checker", onFindings: "worker"},
		{phase: "REVIEW", agent: "final-review", onFindings: "planner"},
	}
}

// itIteration builds a schedule iteration with dependencies.
func itIteration(id domain.IterationID, dependsOn ...domain.IterationID) domain.ScheduleIteration {
	return domain.ScheduleIteration{ID: id, DependsOn: dependsOn}
}

// itOrder plays the block rows of itBuildRows over the given iterations and
// returns the trace of the run: every dispatched row with its recorded stage.
func itOrder(t *testing.T, mode domain.ExecutionMode, its ...domain.ScheduleIteration) scopeTrace {
	t.Helper()
	aw := mustAdmitScope(t, itBuildRows()[:3], stageBlockTables(), mode)
	return scopeRun{aw: aw, schedules: scheduleOf("Stage", its...), mode: mode}.play(t)
}

// requireIterationOrder asserts that the block's iterations ran in the given
// order, each one finishing both of its rows before the next one began.
func requireIterationOrder(t *testing.T, tr scopeTrace, order ...string) {
	t.Helper()
	var wantRows []int
	var wantStages []string
	wantRows = append(wantRows, 0)
	wantStages = append(wantStages, "")
	for _, id := range order {
		wantRows = append(wantRows, 1, 2)
		wantStages = append(wantStages, id, id)
	}
	if got := tr.rows(); !reflect.DeepEqual(got, wantRows) {
		t.Errorf("dispatched rows: want %v, got %v (run ended with %s)", wantRows, got, tr.describe())
	}
	if got := tr.stages(); !reflect.DeepEqual(got, wantStages) {
		t.Errorf("iteration order: want %q, got %q", wantStages, got)
	}
	if tr.final.Complete == nil {
		t.Errorf("run must end with a CompleteDecision, got %s", tr.describe())
	}
}

// itFindingsAt scripts COMPLETED_NEEDS_ACTION once for each "row:stage" spot
// named, and SUCCESS for every other step.
func itFindingsAt(spots ...string) func(int, domain.DispatchStep) domain.StatusCode {
	remaining := map[string]bool{}
	for _, s := range spots {
		remaining[s] = true
	}
	return func(_ int, step domain.DispatchStep) domain.StatusCode {
		key := fmt.Sprintf("%d:%s", step.RowIndex, step.Stage)
		if remaining[key] {
			delete(remaining, key)
			return domain.StatusCOMPLETED_NEEDS_ACTION
		}
		return domain.StatusSUCCESS
	}
}

// itReplanned hands the Next calls `before` until the first row has run a
// second time, and `after` from then on: the schedule as the re-planning row
// left it.
func itReplanned(before, after domain.BlockSchedules) func([]domain.DispatchStep) domain.BlockSchedules {
	return func(steps []domain.DispatchStep) domain.BlockSchedules {
		runs := 0
		for _, s := range steps {
			if s.RowIndex == 0 {
				runs++
			}
		}
		if runs >= 2 {
			return after
		}
		return before
	}
}

// ---- Iteration order ----

func TestIterations_DependsOnLaterIteration_RunsDependencyFirst(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		trace := itOrder(t, mode, itIteration("A", "C"), itIteration("B"), itIteration("C"))

		requireIterationOrder(t, trace, "B", "C", "A")
	})
}

func TestIterations_WithoutDependsOn_RunInScheduleOrder(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		trace := itOrder(t, mode, itIteration("X"), itIteration("M"), itIteration("B"))

		requireIterationOrder(t, trace, "X", "M", "B")
	})
}

func TestIterations_FirstRunnableInScheduleOrderIsChosenAfterEveryCompletion(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		trace := itOrder(t, mode, itIteration("A", "C"), itIteration("B"), itIteration("C"), itIteration("D"))

		// Once C completes, A (earlier in the schedule than D) is runnable
		// and runs before D.
		requireIterationOrder(t, trace, "B", "C", "A", "D")
	})
}

func TestIterations_SeveralDependencies_AllMustHaveCompleted(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		trace := itOrder(t, mode, itIteration("A", "B", "C"), itIteration("B", "C"), itIteration("C"))

		requireIterationOrder(t, trace, "C", "B", "A")
	})
}

func TestIterations_DependsOnIdIsMatchedIgnoringCase(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		trace := itOrder(t, mode, itIteration("A", "c"), itIteration("B"), itIteration("C"))

		requireIterationOrder(t, trace, "B", "C", "A")
	})
}

func TestIterations_IterationsNeverInterleave(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, itBuildRows()[:3], stageBlockTables(), mode)
		run := scopeRun{
			aw:        aw,
			schedules: scheduleOf("Stage", itIteration("A", "C"), itIteration("B"), itIteration("C")),
			mode:      mode,
		}

		trace := run.play(t)

		// play requires exactly one step in every dispatch decision; the
		// iterations must also never interleave.
		seen := map[string]bool{}
		prev := ""
		for _, st := range trace.stages() {
			if st != prev && seen[st] {
				t.Fatalf("iteration %q resumed after another one began: stages %q", st, trace.stages())
			}
			seen[st] = true
			prev = st
		}
	})
}

// ---- Re-entry ----

func TestIterations_ReEntry_OnlyIterationAddedByReplanRuns(t *testing.T) {
	aw := mustAdmitScope(t, itBuildRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw:   aw,
		mode: domain.ExecutionModeAutoReview,
		// A route-back inside iteration 1, then findings from the final
		// review that send the run back to the planner.
		statusOf: itFindingsAt("2:1", "3:"),
		schedulesAt: itReplanned(
			scheduleOf("Stage", idsOf("", "1", "2")...),
			scheduleOf("Stage", idsOf("", "1", "2", "3")...)),
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2, 1, 2, 3, 0, 1, 2, 3})
	requireStages(t, trace, []string{"", "1", "1", "1", "1", "2", "2", "", "", "3", "3", ""})
}

func TestIterations_ReEntry_AllIterationsCompleted_BlockExitsAtOnce(t *testing.T) {
	aw := mustAdmitScope(t, itBuildRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw:        aw,
		schedules: scheduleOf("Stage", idsOf("", "1", "2")...),
		mode:      domain.ExecutionModeAutoReview,
		statusOf:  itFindingsAt("2:1", "3:"),
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2, 1, 2, 3, 0, 3})
	requireStages(t, trace, []string{"", "1", "1", "1", "1", "2", "2", "", "", ""})
}

func TestIterations_ReEntry_NewIterationAheadOfCompletedOnesRunsAloneAfterItsDependency(t *testing.T) {
	aw := mustAdmitScope(t, itBuildRows(), stageBlockTables(), domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw:       aw,
		mode:     domain.ExecutionModeAutoReview,
		statusOf: itFindingsAt("3:"),
		schedulesAt: itReplanned(
			scheduleOf("Stage", idsOf("", "1", "2")...),
			scheduleOf("Stage", itIteration("3", "2"), itIteration("1"), itIteration("2"))),
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2, 3, 0, 1, 2, 3})
	requireStages(t, trace, []string{"", "1", "1", "2", "2", "", "", "3", "3", ""})
}

// Completion is "the last listed group's scope ended with SUCCESS", so an
// iteration of a one-group approach is complete without the other group having
// run.
func TestIterations_ReEntry_GroupedIterationsCompleteByTheirLastListedGroup(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeAutoReview)
	first := scheduleOf("Stage",
		domain.ScheduleIteration{ID: "1", Approach: "TDD"},
		domain.ScheduleIteration{ID: "2", Approach: "Tests-Only"},
		domain.ScheduleIteration{ID: "3", Approach: "Implementation-First"})
	replanned := scheduleOf("Stage",
		domain.ScheduleIteration{ID: "1", Approach: "TDD"},
		domain.ScheduleIteration{ID: "2", Approach: "Tests-Only"},
		domain.ScheduleIteration{ID: "3", Approach: "Implementation-First"},
		domain.ScheduleIteration{ID: "4", Approach: "Implementation-Only"})
	run := scopeRun{
		aw:          aw,
		mode:        domain.ExecutionModeAutoReview,
		statusOf:    itFindingsAt("5:"),
		schedulesAt: itReplanned(first, replanned),
	}
	firstRows, firstStages := bsExpectation([]string{"1", "2", "3"},
		[][]int{bsApproachRows["TDD"], bsApproachRows["Tests-Only"], bsApproachRows["Implementation-First"]})
	againRows, againStages := bsExpectation([]string{"4"}, [][]int{bsApproachRows["Implementation-Only"]})

	trace := run.play(t)

	requireRowsThenComplete(t, trace, append(firstRows, againRows...))
	requireStages(t, trace, append(firstStages, againStages...))
}

// ---- Re-entry of an iteration that has not finished ----

// itLogged is a dispatched step with the status its Execution Log entry carries.
type itLogged struct {
	step   domain.DispatchStep
	status domain.StatusCode
}

// itPlayed plays the block rows of aw over the given iterations with every
// step answering SUCCESS and returns the steps in dispatch order.
func itPlayed(t *testing.T, aw domain.AdmittedWorkflow, mode domain.ExecutionMode, its ...domain.ScheduleIteration) []domain.DispatchStep {
	t.Helper()
	return scopeRun{aw: aw, schedules: scheduleOf("Stage", its...), mode: mode}.play(t).steps
}

// itLoggedSteps records each step as SUCCESS.
func itLoggedSteps(steps []domain.DispatchStep) []itLogged {
	out := make([]itLogged, 0, len(steps))
	for _, s := range steps {
		out = append(out, itLogged{step: s, status: domain.StatusSUCCESS})
	}
	return out
}

// itDecisionAfter hands engine.Next a hand-built Execution Log made of the
// given entries, whose last one is the response just received, and returns the
// decision.
func itDecisionAfter(aw domain.AdmittedWorkflow, mode domain.ExecutionMode,
	schedules domain.BlockSchedules, logged []itLogged) domain.EngineDecision {
	var log []domain.ExecutionLogEntry
	for i, l := range logged {
		agent := fmt.Sprintf("%s#%d", aw.Table.Rows[l.step.RowIndex].BareAgentID(), i+1)
		log = append(log, execLogEntry(i+1, agent, l.step.Phase, l.step.Stage, l.status, l.step.RowIndex+1))
	}
	last := log[len(log)-1]
	return engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        stateWithLog(len(log), log...),
		LastResponse: &domain.ProtocolResponse{AgentInstanceID: last.Agent, StatusCode: last.Status, StatusMessage: "scripted"},
		Agents:       scopeAgents(aw),
		Seq:          len(log),
		Now:          fixedNow,
		Mode:         mode,
	})
}

// itNextAfter returns the step engine.Next dispatches after the given log. The
// run ending or deviating instead fails the test.
func itNextAfter(t *testing.T, aw domain.AdmittedWorkflow, mode domain.ExecutionMode,
	schedules domain.BlockSchedules, logged []itLogged) domain.DispatchStep {
	t.Helper()
	dec := itDecisionAfter(aw, mode, schedules, logged)
	if dec.Dispatch == nil {
		t.Fatalf("an iteration that has not finished must be run again, but the engine decided %s",
			scopeTrace{final: dec}.describe())
	}
	return requireDispatch(t, dec)
}

// itFinishedStepAfter returns the step the engine names in its finished-work
// deviation after the given log: the iteration that has not completed runs
// again, and its first step already has a SUCCESS, so the engine consults
// instead of dispatching it. Anything else fails the test.
func itFinishedStepAfter(t *testing.T, aw domain.AdmittedWorkflow, mode domain.ExecutionMode,
	schedules domain.BlockSchedules, logged []itLogged) domain.DispatchStep {
	t.Helper()
	dec := itDecisionAfter(aw, mode, schedules, logged)
	if dec.Deviation == nil || dec.Deviation.Info.Kind != domain.DeviationFinishedWork ||
		dec.Deviation.Info.PlannedContinuation == nil {
		t.Fatalf("an iteration that has not finished must be run again and its finished step named in a "+
			"finished-work deviation, but the engine decided %s", scopeTrace{final: dec}.describe())
	}
	return *dec.Deviation.Info.PlannedContinuation
}

// An iteration whose Test group ended but whose Implementation group never ran
// has not completed, whatever else the log holds: the block is not left.
func TestIterations_ReEntry_GroupedIterationWithUnrunLastGroupRunsAgain(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), mode)
		firstOnly := itPlayed(t, aw, mode, domain.ScheduleIteration{ID: "1", Approach: "TDD"})
		secondOnly := itPlayed(t, aw, mode, domain.ScheduleIteration{ID: "2", Approach: "TDD"})
		// Planning row, iteration 1 up to the end of its Test group (rows 1
		// and 2), then iteration 2 in full (rows 1 to 4).
		steps := append(append([]domain.DispatchStep{}, firstOnly[:3]...), secondOnly[1:5]...)
		schedules := scheduleOf("Stage",
			domain.ScheduleIteration{ID: "1", Approach: "TDD"},
			domain.ScheduleIteration{ID: "2", Approach: "TDD"})

		step := itFinishedStepAfter(t, aw, mode, schedules, itLoggedSteps(steps))

		// Iteration 1 resumes at its Implementation group (row 3) or starts
		// over at its Test group (row 1); never another iteration or the
		// rows after the block.
		okRemaining := step.RowIndex == 3 && step.Stage == bsTddShapeStage(3, "1")
		okFromStart := step.RowIndex == 1 && step.Stage == bsTddShapeStage(1, "1")
		if !okRemaining && !okFromStart {
			t.Errorf("want iteration 1 at row 3 (%q) or row 1 (%q), got row %d stage %q",
				bsTddShapeStage(3, "1"), bsTddShapeStage(1, "1"), step.RowIndex, step.Stage)
		}
	})
}

// An ungrouped iteration whose last row recorded findings and never ran again
// has not completed; the next iteration having completed does not change that.
func TestIterations_ReEntry_UngroupedIterationEndingOnFindingsRunsAgain(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, itBuildRows()[:3], stageBlockTables(), mode)
		first := itLoggedSteps(itPlayed(t, aw, mode, itIteration("1")))
		first[len(first)-1].status = domain.StatusCOMPLETED_NEEDS_ACTION
		second := itLoggedSteps(itPlayed(t, aw, mode, itIteration("2")))
		logged := append(first, second[1:]...)

		step := itFinishedStepAfter(t, aw, mode, scheduleOf("Stage", itIteration("1"), itIteration("2")), logged)

		if step.RowIndex != 1 || step.Stage != "1" {
			t.Errorf("want iteration 1 run again at row 1, got row %d stage %q", step.RowIndex, step.Stage)
		}
	})
}
