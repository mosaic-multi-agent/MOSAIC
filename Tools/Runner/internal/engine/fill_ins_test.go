package engine_test

// {Name} fill-ins in Input and Output: the current iteration ID of the row's
// block replaces {Name}, in auto, auto-review and orchestrated (row defaults)
// dispatch resolution.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// fillInRows: the planner produces the schedule, row 2 is the block row with
// fill-ins in its Input and Output.
func fillInRows(block, source string) []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: source},
		{
			phase:  "EXECUTION.[" + block + "]",
			agent:  "implementer",
			input:  block + "-{" + block + "}/Plan.md",
			output: block + "-{" + block + "}/Result.md",
		},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

func TestFillIns_StageBlock_ReachesSubagentAsIterationId(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, fillInRows("Stage", "Plan.md"), stageBlockTables(), mode)
		schedules := scheduleOf("Stage", idsOf("", "1")...)

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		step := trace.steps[1]
		if !reflect.DeepEqual(step.Request.InputArtifacts, []string{"Stage-1/Plan.md"}) {
			t.Errorf("InputArtifacts: want [Stage-1/Plan.md], got %v", step.Request.InputArtifacts)
		}
		if !reflect.DeepEqual(step.Request.OutputArtifacts, []string{"Stage-1/Result.md"}) {
			t.Errorf("OutputArtifacts: want [Stage-1/Result.md], got %v", step.Request.OutputArtifacts)
		}
	})
}

func TestFillIns_NonStageBlockWithNonNumericIds_UsesEachIterationsId(t *testing.T) {
	tables := scopeBlocks("| Audit | Audit.md | Items |")
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, fillInRows("Audit", "Audit.md"), tables, mode)
		schedules := scheduleOf("Audit", idsOf("", "alpha", "b-2")...)

		trace := scopeRun{aw: aw, schedules: schedules, mode: mode}.play(t)

		if got := trace.rows(); !reflect.DeepEqual(got, []int{0, 1, 1, 2}) {
			t.Fatalf("dispatched rows: want [0 1 1 2], got %v", got)
		}
		for i, id := range map[int]string{1: "alpha", 2: "b-2"} {
			step := trace.steps[i]
			if want := []string{"Audit-" + id + "/Plan.md"}; !reflect.DeepEqual(step.Request.InputArtifacts, want) {
				t.Errorf("iteration %s InputArtifacts: want %v, got %v", id, want, step.Request.InputArtifacts)
			}
			if want := []string{"Audit-" + id + "/Result.md"}; !reflect.DeepEqual(step.Request.OutputArtifacts, want) {
				t.Errorf("iteration %s OutputArtifacts: want %v, got %v", id, want, step.Request.OutputArtifacts)
			}
		}
	})
}

func TestFillIns_FirstDispatchOfRunThatBeginsWithBlockRow_IsFilledIn(t *testing.T) {
	rows := []scopeRow{
		{phase: "EXECUTION.[Stage]", agent: "implementer", input: "Stage-{Stage}/Plan.md", output: "Stage-{Stage}/Result.md"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAuto)
	schedules := scheduleOf("Stage", idsOf("", "7")...)

	step := requireDispatch(t, firstDecision(aw, schedules, domain.ExecutionModeAuto))

	if !reflect.DeepEqual(step.Request.InputArtifacts, []string{"Stage-7/Plan.md"}) {
		t.Errorf("InputArtifacts: want [Stage-7/Plan.md], got %v", step.Request.InputArtifacts)
	}
}

// The shared row-defaults resolver, which the consulted dispatch uses, fills in
// the iteration it is asked for.
func TestFillIns_RowDefaultsResolver_FillsInRequestedIteration(t *testing.T) {
	tables := scopeBlocks("| Audit | Audit.md | Items |")
	aw := mustAdmitScope(t, fillInRows("Audit", "Audit.md"), tables, domain.ExecutionModeOrchestrated)
	schedules := scheduleOf("Audit", idsOf("", "alpha", "b-2")...)

	got, err := engine.ResolveRowDefaultsForIteration(aw.Table.Rows[1], "b-2", engine.ScheduleContext{Schedules: schedules})

	if err != nil {
		t.Fatalf("ResolveRowDefaultsForIteration: unexpected error %v", err)
	}
	if want := []string{"Audit-b-2/Plan.md"}; !reflect.DeepEqual(got.InputArtifacts, want) {
		t.Errorf("InputArtifacts: want %v, got %v", want, got.InputArtifacts)
	}
	if want := []string{"Audit-b-2/Result.md"}; !reflect.DeepEqual(got.OutputArtifacts, want) {
		t.Errorf("OutputArtifacts: want %v, got %v", want, got.OutputArtifacts)
	}
}

func TestFillIns_ResolveArtifacts_ReplacesBlockNameInInputAndOutput(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: scheduleOf("Audit", idsOf("", "alpha")...)}

	in, err := engine.ResolveArtifactsForIteration([]string{"Audit-{Audit}/Findings.md"}, "Audit", "alpha", sc, true)
	if err != nil {
		t.Fatalf("input: unexpected error %v", err)
	}
	out, err := engine.ResolveArtifactsForIteration([]string{"Audit-{Audit}/Report.md"}, "Audit", "alpha", sc, false)
	if err != nil {
		t.Fatalf("output: unexpected error %v", err)
	}

	if want := []string{"Audit-alpha/Findings.md"}; !reflect.DeepEqual(in, want) {
		t.Errorf("input: want %v, got %v", want, in)
	}
	if want := []string{"Audit-alpha/Report.md"}; !reflect.DeepEqual(out, want) {
		t.Errorf("output: want %v, got %v", want, out)
	}
}

func TestFillIns_ResolveArtifacts_LeavesOtherBracedTextUntouched(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: scheduleOf("Audit", idsOf("", "alpha")...)}

	got, err := engine.ResolveArtifactsForIteration([]string{"Audit-{Audit}/{Other}.md"}, "Audit", "alpha", sc, true)

	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if want := []string{"Audit-alpha/{Other}.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestFillIns_ResolveArtifacts_BlockFillInWithoutIteration_IsAnError(t *testing.T) {
	sc := engine.ScheduleContext{Schedules: scheduleOf("Audit", idsOf("", "alpha")...)}

	_, err := engine.ResolveArtifactsForIteration([]string{"Audit-{Audit}/Findings.md"}, "Audit", "", sc, true)

	if err == nil {
		t.Error("a {Audit} fill-in with no iteration must be an error")
	}
}
