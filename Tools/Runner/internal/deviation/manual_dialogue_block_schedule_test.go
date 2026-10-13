package deviation_test

// Tests for the stage question of the manual dialogue over a table with two
// blocks: the offered iterations, the answer mapping and the preselection come
// from the schedule of the chosen row's own block, by iteration ID.
//
// Rows of twoBlockTable (1-based): 1 dev (Alpha, group Test), 2 dev (Beta,
// group Impl), 3 reviewer (outside every block). Alpha schedules "1", "2",
// "setup-b"; Beta schedules "b1", "B2".

import (
	"slices"
	"testing"

	"mosaic-run/internal/domain"
)

// consultTwoBlockManual runs the dialogue over the two-block table.
func consultTwoBlockManual(t *testing.T, d *dialogue, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	t.Helper()
	return consultManual(t, d, twoBlockTable(), req)
}

func TestManualDialogue_StageQuestion_OffersTheIDsOfTheChosenRowsBlock(t *testing.T) {
	cases := []struct {
		name string
		row  string
		want []string
	}{
		{"row of the first block", "1", []string{"1", "2", "setup-b"}},
		{"row of the second block", "2", []string{"b1", "B2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard(tc.row, tc.want[0])

			consultTwoBlockManual(t, d, manualRequest(twoBlockSchedules())) //nolint:errcheck

			got := optionIDs(d.oneQ(domain.QuestionManualStage, 0))
			if !slices.Equal(got, tc.want) {
				t.Errorf("want the stage options %v, got %v", tc.want, got)
			}
		})
	}
}

func TestManualDialogue_StageAnswer_MapsToTheIDTheBlocksScheduleSpellsIt(t *testing.T) {
	cases := []struct {
		name    string
		row     string
		answer  oneFn
		wantID  domain.IterationID
		wantRow int
	}{
		{"offered non-numeric ID", "1", pick("setup-b"), "setup-b", 0},
		{"offered ID of the second block", "2", pick("b1"), "b1", 1},
		{"typed ID in other letter case", "2", typed("b2"), "B2", 1},
		{"typed non-numeric ID", "1", typed("SETUP-B"), "setup-b", 0},
		{"offered numeric ID", "1", pick("2"), "2", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard(tc.row, "")
			d.one[domain.QuestionManualStage] = []oneFn{tc.answer}

			instr, err := consultTwoBlockManual(t, d, manualRequest(twoBlockSchedules()))

			disp := requireDispatch(t, instr, err)
			if disp.RowIndex != tc.wantRow || disp.Iteration != tc.wantID {
				t.Errorf("want RowIndex=%d Iteration=%q, got RowIndex=%d Iteration=%q",
					tc.wantRow, tc.wantID, disp.RowIndex, disp.Iteration)
			}
		})
	}
}

// An ID that exists only in the other block's schedule is outside the chosen
// row's schedule: the dialogue asks the stage again and then dispatches the
// corrected answer.
func TestManualDialogue_TypedIDOfAnotherBlock_IsRejectedAndTheStageAskedAgain(t *testing.T) {
	d := newDialogue(t).standard("2", "")
	d.one[domain.QuestionManualStage] = []oneFn{typed("setup-b"), pick("B2")}

	instr, err := consultTwoBlockManual(t, d, manualRequest(twoBlockSchedules()))

	disp := requireDispatch(t, instr, err)
	if len(d.oneQs[domain.QuestionManualStage]) < 2 {
		t.Fatalf("want the stage asked again after the rejected ID, got sequence %v", d.calls)
	}
	requireReasonShown(t, d, domain.QuestionManualStage, 1)
	if disp.Iteration != "B2" {
		t.Errorf("want the corrected ID B2 dispatched, got %q", disp.Iteration)
	}
}

func TestManualDialogue_StageQuestion_PreselectsTheDeviatingIDOfTheRowsOwnBlock(t *testing.T) {
	firstIDOfRow := map[string]string{"1": "1", "2": "b1"}
	cases := []struct {
		name         string
		row          string
		currentStage string
		want         string
	}{
		{"non-numeric ID of the row's block", "2", "b1.Impl", "b1"},
		{"ID in other letter case is preselected as spelled by the schedule", "2", "b2.Impl", "B2"},
		{"non-numeric ID in the first block", "1", "setup-b.Test", "setup-b"},
		{"ID that belongs to another block's schedule", "2", "setup-b.Impl", ""},
		{"ID of the second block on a row of the first", "1", "b1.Test", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The answer is irrelevant here: only the preselection is asserted.
			d := newDialogue(t).standard(tc.row, "")
			d.one[domain.QuestionManualStage] = []oneFn{pick(firstIDOfRow[tc.row])}
			req := manualRequest(twoBlockSchedules())
			// The deviation is on the chosen row; its recorded value is read with it.
			req.Deviation = &domain.DeviationInfo{
				CurrentRow: map[string]int{"1": 0, "2": 1}[tc.row], CurrentStage: tc.currentStage,
			}

			consultTwoBlockManual(t, d, req) //nolint:errcheck

			got := d.oneQ(domain.QuestionManualStage, 0).DefaultOptionID
			if got != tc.want {
				t.Errorf("want stage preselection %q, got %q", tc.want, got)
			}
		})
	}
}
