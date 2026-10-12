package engine_test

// Helpers for the iteration-end and phase-end tests. A run is described as the
// list of steps that completed, each one a table row with its recorded stage
// value. The boundary answers are asked once per step, with the Execution Log
// holding every step up to and including that one, the way the session asks
// them after each completed step. A test then states the steps after which an
// iteration or a phase ends.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/workflow"
)

// bndStep is one completed step: the zero-based table row and the stage value
// recorded for it ("" outside every block, "ID" in an ungrouped block,
// "ID.Group" in a grouped one).
type bndStep struct {
	row   int
	stage string
}

// key names the step in expectations as "row:stage".
func (s bndStep) key() string { return fmt.Sprintf("%d:%s", s.row, s.stage) }

// bndStepsOf returns the steps a played run dispatched, all answered SUCCESS.
func bndStepsOf(tr scopeTrace) []bndStep {
	out := make([]bndStep, 0, len(tr.steps))
	for _, s := range tr.steps {
		out = append(out, bndStep{row: s.RowIndex, stage: s.Stage})
	}
	return out
}

// bndSteps builds steps from "row:stage" keys, for runs written by hand.
func bndSteps(t *testing.T, keys ...string) []bndStep {
	t.Helper()
	out := make([]bndStep, 0, len(keys))
	for _, k := range keys {
		var s bndStep
		row, stage, ok := strings.Cut(k, ":")
		if !ok {
			t.Fatalf("step key %q must be row:stage", k)
		}
		if _, err := fmt.Sscanf(row, "%d", &s.row); err != nil {
			t.Fatalf("step key %q: bad row: %v", k, err)
		}
		s.stage = stage
		out = append(out, s)
	}
	return out
}

// bndIteration is the iteration ID of a recorded value: "ID.Group" (3.0) or
// "ID". The pre-3.0 "Test.ID" and "Implementation.ID" spellings, which hand
// written run logs may still carry as inputs, read as their ID too.
func bndIteration(stage string) domain.IterationID {
	first, rest, grouped := strings.Cut(stage, ".")
	if grouped && (first == "Test" || first == "Implementation") {
		return domain.IterationID(rest)
	}
	return domain.IterationID(first)
}

// bndLog is the Execution Log of the steps, each one SUCCESS, numbered from 1.
func bndLog(aw domain.AdmittedWorkflow, steps []bndStep) []domain.ExecutionLogEntry {
	log := make([]domain.ExecutionLogEntry, 0, len(steps))
	for i, s := range steps {
		row := aw.Table.Rows[s.row]
		agent := fmt.Sprintf("%s#%d", row.BareAgentID(), i+1)
		log = append(log, execLogEntry(i+1, agent, row.PhaseParsed.Name, s.stage, domain.StatusSUCCESS, s.row+1))
	}
	return log
}

// bndInputAt is the boundary input for the step at index at: the log holds the
// steps through that one.
func bndInputAt(aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, mode domain.ExecutionMode,
	steps []bndStep, at int) engine.BoundaryInput {
	return engine.BoundaryInput{
		Workflow:  aw,
		Schedules: schedules,
		RowIndex:  steps[at].row,
		Iteration: bndIteration(steps[at].stage),
		Log:       bndLog(aw, steps[:at+1]),
		Mode:      mode,
	}
}

// bndEnds returns the keys of the steps for which answer is true, in order.
func bndEnds(aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, mode domain.ExecutionMode,
	steps []bndStep, answer func(engine.BoundaryInput) bool) []string {
	ends := []string{}
	for i := range steps {
		if answer(bndInputAt(aw, schedules, mode, steps, i)) {
			ends = append(ends, steps[i].key())
		}
	}
	return ends
}

// bndIterationEnds returns the steps after which an iteration ends.
func bndIterationEnds(aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, mode domain.ExecutionMode,
	steps []bndStep) []string {
	return bndEnds(aw, schedules, mode, steps, engine.IsLastRowOfIteration)
}

// bndPhaseEnds returns the steps after which a phase ends.
func bndPhaseEnds(aw domain.AdmittedWorkflow, schedules domain.BlockSchedules, mode domain.ExecutionMode,
	steps []bndStep) []string {
	return bndEnds(aw, schedules, mode, steps, engine.IsLastRowOfPhaseForIteration)
}

// requireEnds asserts the steps after which a boundary was answered true.
func requireEnds(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s: want after steps %q, got after %q", what, want, got)
	}
}

// bndPlay plays aw over the schedules with every step answering SUCCESS and
// returns the completed steps; the run must complete.
func bndPlay(t *testing.T, aw domain.AdmittedWorkflow, mode domain.ExecutionMode, schedules domain.BlockSchedules) []bndStep {
	t.Helper()
	tr := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)
	if tr.final.Complete == nil {
		t.Fatalf("the fixture run must complete, got %s", tr.describe())
	}
	return bndStepsOf(tr)
}

// bndAdmitRaw parses a deployed 2.16 routing region with the strict parser and
// admits it in mode.
func bndAdmitRaw(t *testing.T, content string, mode domain.ExecutionMode) domain.AdmittedWorkflow {
	t.Helper()
	info := domain.WorkflowInfo{ID: "boundary-wf", Version: "2.16"}
	table, err := workflow.Parse([]byte(content), info)
	if err != nil {
		t.Fatalf("strict Parse of the fixture: %v", err)
	}
	aw, err := compat.Admit(table, mode)
	if err != nil {
		t.Fatalf("Admit of the fixture in mode %q: %v", modeName(mode), err)
	}
	return aw
}
