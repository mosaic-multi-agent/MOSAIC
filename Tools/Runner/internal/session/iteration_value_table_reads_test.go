package session

// A row of a named group records its iteration in the 3.0 spelling
// "{ID}.{Group}". The session readers of a recorded Iteration value -- the
// STAGE_END trigger, the defaults of a consultation-routed dispatch and the
// re-dispatch of a schedule-defect producer -- resolve the other spellings to
// no iteration: "Stage-{ID}" names no group and "{Group}.{ID}" is the pre-3.0
// spelling. The 3.0 spelling is covered in iteration_value_readers_test.go.

import (
	"slices"
	"testing"

	"mosaic-run/internal/domain"
)

// ivOldSpellings are recorded values that no longer name iteration A of the
// group Test.
func ivOldSpellings() map[string]string {
	return map[string]string{
		"Stage-N":       "Stage-A",
		"pre-3.0 value": "Test.A",
	}
}

func TestStageEndTrigger_RowRecordedOldSpelling_DoesNotFire(t *testing.T) {
	aw := ivAdmitted(t)
	for spelling, value := range ivOldSpellings() {
		t.Run(spelling, func(t *testing.T) {
			var log []domain.ExecutionLogEntry
			for i, row := range []int{ivRowWriter, ivRowReview} {
				log = append(log, domain.ExecutionLogEntry{
					Seq: i + 1, Agent: aw.Table.Rows[row].BareAgentID() + "#" + string(rune('1'+i)),
					Phase: "EXECUTION", Stage: value,
					WorkflowRow: domain.WorkflowRowFromIndex(row), Status: domain.StatusSUCCESS,
				})
			}

			if ivTriggerFiresAfter(t, aw, "STAGE_END", log) {
				t.Errorf("STAGE_END must not fire after the last row recorded %q", value)
			}
		})
	}
}

func TestConsultPayload_RowRecordedOldSpelling_ResolvesNoIterationDefaults(t *testing.T) {
	aw := ivAdmitted(t)
	row := aw.Table.Rows[ivRowWriter]
	for spelling, recorded := range ivOldSpellings() {
		t.Run(spelling, func(t *testing.T) {
			got, err := resolveConsultPayload(&domain.DispatchInstruction{}, row, recorded,
				ivSchedules())

			if err == nil && slices.Contains(got.Outputs, "out-1-A.md") {
				t.Errorf("%q must not resolve the defaults of iteration A, got outputs %v", recorded, got.Outputs)
			}
		})
	}
}

func TestScheduleDefectStep_RowRecordedOldSpelling_ResolvesNoIterationDefaults(t *testing.T) {
	aw := ivAdmitted(t)
	row := aw.Table.Rows[ivRowWriter]
	for spelling, recorded := range ivOldSpellings() {
		t.Run(spelling, func(t *testing.T) {
			rs := &runStartCtx{admitted: aw, stages: ivSchedules(), seq: 4}
			p := &pendingScheduleDefect{rowIndex: ivRowWriter, stage: recorded}

			step, err := (&sessionImpl{}).scheduleDefectStep(rs, p, row)

			if err == nil && slices.Contains(step.Request.OutputArtifacts, "out-1-A.md") {
				t.Errorf("%q must not resolve the outputs of iteration A, got %v", recorded, step.Request.OutputArtifacts)
			}
		})
	}
}
