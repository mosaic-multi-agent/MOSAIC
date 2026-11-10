package engine_test

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

func TestResumePoint_TargetFormatState_RepeatedAgent_Test_ResolvesNextRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")

	state := domain.ArtifactState{
		GlobalSequence: 9,
		CurrentState: domain.CurrentState{
			Phase:      "EXECUTION",
			Stage:      "1.Test",
			LastStatus: domain.StatusSUCCESS,
			LastAgent:  "build-review#9",
		},
		ExecutionLog: []domain.ExecutionLogEntry{
			{Seq: 9, Agent: "build-review#9", Phase: "EXECUTION", Stage: "1.Test", WorkflowRow: 9, Status: domain.StatusSUCCESS},
		},
	}

	info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
	if err != nil {
		t.Fatalf("ResumePoint: unexpected error: %v", err)
	}
	if info.RowIndex != 9 {
		t.Errorf("want RowIndex=9 (tests-review-tdd, next row in the Test group), got %d", info.RowIndex)
	}
	if info.RerunLast {
		t.Error("want RerunLast=false (clean completion), got true")
	}
}

// TestResumePoint_TargetFormatState_RepeatedAgent_Implementation_ResolvesNextRow
// mirrors the above for the Implementation-group row of the same repeated agent.
func TestResumePoint_TargetFormatState_RepeatedAgent_Implementation_ResolvesNextRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")

	state := domain.ArtifactState{
		GlobalSequence: 12,
		CurrentState: domain.CurrentState{
			Phase:      "EXECUTION",
			Stage:      "1.Implementation",
			LastStatus: domain.StatusSUCCESS,
			LastAgent:  "build-review#12",
		},
		ExecutionLog: []domain.ExecutionLogEntry{
			{Seq: 12, Agent: "build-review#12", Phase: "EXECUTION", Stage: "1.Implementation", WorkflowRow: 12, Status: domain.StatusSUCCESS},
		},
	}

	info, err := engine.ResumePointFromSchedules(aw, schedules, state, nil)
	if err != nil {
		t.Fatalf("ResumePoint: unexpected error: %v", err)
	}
	if info.RowIndex != 12 {
		t.Errorf("want RowIndex=12 (implementation-review, next row in the Implementation group), got %d", info.RowIndex)
	}
	if info.RerunLast {
		t.Error("want RerunLast=false (clean completion), got true")
	}
}
