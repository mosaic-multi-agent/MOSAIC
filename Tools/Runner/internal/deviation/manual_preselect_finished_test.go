package deviation_test

// Tests for the preselected answers of the row and stage questions when the
// consultation was triggered by finished work: the manual dispatch then
// offers the finished next step, not the row that just succeeded.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// finishedWorkDeviation is the deviation the engine produces after a SUCCESS
// at (currentRow, currentStage) whose next step (plannedRow, plannedStage) has
// already finished. Rows are 0-based table indices.
func finishedWorkDeviation(currentRow int, currentStage string, plannedRow int, plannedStage string) *domain.DeviationInfo {
	return &domain.DeviationInfo{
		Kind:         domain.DeviationFinishedWork,
		CurrentRow:   currentRow,
		CurrentStage: currentStage,
		PlannedContinuation: &domain.DispatchStep{
			RowIndex: plannedRow,
			Stage:    plannedStage,
		},
	}
}

func TestManualDialogue_RowQuestion_FinishedWork_PreselectsTheFinishedNextStep(t *testing.T) {
	cases := []struct {
		name      string
		deviation *domain.DeviationInfo
		want      string
	}{
		{
			"test row succeeded, the implementation row of the stage had finished",
			finishedWorkDeviation(1, "2.Test", 2, "2.Implementation"), "3",
		},
		{
			"implementation row succeeded, the next stage's test row had finished",
			finishedWorkDeviation(2, "2.Implementation", 1, "3.Test"), "2",
		},
		{
			"non-staged row succeeded, the first staged row had finished",
			finishedWorkDeviation(0, "", 1, "1.Test"), "2",
		},
		{
			"staged row succeeded, the following non-staged row had finished",
			finishedWorkDeviation(2, "4.Implementation", 3, ""), "4",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard("4", "")
			d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualStopOptionID)}
			req := manualRequest(schedulesUpTo(4))
			req.Deviation = tc.deviation

			consultManual(t, d, stagedRoutingTable(), req) //nolint:errcheck

			got := d.oneQ(domain.QuestionManualRow, 0).DefaultOptionID
			if got != tc.want {
				t.Errorf("want row preselection %q, got %q", tc.want, got)
			}
		})
	}
}

func TestManualDialogue_RowQuestion_OtherKinds_KeepPreselectingTheRowThatDeviated(t *testing.T) {
	planned := &domain.DispatchStep{RowIndex: 3, Stage: ""}
	cases := []struct {
		name      string
		deviation *domain.DeviationInfo
		want      string
	}{
		{"non-success", &domain.DeviationInfo{Kind: domain.DeviationNonSuccess, CurrentRow: 1, CurrentStage: "2.Test"}, "2"},
		{"harness error", &domain.DeviationInfo{Kind: domain.DeviationHarnessError, CurrentRow: 2, CurrentStage: "2.Implementation"}, "3"},
		{
			"a planned continuation on another kind is not followed",
			&domain.DeviationInfo{Kind: domain.DeviationAmbiguousRoute, CurrentRow: 1, CurrentStage: "2.Test", PlannedContinuation: planned},
			"2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard("4", "")
			d.one[domain.QuestionManualRow] = []oneFn{pick(domain.ManualStopOptionID)}
			req := manualRequest(schedulesUpTo(4))
			req.Deviation = tc.deviation

			consultManual(t, d, stagedRoutingTable(), req) //nolint:errcheck

			got := d.oneQ(domain.QuestionManualRow, 0).DefaultOptionID
			if got != tc.want {
				t.Errorf("want row preselection %q, got %q", tc.want, got)
			}
		})
	}
}

func TestManualDialogue_RowQuestion_FinishedWork_EarlierRowAnswerWins(t *testing.T) {
	// The user picks row 2, backs out of the stage question and returns to the
	// row question: it offers the row they chose, not the finished step (row 3).
	d := newDialogue(t).standard("2", "1").cancelFirst(domain.QuestionManualStage)
	req := manualRequest(schedulesUpTo(4))
	req.Deviation = finishedWorkDeviation(0, "", 2, "2.Implementation")

	consultManual(t, d, stagedRoutingTable(), req) //nolint:errcheck

	first := d.oneQ(domain.QuestionManualRow, 0).DefaultOptionID
	if first != "3" {
		t.Errorf("first row question: want the finished step %q preselected, got %q", "3", first)
	}
	again := d.oneQ(domain.QuestionManualRow, 1).DefaultOptionID
	if again != "2" {
		t.Errorf("row question after backing out: want the earlier answer %q, got %q", "2", again)
	}
}

func TestManualDialogue_StageQuestion_FinishedWork_PreselectsTheFinishedIterationOfTheChosenRow(t *testing.T) {
	cases := []struct {
		name      string
		chosenRow string
		stages    int
		deviation *domain.DeviationInfo
		want      string
	}{
		{
			"finished implementation row of the same stage",
			"3", 4, finishedWorkDeviation(1, "2.Test", 2, "2.Implementation"), "2",
		},
		{
			"finished test row of the next stage",
			"2", 4, finishedWorkDeviation(2, "2.Implementation", 1, "3.Test"), "3",
		},
		{
			"finished iteration is not in the chosen row's schedule",
			"3", 3, finishedWorkDeviation(2, "1.Implementation", 2, "9.Implementation"), "",
		},
		{
			"chosen row is of another group than the finished iteration",
			"2", 4, finishedWorkDeviation(1, "2.Test", 2, "2.Implementation"), "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard(tc.chosenRow, "1")
			req := manualRequest(schedulesUpTo(tc.stages))
			req.Deviation = tc.deviation

			consultManual(t, d, stagedRoutingTable(), req) //nolint:errcheck

			got := d.oneQ(domain.QuestionManualStage, 0).DefaultOptionID
			if got != tc.want {
				t.Errorf("want stage preselection %q, got %q", tc.want, got)
			}
		})
	}
}

func TestManualDialogue_StageQuestion_OtherKinds_KeepPreselectingTheDeviatingStage(t *testing.T) {
	d := newDialogue(t).standard("3", "1")
	req := manualRequest(schedulesUpTo(4))
	req.Deviation = &domain.DeviationInfo{Kind: domain.DeviationNonSuccess, CurrentRow: 2, CurrentStage: "3.Implementation"}

	consultManual(t, d, stagedRoutingTable(), req) //nolint:errcheck

	got := d.oneQ(domain.QuestionManualStage, 0).DefaultOptionID
	if got != "3" {
		t.Errorf("want stage preselection %q, got %q", "3", got)
	}
}
