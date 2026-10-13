package deviation_test

// The stage question of the manual dialogue preselects the iteration of the
// deviation. The deviation's recorded value is read with the deviation's own
// row, in the 3.0 spelling "{ID}.{Group}"; a value its row does not accept,
// including the pre-3.0 "{Group}.{ID}", gives no preselection.
//
// Rows of twoBlockTable (zero-based): 0 dev (Alpha, group Test), 1 dev (Beta,
// group Impl). Alpha schedules "1", "2", "setup-b"; Beta schedules "b1", "B2".

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestManualDialogue_StagePreselection_ReadsTheDeviationValueWithItsRow(t *testing.T) {
	cases := []struct {
		name         string
		row          string // chosen row, 1-based
		currentRow   int    // deviating row, zero-based
		currentStage string
		want         string
	}{
		{"3.0 non-numeric ID", "2", 1, "b1.Impl", "b1"},
		{"pre-3.0 non-numeric ID gives no preselection", "2", 1, "Impl.b1", ""},
		{"3.0 ID in other letter case gives the schedule's spelling", "2", 1, "b2.Impl", "B2"},
		{"3.0 ID with a dash in the first block", "1", 0, "setup-b.Test", "setup-b"},
		{"3.0 ID that belongs to another block's schedule", "2", 1, "setup-b.Impl", ""},
		{"3.0 value with the group of another row", "2", 1, "b1.Test", ""},
		{"ID without group on a grouped row", "2", 1, "b1", ""},
		{"3.0 value with an extra segment", "2", 1, "b1.Impl.x", ""},
		{"3.0 value on a deviating row outside every block", "2", 2, "b1.Impl", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard(tc.row, "")
			d.one[domain.QuestionManualStage] = []oneFn{pick("b1")}
			req := manualRequest(twoBlockSchedules())
			req.Deviation = &domain.DeviationInfo{CurrentRow: tc.currentRow, CurrentStage: tc.currentStage}

			consultTwoBlockManual(t, d, req) //nolint:errcheck

			got := d.oneQ(domain.QuestionManualStage, 0).DefaultOptionID
			if got != tc.want {
				t.Errorf("want stage preselection %q, got %q", tc.want, got)
			}
		})
	}
}

func TestManualDialogue_StagePreselection_NumericIDInTheThreeOhSpellingOnly(t *testing.T) {
	cases := []struct {
		name         string
		currentStage string
		want         string
	}{
		{"3.0 value of the same group", "3.Implementation", "3"},
		{"pre-3.0 value of the same group gives no preselection", "Implementation.3", ""},
		{"3.0 value of another group", "3.Test", ""},
		{"3.0 value, ID not in the set", "9.Implementation", ""},
		{"ID without group", "3", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDialogue(t).standard("3", "1")
			req := manualRequest(schedulesUpTo(4))
			req.Deviation = &domain.DeviationInfo{CurrentRow: 2, CurrentStage: tc.currentStage}

			consultManual(t, d, stagedRoutingTable(), req) //nolint:errcheck

			got := d.oneQ(domain.QuestionManualStage, 0).DefaultOptionID
			if got != tc.want {
				t.Errorf("want stage preselection %q, got %q", tc.want, got)
			}
		})
	}
}
