package tui

// The progress screen reads the iteration of a step from the "iteration" key of
// the session notice message, in the value form the session writes: the 3.0
// value `{ID}[.{Group}]`, or an empty value for a row outside every block.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// progressRowFromNotice drives one notice through a progress screen and
// returns the single row it produced.
func progressRowFromNotice(t *testing.T, message string) progressRow {
	t.Helper()
	m := newFlowModel(domain.RunOutcome{Status: domain.RunCompleted})
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress

	m.Update(noticeMsg("agent#1", message))

	rows := m.progressScreen.Rows()
	if len(rows) != 1 {
		t.Fatalf("want one progress row after a running notice, got %d: %+v", len(rows), rows)
	}
	return rows[0]
}

func TestNoticeIteration_RunningNotice_RowCarriesTheIterationValue(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{"grouped block row", `phase=EXECUTION iteration="1.Test" status=running`, "1.Test"},
		{"ungrouped block row", `phase=EXECUTION iteration="1" status=running`, "1"},
		{"non-numeric ID", `phase=EXECUTION iteration="A" status=running`, "A"},
		{"row outside every block", `phase=PLANNING iteration="" status=running`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := progressRowFromNotice(t, tc.message)

			if row.Stage != tc.want {
				t.Errorf("row iteration for %q = %q, want %q", tc.message, row.Stage, tc.want)
			}
			if row.Phase == "" {
				t.Errorf("row phase empty for %q, want the phase of the notice", tc.message)
			}
		})
	}
}

func TestNoticeIteration_RunningNoticeWithOnlyTheOldKey_RowCarriesNoIteration(t *testing.T) {
	row := progressRowFromNotice(t, `phase=EXECUTION stage="1.Test" status=running`)

	if row.Stage != "" {
		t.Errorf("row iteration = %q, want empty: the iteration is read from the iteration key only", row.Stage)
	}
}

func TestExtractField_IterationKey_ReadsQuotedValue(t *testing.T) {
	msg := `phase=EXECUTION iteration="2.Implementation" status=SUCCESS`

	if got := extractField(msg, "iteration"); got != "2.Implementation" {
		t.Errorf("extractField(iteration) = %q, want %q", got, "2.Implementation")
	}
}
