package engine_test

// Fixtures and a run driver for the run-scope routing, block entry and exit,
// fill-in and mode-qualified row tests. Tables are written in WorkflowFormat
// 2.16 notation inline, parsed with the strict parser and admitted, so the
// engine sees exactly the typed model a deployed workflow gives it.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/workflow"
)

const scopeColumns = "| Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
	"|-------|----------|:----:|------------|-------------|-------|--------|\n"

// scopeRow is one routing row of a 2.16 fixture. An empty onSuccess routes
// NEXT (COMPLETE on the last row); an empty onFindings or input is "-"; an
// empty output derives one that fills in every block reference of the phase.
type scopeRow struct {
	phase      string
	agent      string // whole Subagent cell, mode qualifier included
	onSuccess  string
	onFindings string
	input      string
	output     string
}

func scopeOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// scopeOutput derives an Output cell for row i that fills in the block
// references of its phase.
func scopeOutput(i int, phase string) string {
	out := fmt.Sprintf("out-%d", i)
	for _, seg := range strings.Split(phase, ".") {
		if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
			out += "-{" + strings.Trim(seg, "[]") + "}"
		}
	}
	return out + ".md"
}

// scopeRegion renders the routing table of rows, then each extra table.
func scopeRegion(rows []scopeRow, tables ...string) string {
	var sb strings.Builder
	sb.WriteString(scopeColumns)
	for i, r := range rows {
		onSuccess := scopeOr(r.onSuccess, "NEXT")
		if r.onSuccess == "" && i == len(rows)-1 {
			onSuccess = "COMPLETE"
		}
		fmt.Fprintf(&sb, "| %s | %s | FALSE | %s | %s | %s | %s |\n",
			r.phase, r.agent, onSuccess, scopeOr(r.onFindings, "-"),
			scopeOr(r.input, "-"), scopeOr(r.output, scopeOutput(i, r.phase)))
	}
	sb.WriteString(strings.Join(tables, ""))
	return sb.String()
}

// scopeBlocks renders the **Blocks:** table with the given data rows.
func scopeBlocks(rows ...string) string {
	return "\n**Blocks:**\n\n| Block | Source | Section |\n|-------|--------|---------|\n" +
		strings.Join(rows, "\n") + "\n"
}

// scopeGroups renders the **Groups:** table with the given data rows.
func scopeGroups(rows ...string) string {
	return "\n**Groups:**\n\n| Block | Approach | Groups |\n|-------|----------|--------|\n" +
		strings.Join(rows, "\n") + "\n"
}

const stageBlockRow = "| Stage | Plan.md | Stages |"

// stageBlockTables declares the ungrouped block Stage.
func stageBlockTables() string { return scopeBlocks(stageBlockRow) }

// groupedStageTables declares block Stage with groups Test and Implementation
// and the TDD and Implementation-First approaches.
func groupedStageTables() string {
	return scopeBlocks(stageBlockRow) + scopeGroups(
		"| Stage | TDD | Test, Implementation |",
		"| Stage | Implementation-First | Implementation, Test |",
	)
}

// mustAdmitScope parses rows and tables with the strict 2.16 parser and
// admits the table in mode.
func mustAdmitScope(t *testing.T, rows []scopeRow, tables string, mode domain.ExecutionMode) domain.AdmittedWorkflow {
	t.Helper()
	info := domain.WorkflowInfo{ID: "scope-wf", Version: "2.16"}
	table, err := workflow.Parse([]byte(scopeRegion(rows, tables)), info)
	if err != nil {
		t.Fatalf("strict Parse of the fixture: %v", err)
	}
	aw, err := compat.Admit(table, mode)
	if err != nil {
		t.Fatalf("Admit of the fixture in mode %q: %v", modeName(mode), err)
	}
	return aw
}

// scopeAgents returns an agent map keyed by the bare agent id of every row.
func scopeAgents(aw domain.AdmittedWorkflow) map[string]domain.AgentReference {
	var ids []string
	for _, row := range aw.Table.Rows {
		ids = append(ids, row.BareAgentID())
	}
	return newTestAgents(ids...)
}

// ---- Schedules ----

// scheduleOf returns the schedules of one block holding the given iterations.
func scheduleOf(block domain.BlockName, its ...domain.ScheduleIteration) domain.BlockSchedules {
	return domain.BlockSchedules{block: domain.BlockSchedule{Block: block, Iterations: its}}
}

// idsOf returns plain iterations with the given IDs; the approach, when
// non-empty, applies to every one.
func idsOf(approach domain.Approach, ids ...domain.IterationID) []domain.ScheduleIteration {
	its := make([]domain.ScheduleIteration, 0, len(ids))
	for _, id := range ids {
		its = append(its, domain.ScheduleIteration{ID: id, Approach: approach})
	}
	return its
}

// joinSchedules merges per-block schedules into one value.
func joinSchedules(parts ...domain.BlockSchedules) domain.BlockSchedules {
	out := domain.BlockSchedules{}
	for _, p := range parts {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

// ---- Run driver ----

// scopeRun plays a workflow through engine.Next, answering every dispatched
// step with a scripted status, until Next returns something other than a
// dispatch.
type scopeRun struct {
	aw        domain.AdmittedWorkflow
	schedules domain.BlockSchedules
	mode      domain.ExecutionMode
	// statusOf scripts the status of the visit-th dispatched step (zero-based);
	// nil answers SUCCESS to every step.
	statusOf  func(visit int, step domain.DispatchStep) domain.StatusCode
	loopLimit int
	// schedulesAt, when set, gives the schedules handed to the Next call that
	// follows the steps dispatched so far, modelling a schedule re-read after a
	// producing row; it takes precedence over schedules.
	schedulesAt func(dispatched []domain.DispatchStep) domain.BlockSchedules
	// source is the stage-source descriptor handed to every Next call.
	source domain.StageSource
	// consultantContinues models the orchestrator answering a finished-work
	// deviation by dispatching the finished step it names: the run goes on
	// with that step as an authoritative dispatch, and the deviation is
	// recorded in the trace.
	consultantContinues bool
}

// scopeTrace is what a played run dispatched and how it ended.
type scopeTrace struct {
	steps []domain.DispatchStep
	final domain.EngineDecision
	// finishedWork holds the finished-work deviations the run met and
	// continued past (scopeRun.consultantContinues).
	finishedWork []domain.DeviationInfo
}

const scopeStepGuard = 60

// play runs the workflow; the failing test names the step guard when the run
// does not end.
func (r scopeRun) play(t *testing.T) scopeTrace {
	t.Helper()
	agents := scopeAgents(r.aw)
	var (
		log   []domain.ExecutionLogEntry
		state domain.ArtifactState
		resp  *domain.ProtocolResponse
		trace scopeTrace
		seq   int
	)
	for i := 0; i < scopeStepGuard; i++ {
		schedules := r.schedules
		if r.schedulesAt != nil {
			schedules = r.schedulesAt(trace.steps)
		}
		dec := engine.Next(engine.NextInput{
			Workflow:     r.aw,
			Schedules:    schedules,
			State:        state,
			LastResponse: resp,
			Agents:       agents,
			Seq:          seq,
			Now:          fixedNow,
			StageSource:  r.source,
			Mode:         r.mode,
		})
		if r.consultantContinues && dec.Deviation != nil && dec.Deviation.Info.Kind == domain.DeviationFinishedWork &&
			dec.Deviation.Info.PlannedContinuation != nil {
			trace.finishedWork = append(trace.finishedWork, dec.Deviation.Info)
			dec = domain.EngineDecision{Dispatch: &domain.DispatchDecision{Steps: []domain.DispatchStep{
				*dec.Deviation.Info.PlannedContinuation}}}
		}
		if dec.Dispatch == nil {
			trace.final = dec
			return trace
		}
		step := requireDispatch(t, dec)
		trace.steps = append(trace.steps, step)
		seq++
		status := domain.StatusSUCCESS
		if r.statusOf != nil {
			status = r.statusOf(len(trace.steps)-1, step)
		}
		entry := execLogEntry(seq, step.Request.AgentInstanceID, step.Phase, step.Stage, status, step.RowIndex+1)
		log = append(log, entry)
		state = stateWithLog(seq, log...)
		state.ReviewLoopLimit = r.loopLimit
		resp = &domain.ProtocolResponse{AgentInstanceID: entry.Agent, StatusCode: status, StatusMessage: "scripted"}
	}
	t.Fatalf("run did not end within %d steps; rows so far %v", scopeStepGuard, trace.rows())
	return trace
}

// rows returns the zero-based row index of every dispatched step.
func (tr scopeTrace) rows() []int {
	out := make([]int, 0, len(tr.steps))
	for _, s := range tr.steps {
		out = append(out, s.RowIndex)
	}
	return out
}

// stages returns the recorded stage value of every dispatched step.
func (tr scopeTrace) stages() []string {
	out := make([]string, 0, len(tr.steps))
	for _, s := range tr.steps {
		out = append(out, s.Stage)
	}
	return out
}

// describe names the final decision of a trace for failure messages.
func (tr scopeTrace) describe() string {
	switch {
	case tr.final.Complete != nil:
		return "CompleteDecision"
	case tr.final.Deviation != nil:
		return fmt.Sprintf("DeviationDecision(kind=%q, row=%d)", tr.final.Deviation.Info.Kind, tr.final.Deviation.Info.CurrentRow)
	case tr.final.Stop != nil:
		return fmt.Sprintf("StopDecision(%q)", tr.final.Stop.Reason)
	case tr.final.Consult != nil:
		return "ConsultDecision"
	default:
		return "no decision"
	}
}

// requireRowsThenComplete asserts the zero-based rows the run dispatched, in order, and
// that it ended with a CompleteDecision.
func requireRowsThenComplete(t *testing.T, tr scopeTrace, want []int) {
	t.Helper()
	if got := tr.rows(); !reflect.DeepEqual(got, want) {
		t.Errorf("dispatched rows: want %v, got %v (run ended with %s)", want, got, tr.describe())
	}
	if tr.final.Complete == nil {
		t.Errorf("run must end with a CompleteDecision, got %s", tr.describe())
	}
}

// requireStages asserts the recorded stage value of every dispatched step.
func requireStages(t *testing.T, tr scopeTrace, want []string) {
	t.Helper()
	if got := tr.stages(); !reflect.DeepEqual(got, want) {
		t.Errorf("recorded stage values: want %q, got %q", want, got)
	}
}

// requireFinishedWorkAt asserts the finished-work deviations a run met and
// continued past (scopeRun.consultantContinues): one per wantRows entry, in
// order, each naming that zero-based row at the recorded stage value wantStage.
func requireFinishedWorkAt(t *testing.T, tr scopeTrace, wantRows []int, wantStage string) {
	t.Helper()
	var got []int
	for _, info := range tr.finishedWork {
		got = append(got, info.PlannedContinuation.RowIndex)
		if info.PlannedContinuation.Stage != wantStage {
			t.Errorf("finished-work step stage: want %q, got %q", wantStage, info.PlannedContinuation.Stage)
		}
	}
	if !reflect.DeepEqual(got, wantRows) {
		t.Errorf("finished-work deviations at rows: want %v, got %v", wantRows, got)
	}
}

// forEachAutoMode runs fn once per mode in which the engine routes (Modes 2
// and 3).
func forEachAutoMode(t *testing.T, fn func(t *testing.T, mode domain.ExecutionMode)) {
	t.Helper()
	for _, mode := range autoModes {
		t.Run(modeName(mode), func(t *testing.T) { fn(t, mode) })
	}
}
