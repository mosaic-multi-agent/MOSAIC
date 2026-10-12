package domain_test

// Tests for domain.ValidateDispatchIteration with a consultant's iteration in
// the run artifact's value form ("{ID}" or "{ID}.{Group}"): the value is split
// by the named row's block and group, each ID is checked against the schedule
// of the row's block, and the agent is compared with the row's bare agent id.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// iterationRows (1-based row numbers):
//
//	1 researcher                (outside every block)
//	2 dev                       (block Stage, group Test)
//	3 dev                       (block Stage, group Implementation)
//	4 builder                   (block Batch, ungrouped)
//	5 build-review(tests)       (outside every block, qualified)
//	6 build-review(impl)        (block Stage, group Implementation, qualified)
//
// The Phase text of every row is deliberately uninformative: block and group
// come from the parsed phase.
func iterationRows() domain.RoutingTable {
	stage := []domain.BlockName{"Stage"}
	return domain.RoutingTable{Rows: []domain.RoutingRow{
		{Index: 0, Phase: "RESEARCH", Agent: "researcher", AgentID: "researcher",
			PhaseParsed: domain.PhaseParsed{Name: "RESEARCH"}},
		{Index: 1, Phase: "EXECUTION", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Test", Blocks: stage}},
		{Index: 2, Phase: "EXECUTION", Agent: "dev", AgentID: "dev",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Implementation", Blocks: stage}},
		{Index: 3, Phase: "BATCHING", Agent: "builder", AgentID: "builder",
			PhaseParsed: domain.PhaseParsed{Name: "BATCHING", IsStaged: true, Blocks: []domain.BlockName{"Batch"}}},
		{Index: 4, Phase: "PLANNING", Agent: "build-review(tests)", AgentID: "build-review", Qualifier: "tests",
			PhaseParsed: domain.PhaseParsed{Name: "PLANNING"}},
		{Index: 5, Phase: "EXECUTION", Agent: "build-review(impl)", AgentID: "build-review", Qualifier: "impl",
			PhaseParsed: domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Group: "Implementation", Blocks: stage}},
	}}
}

// iterationSchedules gives block Stage the IDs 1, A and b-2 and block Batch
// the IDs 1 and A.
func iterationSchedules() domain.BlockSchedules {
	build := func(block domain.BlockName, ids ...string) domain.BlockSchedule {
		s := domain.BlockSchedule{Block: block}
		for _, id := range ids {
			s.Iterations = append(s.Iterations, domain.ScheduleIteration{ID: domain.IterationID(id)})
		}
		return s
	}
	return domain.BlockSchedules{
		"Stage": build("Stage", "1", "A", "b-2"),
		"Batch": build("Batch", "1", "A"),
	}
}

func TestValidateDispatchIteration_IterationValue_ResolvesToScheduleSpellingAndRecordedValue(t *testing.T) {
	cases := []struct {
		name         string
		target       domain.DispatchTarget
		wantIndex    int
		wantID       domain.IterationID
		wantRecorded string
	}{
		{"grouped row with its group",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1.Test"}, 1, "1", "1.Test"},
		{"grouped row, non-numeric ID",
			domain.DispatchTarget{Agent: "dev", Row: 3, Iteration: "A.Implementation"}, 2, "A", "A.Implementation"},
		{"grouped row, ID with a dash",
			domain.DispatchTarget{Agent: "dev", Row: 3, Iteration: "b-2.Implementation"}, 2, "b-2", "b-2.Implementation"},
		{"grouped row, ID in other letter case is recorded as the schedule spells it",
			domain.DispatchTarget{Agent: "dev", Row: 3, Iteration: "a.Implementation"}, 2, "A", "A.Implementation"},
		{"grouped row, upper-case spelling of a dashed ID",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "B-2.Test"}, 1, "b-2", "b-2.Test"},
		{"ungrouped block row, numeric ID",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "1"}, 3, "1", "1"},
		{"ungrouped block row, non-numeric ID",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "A"}, 3, "A", "A"},
		{"ungrouped block row, ID in other letter case",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "a"}, 3, "A", "A"},
		{"row outside every block, no iteration",
			domain.DispatchTarget{Agent: "researcher", Row: 1}, 0, "", ""},
		{"qualified grouped row by its bare agent id",
			domain.DispatchTarget{Agent: "build-review", Row: 6, Iteration: "b-2.Implementation"}, 5, "b-2", "b-2.Implementation"},
		{"qualified row outside every block by its bare agent id",
			domain.DispatchTarget{Agent: "build-review", Row: 5}, 4, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ValidateDispatchIteration(iterationRows(), iterationSchedules(), tc.target)

			if err != nil {
				t.Fatalf("want the target accepted, got %v", err)
			}
			if got.Row.Index != tc.wantIndex {
				t.Errorf("want the named row (index %d), got index %d", tc.wantIndex, got.Row.Index)
			}
			if got.Iteration != tc.wantID {
				t.Errorf("want Iteration %q, got %q", tc.wantID, got.Iteration)
			}
			if got.RecordedStage != tc.wantRecorded {
				t.Errorf("want RecordedStage %q, got %q", tc.wantRecorded, got.RecordedStage)
			}
		})
	}
}

func TestValidateDispatchIteration_IterationValue_RejectedWithItsReason(t *testing.T) {
	cases := []struct {
		name   string
		target domain.DispatchTarget
		want   domain.DispatchTargetReason
	}{
		// Group segment missing or different on a grouped row.
		{"grouped row without a group segment",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1"}, domain.ReasonStageGroupMismatch},
		{"grouped row with another group",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1.Implementation"}, domain.ReasonStageGroupMismatch},
		{"grouped row with an unknown group",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1.Wrong"}, domain.ReasonStageGroupMismatch},
		{"group compared exactly",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1.test"}, domain.ReasonStageGroupMismatch},
		{"empty group segment",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1."}, domain.ReasonStageGroupMismatch},
		// Malformed values.
		{"too many segments on a grouped row",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "1.Test.x"}, domain.ReasonMalformedIteration},
		{"dash on a grouped row",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "-"}, domain.ReasonMalformedIteration},
		{"empty ID on a grouped row",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: ".Test"}, domain.ReasonMalformedIteration},
		{"ID outside the character set",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "bad_id.Test"}, domain.ReasonMalformedIteration},
		{"ID check runs before the group check",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "bad_id.Other"}, domain.ReasonMalformedIteration},
		{"group segment on an ungrouped block row",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "1.Test"}, domain.ReasonMalformedIteration},
		{"dash on an ungrouped block row",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "-"}, domain.ReasonMalformedIteration},
		{"ID outside the character set on an ungrouped row",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "bad_id"}, domain.ReasonMalformedIteration},
		// Unknown ID.
		{"unknown ID on a grouped row",
			domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "Z.Test"}, domain.ReasonUnknownStage},
		{"unknown ID on an ungrouped row",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "Z"}, domain.ReasonUnknownStage},
		{"ID of another block's schedule",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "b-2"}, domain.ReasonUnknownStage},
		{"zero is an ID like any other",
			domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "0"}, domain.ReasonUnknownStage},
		// Missing iteration on a block row.
		{"ungrouped block row without an iteration",
			domain.DispatchTarget{Agent: "builder", Row: 4}, domain.ReasonMissingStage},
		{"grouped row without an iteration",
			domain.DispatchTarget{Agent: "dev", Row: 3}, domain.ReasonMissingStage},
		// Iteration given on a row outside every block.
		{"ID on a row outside every block",
			domain.DispatchTarget{Agent: "researcher", Row: 1, Iteration: "1"}, domain.ReasonStageOnNonStagedRow},
		{"non-numeric ID on a row outside every block",
			domain.DispatchTarget{Agent: "researcher", Row: 1, Iteration: "A"}, domain.ReasonStageOnNonStagedRow},
		{"grouped value on a row outside every block",
			domain.DispatchTarget{Agent: "researcher", Row: 1, Iteration: "1.Test"}, domain.ReasonStageOnNonStagedRow},
		{"dash on a row outside every block",
			domain.DispatchTarget{Agent: "researcher", Row: 1, Iteration: "-"}, domain.ReasonStageOnNonStagedRow},
		{"ID on a qualified row outside every block",
			domain.DispatchTarget{Agent: "build-review", Row: 5, Iteration: "A"}, domain.ReasonStageOnNonStagedRow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ValidateDispatchIteration(iterationRows(), iterationSchedules(), tc.target)

			assertTargetReason(t, err, tc.want)
		})
	}
}

// A row's block is decided by its parsed block, so the same ID is valid on the
// row of the block that schedules it and unknown on the row of the other block.
func TestValidateDispatchIteration_IDIsCheckedAgainstTheScheduleOfTheRowsBlock(t *testing.T) {
	onStage, errStage := domain.ValidateDispatchIteration(iterationRows(), iterationSchedules(),
		domain.DispatchTarget{Agent: "dev", Row: 2, Iteration: "b-2.Test"})
	_, errBatch := domain.ValidateDispatchIteration(iterationRows(), iterationSchedules(),
		domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "b-2"})

	if errStage != nil || onStage.Iteration != "b-2" {
		t.Errorf("want b-2 accepted on the Stage row, got %+v err=%v", onStage, errStage)
	}
	assertTargetReason(t, errBatch, domain.ReasonUnknownStage)
}

// A block without a schedule rejects every iteration that is well formed.
func TestValidateDispatchIteration_BlockWithoutScheduleRejectsWellFormedIteration(t *testing.T) {
	onlyStage := domain.BlockSchedules{"Stage": iterationSchedules()["Stage"]}

	_, err := domain.ValidateDispatchIteration(iterationRows(), onlyStage,
		domain.DispatchTarget{Agent: "builder", Row: 4, Iteration: "1"})

	assertTargetReason(t, err, domain.ReasonUnknownStage)
}

// ===== Agent cross-check for mode-qualified rows =====

func TestValidateDispatchIteration_QualifiedRowAcceptsOnlyTheBareAgentID(t *testing.T) {
	cases := []struct {
		name   string
		target domain.DispatchTarget
		want   domain.DispatchTargetReason // "" = accepted
	}{
		{"bare id", domain.DispatchTarget{Agent: "build-review", Row: 5}, ""},
		{"whole cell", domain.DispatchTarget{Agent: "build-review(tests)", Row: 5}, domain.ReasonAgentMismatch},
		{"whole cell of the other qualified row",
			domain.DispatchTarget{Agent: "build-review(impl)", Row: 5}, domain.ReasonAgentMismatch},
		{"another agent", domain.DispatchTarget{Agent: "dev", Row: 5}, domain.ReasonAgentMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ValidateDispatchIteration(iterationRows(), iterationSchedules(), tc.target)

			if tc.want == "" {
				if err != nil {
					t.Fatalf("want the bare agent id accepted, got %v", err)
				}
				return
			}
			assertTargetReason(t, err, tc.want)
		})
	}
}

// The row named decides, not the first row holding the bare agent id.
func TestValidateDispatchIteration_BareAgentIDResolvesToTheNamedQualifiedRow(t *testing.T) {
	got, err := domain.ValidateDispatchIteration(iterationRows(), iterationSchedules(),
		domain.DispatchTarget{Agent: "build-review", Row: 6, Iteration: "A.Implementation"})

	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if got.Row.Index != 5 || got.Row.Qualifier != "impl" {
		t.Errorf("want the (impl) row at index 5, got index %d qualifier %q", got.Row.Index, got.Row.Qualifier)
	}
}

// The agent check comes before the iteration checks.
func TestValidateDispatchIteration_AgentMismatchReportedBeforeIterationProblems(t *testing.T) {
	_, err := domain.ValidateDispatchIteration(iterationRows(), iterationSchedules(),
		domain.DispatchTarget{Agent: "build-review(impl)", Row: 6, Iteration: "bad_id"})

	assertTargetReason(t, err, domain.ReasonAgentMismatch)
}
