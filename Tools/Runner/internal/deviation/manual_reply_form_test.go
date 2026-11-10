package deviation_test

// Tests for the dispatch the manual routing dialogue builds: it carries the
// bare agent id of the chosen row (also for a mode-qualified row), and the
// iteration it names forms, with the row's group, the run artifact's value that
// the shared validator accepts for exactly that row.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// replyTarget returns the target a consultant reply naming the dispatch would
// carry: the bare agent, the 1-based row and the iteration in the run
// artifact's value form (the ID, then the row's group when it has one).
func replyTarget(table domain.RoutingTable, d *domain.DispatchInstruction) domain.DispatchTarget {
	target := domain.DispatchTarget{Agent: d.Agent, Row: domain.WorkflowRowFromIndex(d.RowIndex)}
	if d.Iteration != "" {
		group := table.Rows[d.RowIndex].PhaseParsed.Group
		value := domain.FormatIterationValue(domain.IterationValue{ID: d.Iteration, Group: group})
		// Set by reflection: the target field holds a string of the value form.
		reflect.ValueOf(&target).Elem().FieldByName("Iteration").SetString(value)
	}
	return target
}

// requireValidReply fails unless the dispatch is a valid target of table under
// schedules, in the value form of the run artifact.
func requireValidReply(t *testing.T, table domain.RoutingTable, schedules domain.BlockSchedules, d *domain.DispatchInstruction) domain.ResolvedDispatchTarget {
	t.Helper()
	resolved, err := domain.ValidateDispatchIteration(table, schedules, replyTarget(table, d))
	if err != nil {
		t.Fatalf("want the dispatch to satisfy the shared validator in the value form, got %v", err)
	}
	return resolved
}

func TestManualDialogue_GroupedRow_BuiltReplyNamesTheValueFormOfTheIteration(t *testing.T) {
	d := newDialogue(t).standard("3", "2")

	instr, err := consultStaged(t, d, 3)

	disp := requireDispatch(t, instr, err)
	resolved := requireValidReply(t, stagedRoutingTable(), schedulesUpTo(3), disp)
	if resolved.RecordedStage != "2.Implementation" {
		t.Errorf("want the recorded value %q, got %q", "2.Implementation", resolved.RecordedStage)
	}
	if resolved.Row.Index != disp.RowIndex {
		t.Errorf("want the validated row to be the dispatched row %d, got %d", disp.RowIndex, resolved.Row.Index)
	}
}

func TestManualDialogue_QualifiedRowOutsideEveryBlock_DispatchesTheBareAgentID(t *testing.T) {
	d := newDialogue(t).standard("2", "")

	instr, err := consultManual(t, d, qualifiedRoutingTable(), manualRequest(qualifiedSchedules()))

	disp := requireDispatch(t, instr, err)
	if disp.Agent != "build-review" {
		t.Errorf("want the bare agent id dispatched, got %q", disp.Agent)
	}
	if disp.RowIndex != 1 || disp.Iteration != "" {
		t.Errorf("want RowIndex=1 with no iteration, got RowIndex=%d Iteration=%q", disp.RowIndex, disp.Iteration)
	}
	requireValidReply(t, qualifiedRoutingTable(), qualifiedSchedules(), disp)
}

func TestManualDialogue_QualifiedRowInABlock_DispatchesTheBareAgentIDAndIteration(t *testing.T) {
	d := newDialogue(t).standard("4", "A")

	instr, err := consultManual(t, d, qualifiedRoutingTable(), manualRequest(qualifiedSchedules()))

	disp := requireDispatch(t, instr, err)
	if disp.Agent != "build-review" {
		t.Errorf("want the bare agent id dispatched, got %q", disp.Agent)
	}
	if disp.RowIndex != 3 || disp.Iteration != "A" {
		t.Errorf("want RowIndex=3 Iteration=A, got RowIndex=%d Iteration=%q", disp.RowIndex, disp.Iteration)
	}
	resolved := requireValidReply(t, qualifiedRoutingTable(), qualifiedSchedules(), disp)
	if resolved.RecordedStage != "A.Implementation" {
		t.Errorf("want the recorded value %q, got %q", "A.Implementation", resolved.RecordedStage)
	}
}

// The agent named in the labels of the dialogue is the bare id: no option
// shows a whole qualified cell as the agent.
func TestManualDialogue_QualifiedRows_AreOfferedUnderTheBareAgentID(t *testing.T) {
	d := newDialogue(t).standard("2", "")

	instr, err := consultManual(t, d, qualifiedRoutingTable(), manualRequest(qualifiedSchedules()))

	requireDispatch(t, instr, err)
	rowQ := d.oneQ(domain.QuestionManualRow, 0)
	requireOptions(t, rowQ, 4)
	for _, o := range rowQ.Options {
		for _, cell := range []string{"build-review(tests)", "build-review(impl)"} {
			if strings.Contains(o.Label, cell) {
				t.Errorf("option %q: want the bare agent id, found the whole cell %q in %q", o.ID, cell, o.Label)
			}
		}
	}
	if q := d.textQ(domain.QuestionManualTask, 0); strings.Contains(q.Prompt, "build-review(tests)") {
		t.Errorf("want the task prompt to name the bare agent id, got %q", q.Prompt)
	}
}
