package engine_test

// The schedule-defect attempt counter is pure over the Execution Log: the
// SUCCESS entries of the producing row at one recorded value in the trailing
// run of the log. Infrastructure entries are skipped, an entry of any other
// routing-table row ends the run, and a non-SUCCESS entry of the row neither
// counts nor ends it.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const (
	sdProducerIndex = 0 // the producing row, 1-based row number 1
	sdOtherIndex    = 1
)

// sdEntry is one log entry of the row at index with the given recorded value
// and status; index -1 builds an infrastructure entry (no workflow row).
func sdEntry(index int, value string, status domain.StatusCode) domain.ExecutionLogEntry {
	row := domain.NoWorkflowRow
	if index >= 0 {
		row = domain.WorkflowRowFromIndex(index)
	}
	return domain.ExecutionLogEntry{Agent: "agent#1", WorkflowRow: row, Stage: value, Status: status}
}

func sdProducerRow() domain.RoutingRow { return domain.RoutingRow{Index: sdProducerIndex} }

// sdBlockProducerRow is the producing row as a row of an ungrouped block, whose
// recorded values are iteration IDs.
func sdBlockProducerRow() domain.RoutingRow { return ivUngroupedRow(sdProducerIndex) }

func TestScheduleDefectAttempts_CountsSuccessEntriesOfTheRowAtTheValue(t *testing.T) {
	cases := []struct {
		name     string
		inBlock  bool
		log      []domain.ExecutionLogEntry
		recorded string
		want     int
	}{
		{
			name: "consecutive attempts of the producer",
			log: []domain.ExecutionLogEntry{
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
			},
			want: 2,
		},
		{
			name: "an infrastructure entry between attempts does not end the run",
			log: []domain.ExecutionLogEntry{
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(-1, "", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
			},
			want: 2,
		},
		{
			name: "an entry of another routing-table row ends the run",
			log: []domain.ExecutionLogEntry{
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(sdOtherIndex, "A", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
			},
			want: 1,
		},
		{
			name: "a PARTIALLY_DONE entry of the row is not an attempt and does not end the run",
			log: []domain.ExecutionLogEntry{
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusPARTIALLY_DONE),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
			},
			want: 2,
		},
		{
			name:    "entries of the row at another value neither count nor end the run",
			inBlock: true,
			log: []domain.ExecutionLogEntry{
				sdEntry(sdProducerIndex, "A", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "B", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "A", domain.StatusSUCCESS),
			},
			recorded: "A",
			want:     2,
		},
		{
			name: "only the trailing run counts: earlier attempts before another row do not",
			log: []domain.ExecutionLogEntry{
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(sdOtherIndex, "A", domain.StatusSUCCESS),
				sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
				sdEntry(-1, "", domain.StatusSUCCESS),
			},
			want: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := sdProducerRow()
			if tc.inBlock {
				row = sdBlockProducerRow()
			}

			got := engine.ScheduleDefectAttempts(tc.log, row, tc.recorded)

			if got != tc.want {
				t.Errorf("want %d attempts, got %d", tc.want, got)
			}
		})
	}
}

func TestScheduleDefectAttempts_ThreeAttemptsReachTheLimit(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
		sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
		sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
	}

	attempts := engine.ScheduleDefectAttempts(log, sdProducerRow(), "")

	if used := attempts - 1; used != engine.ScheduleDefectRedispatchLimit {
		t.Errorf("want %d re-dispatches used after 3 attempts, got %d", engine.ScheduleDefectRedispatchLimit, used)
	}
}
