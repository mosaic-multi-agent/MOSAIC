package session

// A row of a named group records its iteration in the 3.0 spelling
// "{ID}.{Group}". The two session counters that read the Execution Log -- the
// E501 budget that bounds a raw-text bypass and the re-dispatch limit of a
// schedule-defect producer -- count such entries as the iteration of that row;
// entries in an old spelling ("Stage-N", "{Group}.{ID}") are not counted.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// ivThreeOh is the recorded value of iteration A of the group Test.
const ivThreeOh = "A.Test"

// ivSessionLog is n entries of the writer row, each recorded with value.
func ivSessionLog(n int, value string, status domain.StatusCode, code domain.ErrorCode) []domain.ExecutionLogEntry {
	var log []domain.ExecutionLogEntry
	for i := 0; i < n; i++ {
		log = append(log, domain.ExecutionLogEntry{
			Seq: i + 1, Agent: "test-writer#" + string(rune('1'+i)), Phase: "EXECUTION", Stage: value,
			WorkflowRow: domain.WorkflowRowFromIndex(ivRowWriter), Status: status, ErrorCode: code,
		})
	}
	return log
}

func TestE501BudgetLeft_EntriesRecordedThreeOh_SpendTheBudgetOfTheIteration(t *testing.T) {
	aw := ivAdmitted(t)
	t.Run("budget left", func(t *testing.T) {
		state := domain.ArtifactState{
			ExecutionLog: ivSessionLog(engine.E501AttemptLimit-1, ivThreeOh, domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE),
		}

		if !e501BudgetLeft(state, aw.Table, ivRowWriter, ivThreeOh) {
			t.Errorf("one attempt must be left after %d entries recorded %q", engine.E501AttemptLimit-1, ivThreeOh)
		}
	})
	t.Run("limit reached", func(t *testing.T) {
		state := domain.ArtifactState{
			ExecutionLog: ivSessionLog(engine.E501AttemptLimit, ivThreeOh, domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE),
		}

		if e501BudgetLeft(state, aw.Table, ivRowWriter, ivThreeOh) {
			t.Errorf("no attempt may be left after %d entries recorded %q", engine.E501AttemptLimit, ivThreeOh)
		}
	})
}

// Entries recorded in an old spelling are values the row does not accept: they
// spend no budget of iteration A.
func TestE501BudgetLeft_EntriesRecordedOldSpelling_SpendNoBudget(t *testing.T) {
	aw := ivAdmitted(t)
	for spelling, value := range ivOldSpellings() {
		t.Run(spelling, func(t *testing.T) {
			state := domain.ArtifactState{
				ExecutionLog: ivSessionLog(engine.E501AttemptLimit, value, domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE),
			}

			if !e501BudgetLeft(state, aw.Table, ivRowWriter, ivThreeOh) {
				t.Errorf("entries recorded %q must not spend the budget of %q", value, ivThreeOh)
			}
		})
	}
}

// handleDefectAfterAttempts runs handleScheduleDefect for a defect of the
// writer row after n successful attempts recorded with value. A re-dispatch
// reaches the engine, which this session does not have; that is reported as a
// failure of the test instead of a crash of the test binary.
func handleDefectAfterAttempts(t *testing.T, n int, value string) domain.RunOutcome {
	t.Helper()
	aw := ivAdmitted(t)
	s := &sessionImpl{deps: Deps{Debug: domain.NopDebugLogger{}}}
	s.scheduleDefect = &pendingScheduleDefect{rowIndex: ivRowWriter, stage: ivThreeOh}
	rs := &runStartCtx{
		admitted: aw, stages: ivSchedules(), seq: n,
		state: domain.ArtifactState{ExecutionLog: ivSessionLog(n, value, domain.StatusSUCCESS, domain.ErrorNone)},
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the producer was re-dispatched after %d attempts recorded %q: %v", n, value, r)
		}
	}()

	out, _, err := s.handleScheduleDefect(context.Background(), rs)

	if err != nil {
		t.Fatalf("handleScheduleDefect: %v", err)
	}
	return out
}

func TestHandleScheduleDefect_AttemptsRecordedThreeOh_ReachTheRedispatchLimit(t *testing.T) {
	// The limit is used up once the log holds one attempt more than it allows
	// re-dispatches: the session consults instead, and without a consultant the
	// run ends unresolved.
	out := handleDefectAfterAttempts(t, engine.ScheduleDefectRedispatchLimit+1, ivThreeOh)

	if out.Status != domain.RunDeviationUnresolved {
		t.Errorf("status after the re-dispatch limit: want %q, got %q (%s)",
			domain.RunDeviationUnresolved, out.Status, out.Message)
	}
}
