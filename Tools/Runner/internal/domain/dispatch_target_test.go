package domain_test

// Tests for domain.ValidateDispatchIteration: the shared check of a consultant's
// proposed row and stage against the routing table and the current stage set.

import (
	"errors"
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
)

// dispatchTargetTable returns a four-row table (1-based row numbers):
//
//	1 researcher (RESEARCH, non-staged)
//	2 dev        (EXECUTION.[Stage].Test, staged)
//	3 dev        (EXECUTION.[Stage].Implementation, staged)
//	4 reviewer   (REVIEW, non-staged)
func dispatchTargetTable() domain.RoutingTable {
	return domain.RoutingTable{Rows: []domain.RoutingRow{
		{Index: 0, Phase: "RESEARCH", Agent: "researcher", AgentID: "researcher", PhaseParsed: domain.PhaseParsed{Name: "RESEARCH"}},
		{Index: 1, Phase: "EXECUTION.[Stage].Test", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Test", Blocks: []domain.BlockName{"Stage"}}},
		{Index: 2, Phase: "EXECUTION.[Stage].Implementation", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Implementation", Blocks: []domain.BlockName{"Stage"}}},
		{Index: 3, Phase: "REVIEW", Agent: "reviewer", AgentID: "reviewer", PhaseParsed: domain.PhaseParsed{Name: "REVIEW"}},
	}}
}

func stageSetOf(n int) domain.BlockSchedules {
	sched := domain.BlockSchedule{Block: "Stage"}
	for i := 1; i <= n; i++ {
		sched.Iterations = append(sched.Iterations, domain.ScheduleIteration{ID: domain.IterationID(strconv.Itoa(i))})
	}
	return domain.BlockSchedules{"Stage": sched}
}

func assertTargetReason(t *testing.T, err error, want domain.DispatchTargetReason) {
	t.Helper()
	var te *domain.DispatchTargetError
	if !errors.As(err, &te) {
		t.Fatalf("want *DispatchTargetError with reason %q, got %T: %v", want, err, err)
	}
	if te.Reason != want {
		t.Errorf("want reason %q, got %q", want, te.Reason)
	}
}

func TestValidateDispatchIteration_ValidNonStagedRow(t *testing.T) {
	got, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2),
		domain.DispatchTarget{Agent: "reviewer", Row: 4})

	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if got.Row.Index != 3 || got.Row.Agent != "reviewer" {
		t.Errorf("want resolved row index 3 (reviewer), got index %d agent %q", got.Row.Index, got.Row.Agent)
	}
	if got.Iteration != "" || got.RecordedStage != "" {
		t.Errorf("want no stage for a non-staged row, got %q / %q", got.Iteration, got.RecordedStage)
	}
}

func TestValidateDispatchIteration_ValidStagedRowResolvesRowAndRecordedStage(t *testing.T) {
	got, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(3),
		domain.DispatchTarget{Agent: "dev", Row: 3, Iteration: "2.Implementation"})

	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if got.Row.Index != 2 {
		t.Errorf("want the named row (index 2), not the first row of the agent, got index %d", got.Row.Index)
	}
	if got.Iteration != "2" {
		t.Errorf("want Iteration %q, got %q", "2", got.Iteration)
	}
	if got.RecordedStage != "2.Implementation" {
		t.Errorf("want RecordedStage %q, got %q", "2.Implementation", got.RecordedStage)
	}
}

func TestValidateDispatchIteration_GroupSegmentMatchingRowIsAccepted(t *testing.T) {
	got, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(3),
		domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1.Test"})

	if err != nil {
		t.Fatalf("want no error for a stage group equal to the row's group, got %v", err)
	}
	if got.RecordedStage != "1.Test" {
		t.Errorf("want RecordedStage %q, got %q", "1.Test", got.RecordedStage)
	}
}

func TestValidateDispatchIteration_GroupSegmentOfAnotherGroupIsRejected(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(3),
		domain.DispatchTarget{Agent: "dev", Row: 3, Iteration: "1.Test"})

	assertTargetReason(t, err, domain.ReasonStageGroupMismatch)
}

func TestValidateDispatchIteration_MissingRowIsRejected(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2),
		domain.DispatchTarget{Agent: "researcher", Row: domain.NoWorkflowRow})

	assertTargetReason(t, err, domain.ReasonMissingRow)
}

func TestValidateDispatchIteration_UnknownRowIsRejected(t *testing.T) {
	for _, row := range []domain.WorkflowRow{5, 99, -1} {
		_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2),
			domain.DispatchTarget{Agent: "researcher", Row: row})

		assertTargetReason(t, err, domain.ReasonUnknownRow)
	}
}

func TestValidateDispatchIteration_AgentNotMatchingRowIsRejected(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2),
		domain.DispatchTarget{Agent: "reviewer", Row: 1})

	assertTargetReason(t, err, domain.ReasonAgentMismatch)
}

func TestValidateDispatchIteration_StagedRowWithoutStageIsRejected(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2),
		domain.DispatchTarget{Agent: "dev", Row: 2})

	assertTargetReason(t, err, domain.ReasonMissingStage)
}

func TestValidateDispatchIteration_StageOutsideCurrentSetIsRejected(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2),
		domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "3.Test"})

	assertTargetReason(t, err, domain.ReasonUnknownStage)
}

func TestValidateDispatchIteration_StagedRowWithNilStageSetIsRejected(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), domain.BlockSchedules(nil),
		domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1.Test"})

	assertTargetReason(t, err, domain.ReasonUnknownStage)
}

// A stage that appeared mid-run is valid as soon as the stage set holds it,
// and the same target is invalid against the earlier, smaller set.
func TestValidateDispatchIteration_StageAddedMidRunIsAccepted(t *testing.T) {
	target := domain.DispatchTarget{Agent: "dev", Row: 3, Iteration: "3.Implementation"}

	_, errBefore := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2), target)
	got, errAfter := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(3), target)

	assertTargetReason(t, errBefore, domain.ReasonUnknownStage)
	if errAfter != nil {
		t.Fatalf("want stage 3 accepted once the stage set holds it, got %v", errAfter)
	}
	if got.RecordedStage != "3.Implementation" {
		t.Errorf("want RecordedStage %q, got %q", "3.Implementation", got.RecordedStage)
	}
}

func TestValidateDispatchIteration_StageOnNonStagedRowIsRejectedNotDropped(t *testing.T) {
	cases := []struct {
		name   string
		target domain.DispatchTarget
	}{
		{"bare stage number", domain.DispatchTarget{Agent: "researcher", Row: 1, Iteration: "1"}},
		{"grouped value", domain.DispatchTarget{Agent: "researcher", Row: 1, Iteration: "1.Test"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2), tc.target)

			assertTargetReason(t, err, domain.ReasonStageOnNonStagedRow)
		})
	}
}

// Checks run in the documented order: a missing row is reported before the
// agent/row relation is looked at.
func TestValidateDispatchIteration_MissingRowReportedBeforeAgentMismatch(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(dispatchTargetTable(), stageSetOf(2),
		domain.DispatchTarget{Agent: "no-such-agent"})

	assertTargetReason(t, err, domain.ReasonMissingRow)
}
