package engine_test

// A log entry that records no workflow row (a log written before rows were
// recorded) is matched to a row by agent and phase, and its recorded Iteration
// value is read with that row, in the 3.0 spelling "{ID}.{Group}". A value the
// matched row does not accept, the pre-3.0 "{Group}.{ID}" included, is not a
// reason to fall back to every row of the agent: no row matches the entry.

import (
	"errors"
	"testing"

	"mosaic-run/internal/domain"
)

// ivLoopSteps are the first seven steps of the staged findings loop (writer,
// gate, review in the Test group) with every value in the 3.0 spelling.
func ivLoopSteps() []runStep {
	steps := make([]runStep, 0, 7)
	for _, s := range stagedLoopRun[:7] {
		s.stage = "1.Test"
		steps = append(steps, s)
	}
	return steps
}

// ivLoopLogEndingWithoutRow is the loop run followed by an entry of the one
// Implementation row of the agent that records no row, with the given value.
func ivLoopLogEndingWithoutRow(value string) *runLog {
	return (&runLog{}).play(ivLoopSteps(), 7).
		workflowStep("loop-agent", value, domain.StatusSUCCESS, int(domain.NoWorkflowRow))
}

// The agent fills four rows of one phase; the group the value names is what
// tells the Implementation row from the Test rows.
func TestResumePoint_EntryWithoutRecordedRow_ValueNamesTheGroup(t *testing.T) {
	aw := mustParseAndAdmitV216(t, findingsLoopContent, "staged-findings-loop", "1.0")
	log := ivLoopLogEndingWithoutRow("1.Implementation")

	clean, cleanErr := resumeLogIDs(aw, singleSchedules("TDD"), log, false)
	rerun, rerunErr := resumeLogIDs(aw, singleSchedules("TDD"), log, true)

	requireResumeAt(t, clean, cleanErr, len(aw.Table.Rows), false, "", -1)
	requireResumeAt(t, rerun, rerunErr, loopRowImpl-1, true, "1", groupImpl)
}

// requirePositionUnresolved asserts err carries a *domain.PositionUnresolvedError
// of the given cause and returns it.
func requirePositionUnresolved(
	t *testing.T, err error, want domain.PositionUnresolvedCause,
) *domain.PositionUnresolvedError {
	t.Helper()
	var perr *domain.PositionUnresolvedError
	if !errors.As(err, &perr) {
		t.Fatalf("want a *PositionUnresolvedError, got %v", err)
	}
	if perr.Cause != want {
		t.Errorf("position cause: want %d, got %d (%v)", want, perr.Cause, perr)
	}
	return perr
}

// ivUnreadableForTheWriterRow are values the grouped test-writer row does not
// accept, including the pre-3.0 spelling.
var ivUnreadableForTheWriterRow = []struct{ name, value string }{
	{"pre-3.0 value of the row's group", "Test.1"},
	{"pre-3.0 value with another group", "Wrong.1"},
	{"3.0 value with another group", "1.Wrong"},
	{"pre-3.0 value naming the group of another row", "Implementation.1"},
	{"3.0 value naming the group of another row", "1.Implementation"},
	{"ID without a group", "1"},
}

func TestResumePoint_EntryWithoutRecordedRow_ValueTheMatchedRowDoesNotAccept_NoRowMatches(t *testing.T) {
	f := newRetryFixture(t)
	for _, tc := range ivUnreadableForTheWriterRow {
		for _, interrupted := range []bool{false, true} {
			name := tc.name + "/clean"
			if interrupted {
				name = tc.name + "/interrupted"
			}
			t.Run(name, func(t *testing.T) {
				log := (&runLog{}).
					workflowStep(rtTestWriter, tc.value, domain.StatusSUCCESS, int(domain.NoWorkflowRow))

				_, err := resumeLogIDs(f.aw, f.schedules, log, interrupted)

				perr := requirePositionUnresolved(t, err, domain.CauseNoMatchingRow)
				if perr.RecordedRow != domain.NoWorkflowRow {
					t.Errorf("RecordedRow: want NoWorkflowRow, got %d", perr.RecordedRow)
				}
			})
		}
	}
}

func TestNext_EntryWithoutRecordedRow_ValueTheMatchedRowDoesNotAccept_NoRowMatches(t *testing.T) {
	f := newRetryFixture(t)
	for _, tc := range ivUnreadableForTheWriterRow {
		t.Run(tc.name, func(t *testing.T) {
			log := (&runLog{}).
				workflowStep(rtTestWriter, tc.value, domain.StatusSUCCESS, int(domain.NoWorkflowRow))

			dec := f.next(domain.ExecutionModeAuto, log, false)

			perr := requirePositionCause(t, dec, domain.CauseNoMatchingRow)
			if perr.RecordedRow != domain.NoWorkflowRow {
				t.Errorf("RecordedRow: want NoWorkflowRow, got %d", perr.RecordedRow)
			}
		})
	}
}
