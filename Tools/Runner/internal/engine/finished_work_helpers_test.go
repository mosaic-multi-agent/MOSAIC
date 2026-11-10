package engine_test

// Fixtures and assertions for the finished-work tests. In the automatic modes
// the engine never repeats work that already finished: when the step it would
// dispatch after an engine-routed SUCCESS already has a SUCCESS in the
// Execution Log (same recorded row, same iteration value), it returns a
// finished-work deviation that names that step instead.
//
// Two kinds of fixtures are used:
//   - the brownfield TDD workflow (retryFixture) with hand-built logs;
//   - a four-row table with an ungrouped block, for row numbering and the
//     matching of recorded iteration values.

import (
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// 1-based rows of brownfieldTDDV216Content used by the finished-work tests.
const (
	fwRowResearch       = 1
	fwRowRefinement     = 2
	fwRowReqReview      = 3
	fwRowPlanner        = 4
	fwRowPlanReview     = 5
	fwRowDesigner       = 6
	fwRowContractReview = 7
	fwRowTestWriter     = 8
	fwRowTestsReview    = 9
	fwRowImplementer    = 10
	fwRowImplReview     = 11
	fwRowTestRunner     = 12
)

const (
	fwDesigner       = "contracts-designer"
	fwContractReview = "contracts-review"
	fwImplementer    = "implementation-tdd"
	fwImplReview     = "implementation-review"
	fwTestRunner     = "test-runner"
)

// newFinishedWorkFixture is the brownfield TDD fixture whose schedule holds
// the iterations 1..n, all with the TDD approach.
func newFinishedWorkFixture(t *testing.T, n int) retryFixture {
	t.Helper()
	f := newRetryFixture(t)
	its := make([]domain.ScheduleIteration, 0, n)
	for i := 1; i <= n; i++ {
		its = append(its, domain.ScheduleIteration{ID: domain.IterationID(strconv.Itoa(i)), Approach: "TDD"})
	}
	f.schedules = newTestSchedules(its...)
	return f
}

// fwBase appends the research, planning and design rows, then both groups of
// iterations 1..done, each row succeeding once.
func fwBase(l *runLog, done int) *runLog {
	l.ok("codebase-research", "", fwRowResearch).
		ok("requirements-refinement", "", fwRowRefinement).
		ok("requirements-review", "", fwRowReqReview).
		ok(rtPlanner, "", fwRowPlanner).
		ok(rtPlanReview, "", fwRowPlanReview).
		ok(fwDesigner, "", fwRowDesigner).
		ok(fwContractReview, "", fwRowContractReview)
	for i := 1; i <= done; i++ {
		id := strconv.Itoa(i)
		l.ok(rtTestWriter, id+".Test", fwRowTestWriter).
			ok(rtTestsReview, id+".Test", fwRowTestsReview).
			ok(fwImplementer, id+".Implementation", fwRowImplementer).
			ok(fwImplReview, id+".Implementation", fwRowImplReview)
	}
	return l
}

// fwStage4TestGroup appends the Test group of iteration 4, then an
// implementation-tdd attempt at 4.Implementation that came back BLOCKED.
func fwStage4TestGroup(l *runLog) *runLog {
	return l.ok(rtTestWriter, "4.Test", fwRowTestWriter).
		ok(rtTestsReview, "4.Test", fwRowTestsReview).
		blocked(fwImplementer, "4.Implementation", domain.ErrorPERMISSION_DENIED, fwRowImplementer)
}

// fwScenario is a log whose next engine-routed step has already finished.
type fwScenario struct {
	name       string
	iterations int
	log        *runLog
	wantRow    int // 0-based row index of the finished step
	wantStage  string
}

// finishedWorkScenarios are the logs after which the engine's next step has
// already finished.
func finishedWorkScenarios() []fwScenario {
	plannerRerun := fwStage4TestGroup(fwBase(&runLog{}, 3)).ok(rtPlanner, "", fwRowPlanner)
	planReviewRerun := fwStage4TestGroup(fwBase(&runLog{}, 3)).
		ok(rtPlanner, "", fwRowPlanner).ok(rtPlanReview, "", fwRowPlanReview)
	reEntry := fwStage4TestGroup(fwBase(&runLog{}, 3)).
		ok(fwDesigner, "", fwRowDesigner).ok(fwContractReview, "", fwRowContractReview)
	reviewerPassed := fwBase(&runLog{}, 3).
		ok(rtTestWriter, "4.Test", fwRowTestWriter).
		ok(rtTestsReview, "4.Test", fwRowTestsReview).
		blocked(fwImplementer, "4.Implementation", domain.ErrorPERMISSION_DENIED, fwRowImplementer).
		ok(rtTestWriter, "4.Test", fwRowTestWriter)
	return []fwScenario{
		{"planner rerun, plan-review finished", 4, plannerRerun, fwRowPlanReview - 1, ""},
		{"plan-review rerun, designer finished", 4, planReviewRerun, fwRowDesigner - 1, ""},
		{"block re-entry onto test-writer at 4.Test", 4, reEntry, fwRowTestWriter - 1, "4.Test"},
		{"reviewer already passed at 4.Test", 4, reviewerPassed, fwRowTestsReview - 1, "4.Test"},
		{"after the new iteration, test-runner finished", 6, fwNewIteration(4), fwRowTestRunner - 1, ""},
	}
}

// fwNewIteration is the log of a run whose iterations 1..5 and test-runner
// finished, then a planner SUCCESS added iteration 6 and the orchestrator ran
// its first rows: the first k of test-writer, tests-review, implementation-tdd
// and implementation-review at iteration 6.
func fwNewIteration(k int) *runLog {
	l := fwBase(&runLog{}, 5).ok(fwTestRunner, "", fwRowTestRunner).ok(rtPlanner, "", fwRowPlanner)
	steps := []struct {
		agent, stage string
		row          int
	}{
		{rtTestWriter, "6.Test", fwRowTestWriter},
		{rtTestsReview, "6.Test", fwRowTestsReview},
		{fwImplementer, "6.Implementation", fwRowImplementer},
		{fwImplReview, "6.Implementation", fwRowImplReview},
	}
	if k > len(steps) {
		k = len(steps)
	}
	for _, s := range steps[:k] {
		l.ok(s.agent, s.stage, s.row)
	}
	return l
}

// requireFinishedWork requires the finished-work deviation naming the step at
// the given 0-based row index and recorded stage, and returns its info.
func requireFinishedWork(t *testing.T, dec domain.EngineDecision, wantRow int, wantStage string) domain.DeviationInfo {
	t.Helper()
	info := requireDeviation(t, dec).Info
	if info.Kind != domain.DeviationFinishedWork {
		t.Fatalf("deviation kind: want %q, got %q", domain.DeviationFinishedWork, info.Kind)
	}
	if info.PlannedContinuation == nil {
		t.Fatalf("finished-work deviation must carry the finished step as PlannedContinuation")
	}
	requireStepAt(t, *info.PlannedContinuation, wantRow, wantStage)
	return info
}

// requireDispatchAt requires a single dispatch of the given 0-based row index
// and recorded stage.
func requireDispatchAt(t *testing.T, dec domain.EngineDecision, wantRow int, wantStage string) domain.DispatchStep {
	t.Helper()
	step := requireDispatch(t, dec)
	requireStepAt(t, step, wantRow, wantStage)
	return step
}

// ---- Ungrouped-block fixture ----

// fwEdge is the four-row table
//
//	0 worker-one  1 worker-two  2 checker (findings -> worker-one)  3 final-review
//
// with the block Stage around rows 0-2 (the table starts with the block, so its
// schedule is seeded) and a schedule holding iterations "A" and "B". Row 0 is
// the first and row 3 the last row of the table.
type fwEdge struct {
	t    *testing.T
	aw   domain.AdmittedWorkflow
	mode domain.ExecutionMode
}

func newFinishedWorkEdge(t *testing.T, mode domain.ExecutionMode) fwEdge {
	t.Helper()
	rows := []scopeRow{
		{phase: "BUILD.[Stage]", agent: "worker-one"},
		{phase: "BUILD.[Stage]", agent: "worker-two"},
		{phase: "BUILD.[Stage]", agent: "checker", onFindings: "worker-one"},
		{phase: "REVIEW", agent: "final-review"},
	}
	return fwEdge{t: t, aw: mustAdmitScope(t, rows, stageBlockTables(), mode), mode: mode}
}

// fwRec is one Execution Log entry of a fwEdge run, always SUCCESS.
type fwRec struct {
	agent   string
	stage   string
	row     int // 1-based recorded row; 0 = "-"
	summary string
}

// next runs engine.Next after the last of the records. With resume, no
// response is supplied.
func (e fwEdge) next(recs []fwRec, resume bool) domain.EngineDecision {
	e.t.Helper()
	entries := make([]domain.ExecutionLogEntry, 0, len(recs))
	for i, r := range recs {
		seq := i + 1
		entry := execLogEntry(seq, r.agent+"#"+strconv.Itoa(seq), "EXECUTION", r.stage, domain.StatusSUCCESS, r.row)
		entry.Summary = r.summary
		entries = append(entries, entry)
	}
	last := entries[len(entries)-1]
	in := engine.NextInput{
		Workflow:  e.aw,
		Schedules: scheduleOf("Stage", idsOf("", "A", "B")...),
		State:     stateWithLog(len(entries), entries...),
		Agents:    scopeAgents(e.aw),
		Seq:       len(entries),
		Now:       fixedNow,
		Mode:      e.mode,
	}
	if !resume {
		in.LastResponse = successResponse(last.Agent)
	}
	return engine.Next(in)
}

// 1-based rows of the fwEdge table.
const (
	edgeWorkerOneRow = 1
	edgeWorkerTwoRow = 2
	edgeCheckerRow   = 3
	edgeFinalRow     = 4
)
