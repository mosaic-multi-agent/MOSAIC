package runflow

// RecordStep finishes the running step row with the recorded row, or appends the
// recorded row when no step is running, and never leaves a row running.

import (
	"reflect"
	"testing"
)

func TestProgressScreen_RecordStep_ReplacesTheRunningRowInEveryField(t *testing.T) {
	s := NewProgressScreen(80, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "done#1", Phase: "PLANNING", Status: "SUCCESS"})
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#2", Phase: "PLANNING", Stage: "1", Status: "running"})
	rec := ProgressRow{AgentInstance: "agent-a#3", Phase: "EXECUTION", Stage: "1.Test", Status: "BLOCKED"}

	s.RecordStep(rec)

	want := []ProgressRow{{AgentInstance: "done#1", Phase: "PLANNING", Status: "SUCCESS"}, rec}
	if got := s.Rows(); !reflect.DeepEqual(got, want) {
		t.Errorf("Rows() = %+v, want the running row replaced by the recorded row: %+v", got, want)
	}
}

func TestProgressScreen_RecordStep_NoRunningRow_AppendsTheRecordedRow(t *testing.T) {
	s := NewProgressScreen(80, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "done#1", Phase: "PLANNING", Status: "running"})
	s.CompleteRow("SUCCESS")
	rec := ProgressRow{AgentInstance: "commit-manager-git#2", Status: "SUCCESS"}

	s.RecordStep(rec)

	want := []ProgressRow{{AgentInstance: "done#1", Phase: "PLANNING", Status: "SUCCESS"}, rec}
	if got := s.Rows(); !reflect.DeepEqual(got, want) {
		t.Errorf("Rows() = %+v, want the recorded row appended: %+v", got, want)
	}
}

func TestProgressScreen_RecordStep_LeavesNoRowRunning(t *testing.T) {
	s := NewProgressScreen(80, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#1", Status: "running"})
	s.RecordStep(ProgressRow{AgentInstance: "agent-a#1", Status: "BLOCKED"})

	// A completion notice with no running row must not rewrite the recorded row.
	s.CompleteRow("SUCCESS")

	if got := s.Rows(); len(got) != 1 || got[0].Status != "BLOCKED" {
		t.Errorf("Rows() = %+v, want the single BLOCKED row unchanged", got)
	}
}

func TestProgressScreen_RecordStep_NextAppendedRowRunsAfterTheRecordedOne(t *testing.T) {
	s := NewProgressScreen(80, 24, progressStyles())
	s.AppendRow(ProgressRow{AgentInstance: "agent-a#1", Status: "running"})
	s.RecordStep(ProgressRow{AgentInstance: "agent-a#1", Status: "BLOCKED"})

	s.AppendRow(ProgressRow{AgentInstance: "agent-a#2", Status: "running"})
	s.CompleteRow("SUCCESS")

	want := []string{"BLOCKED", "SUCCESS"}
	rows := s.Rows()
	if len(rows) != 2 || rows[0].Status != want[0] || rows[1].Status != want[1] {
		t.Errorf("Rows() = %+v, want statuses %v", rows, want)
	}
}
