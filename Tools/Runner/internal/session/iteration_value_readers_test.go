package session

// The session's readers of a recorded Iteration value -- the STAGE_END and
// PHASE_END triggers, the defaults of a consultation-routed dispatch and the
// re-dispatch of a schedule-defect producer -- read the value with the row it
// was recorded for. A value in the 3.0 spelling "{ID}.{Group}" resolves the
// iteration, with a non-numeric ID; the pre-3.0 "{Group}.{ID}" resolves none.

import (
	"slices"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

// ivWorkflowTable is a planner, a grouped block Stage (group Test: a writer and
// a review row), and a reporter.
//
//	0 planner  1 test-writer  2 tests-review  3 reporter
const ivWorkflowTable = `| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | NEXT | - | - | Plan.md |
| EXECUTION.[Stage].Test | test-writer | FALSE | NEXT | - | - | out-1-{Stage}.md |
| EXECUTION.[Stage].Test | tests-review | FALSE | COMPLETE | test-writer | - | out-2-{Stage}.md |
| REPORT | reporter | FALSE | COMPLETE | - | - | out-3.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Stage | Plan.md | Stages |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Stage | Tests-Only | Test |
`

const (
	ivRowWriter = 1
	ivRowReview = 2
)

// ivAdmitted parses and admits ivWorkflowTable.
func ivAdmitted(t *testing.T) domain.AdmittedWorkflow {
	t.Helper()
	info := domain.WorkflowInfo{ID: "iteration-value", Version: "2.16"}
	table, err := workflow.Parse([]byte(ivWorkflowTable), info)
	if err != nil {
		t.Fatalf("Parse of the fixture: %v", err)
	}
	aw, err := compat.Admit(table, domain.ExecutionModeAutoReview)
	if err != nil {
		t.Fatalf("Admit of the fixture: %v", err)
	}
	return aw
}

// ivSchedules holds the iterations A and B, non-numeric IDs, for block Stage.
func ivSchedules() domain.BlockSchedules {
	return domain.BlockSchedules{"Stage": domain.BlockSchedule{
		Block: "Stage",
		Iterations: []domain.ScheduleIteration{
			{ID: "A", Approach: "Tests-Only"},
			{ID: "B", Approach: "Tests-Only"},
		},
	}}
}

// ivSpell writes the recorded value of iteration id of group Test: "3.0 value"
// is the spelling that is read, "pre-3.0 value" the one that is not.
func ivSpell(spelling, id string) string {
	if spelling == "3.0 value" {
		return id + ".Test"
	}
	return "Test." + id
}

// ivStep is one completed step of the fixture workflow: a table row and the
// ID of the iteration it ran in.
type ivStep struct {
	row int
	id  string
}

// ivTriggerFires asks whether the trigger fires after the last of the steps,
// every one recorded in the given spelling; the log holds all of them.
func ivTriggerFires(t *testing.T, trigger string, spelling string, steps ...ivStep) bool {
	t.Helper()
	aw := ivAdmitted(t)
	var log []domain.ExecutionLogEntry
	for i, s := range steps {
		log = append(log, domain.ExecutionLogEntry{
			Seq: i + 1, Agent: aw.Table.Rows[s.row].BareAgentID() + "#" + string(rune('1'+i)),
			Phase: "EXECUTION", Stage: ivSpell(spelling, s.id),
			WorkflowRow: domain.WorkflowRowFromIndex(s.row), Status: domain.StatusSUCCESS,
		})
	}
	return ivTriggerFiresAfter(t, aw, trigger, log)
}

// ivTriggerFiresAfter asks whether the trigger fires after the last entry of
// log, whose row is the completed step's row.
func ivTriggerFiresAfter(t *testing.T, aw domain.AdmittedWorkflow, trigger string, log []domain.ExecutionLogEntry) bool {
	t.Helper()
	last := log[len(log)-1]
	step := domain.CompletedStep{
		Seq: last.Seq, AgentInstance: last.Agent, Phase: last.Phase, Stage: last.Stage,
		WorkflowRow: last.WorkflowRow, Status: domain.StatusSUCCESS,
	}
	boundaries := triggerBoundaries{
		Workflow: aw, Stages: ivSchedules(), RowIndex: last.WorkflowRow.Index(), Mode: domain.ExecutionModeAutoReview,
	}

	return infraTriggerFires(domain.DeclaredInfraTrigger{Trigger: trigger}, last.Seq,
		log, "checkpoint-manager", step, boundaries, true)
}

func TestIterationEndTriggers_RecordedValueInTheThreeOhSpelling_ResolvesTheIteration(t *testing.T) {
	cases := []struct {
		name    string
		trigger string
		steps   []ivStep
		want    bool
	}{
		{"STAGE_END at the last row of the group", "STAGE_END",
			[]ivStep{{ivRowWriter, "A"}, {ivRowReview, "A"}}, true},
		{"STAGE_END before the last row of the group", "STAGE_END",
			[]ivStep{{ivRowWriter, "A"}}, false},
		{"PHASE_END at the last row of the last iteration", "PHASE_END",
			[]ivStep{{ivRowWriter, "A"}, {ivRowReview, "A"}, {ivRowWriter, "B"}, {ivRowReview, "B"}}, true},
		{"PHASE_END at the last row of an earlier iteration", "PHASE_END",
			[]ivStep{{ivRowWriter, "A"}, {ivRowReview, "A"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ivTriggerFires(t, tc.trigger, "3.0 value", tc.steps...)

			if got != tc.want {
				t.Errorf("%s: want fires=%v, got %v", tc.trigger, tc.want, got)
			}
		})
	}
}

// The pre-3.0 spelling is a value the row does not accept: the step that would
// end the iteration or the phase ends nothing.
func TestIterationEndTriggers_RecordedValueInThePreThreeOhSpelling_DoesNotFire(t *testing.T) {
	cases := []struct {
		name    string
		trigger string
		steps   []ivStep
	}{
		{"STAGE_END at the last row of the group", "STAGE_END",
			[]ivStep{{ivRowWriter, "A"}, {ivRowReview, "A"}}},
		{"PHASE_END at the last row of the last iteration", "PHASE_END",
			[]ivStep{{ivRowWriter, "A"}, {ivRowReview, "A"}, {ivRowWriter, "B"}, {ivRowReview, "B"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if ivTriggerFires(t, tc.trigger, "pre-3.0 value", tc.steps...) {
				t.Errorf("%s must not fire for a pre-3.0 value", tc.trigger)
			}
		})
	}
}

func TestIterationEndTriggers_RecordedValueTheRowDoesNotAccept_DoesNotFire(t *testing.T) {
	aw := ivAdmitted(t)
	log := []domain.ExecutionLogEntry{{
		Seq: 1, Agent: "tests-review#1", Phase: "EXECUTION", Stage: "A.Wrong",
		WorkflowRow: domain.WorkflowRowFromIndex(ivRowReview), Status: domain.StatusSUCCESS,
	}}
	for _, trigger := range []string{"STAGE_END", "PHASE_END"} {
		t.Run(trigger, func(t *testing.T) {
			if ivTriggerFiresAfter(t, aw, trigger, log) {
				t.Errorf("%s must not fire for a value the row does not accept", trigger)
			}
		})
	}
}

func TestConsultPayload_RecordedValueInTheThreeOhSpelling_ResolvesTheIterationDefaults(t *testing.T) {
	aw := ivAdmitted(t)
	row := aw.Table.Rows[ivRowWriter]
	recorded := ivSpell("3.0 value", "A")

	got, err := resolveConsultPayload(&domain.DispatchInstruction{}, row, recorded,
		ivSchedules())

	if err != nil {
		t.Fatalf("resolveConsultPayload(%q): %v", recorded, err)
	}
	if !slices.Contains(got.Outputs, "out-1-A.md") {
		t.Errorf("want the defaults of iteration A (out-1-A.md), got outputs %v", got.Outputs)
	}
}

func TestConsultPayload_RecordedValueInThePreThreeOhSpelling_ResolvesNoIterationDefaults(t *testing.T) {
	aw := ivAdmitted(t)
	row := aw.Table.Rows[ivRowWriter]
	recorded := ivSpell("pre-3.0 value", "A")

	got, err := resolveConsultPayload(&domain.DispatchInstruction{}, row, recorded,
		ivSchedules())

	if err == nil && slices.Contains(got.Outputs, "out-1-A.md") {
		t.Errorf("a pre-3.0 value %q must not resolve the defaults of iteration A, got outputs %v", recorded, got.Outputs)
	}
}

func TestScheduleDefectStep_RecordedValueInTheThreeOhSpelling_ResolvesTheIterationDefaults(t *testing.T) {
	aw := ivAdmitted(t)
	row := aw.Table.Rows[ivRowWriter]
	recorded := ivSpell("3.0 value", "B")
	rs := &runStartCtx{admitted: aw, stages: ivSchedules(), seq: 4}
	p := &pendingScheduleDefect{rowIndex: ivRowWriter, stage: recorded}

	step, err := (&sessionImpl{}).scheduleDefectStep(rs, p, row)

	if err != nil {
		t.Fatalf("scheduleDefectStep(%q): %v", recorded, err)
	}
	if !slices.Contains(step.Request.OutputArtifacts, "out-1-B.md") {
		t.Errorf("want the outputs of iteration B (out-1-B.md), got %v", step.Request.OutputArtifacts)
	}
}

func TestScheduleDefectStep_RecordedValueInThePreThreeOhSpelling_ResolvesNoIterationDefaults(t *testing.T) {
	aw := ivAdmitted(t)
	row := aw.Table.Rows[ivRowWriter]
	recorded := ivSpell("pre-3.0 value", "B")
	rs := &runStartCtx{admitted: aw, stages: ivSchedules(), seq: 4}
	p := &pendingScheduleDefect{rowIndex: ivRowWriter, stage: recorded}

	step, err := (&sessionImpl{}).scheduleDefectStep(rs, p, row)

	if err == nil && slices.Contains(step.Request.OutputArtifacts, "out-1-B.md") {
		t.Errorf("a pre-3.0 value %q must not resolve the outputs of iteration B, got %v",
			recorded, step.Request.OutputArtifacts)
	}
}
