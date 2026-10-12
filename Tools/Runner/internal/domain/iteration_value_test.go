package domain_test

// Tests for domain.ReadIterationValue and domain.FormatIterationValue: the one
// reader of a recorded Iteration value, which splits a value only with the
// recorded row, and the writer of the 3.0 spelling.
//
// A recorded value is read only in the 3.0 spelling:
//   - grouped block row: "{ID}.{Group}" ("{Group}.{ID}" is refused)
//   - ungrouped block row: "{ID}" verbatim, so "Stage-1" is the ID "Stage-1"
//   - row outside every block: "" and "-"

import (
	"errors"
	"testing"

	"mosaic-run/internal/domain"
)

// ivGroupedRow is a parsed row of group Test in block Stage.
func ivGroupedRow() domain.RoutingRow {
	return domain.RoutingRow{
		Index: 7,
		PhaseParsed: domain.PhaseParsed{
			Name: "EXECUTION", IsStaged: true, Group: "Test", Blocks: []domain.BlockName{"Stage"},
		},
	}
}

// ivUngroupedRow is a parsed row of the ungrouped block Stage.
func ivUngroupedRow() domain.RoutingRow {
	return domain.RoutingRow{
		Index: 3,
		PhaseParsed: domain.PhaseParsed{
			Name: "BUILD", IsStaged: true, Blocks: []domain.BlockName{"Stage"},
		},
	}
}

// ivOutsideRow is a row outside every block.
func ivOutsideRow() domain.RoutingRow {
	return domain.RoutingRow{Index: 0, PhaseParsed: domain.PhaseParsed{Name: "PLANNING"}}
}

type ivReadCase struct {
	name      string
	recorded  string
	wantID    domain.IterationID
	wantGroup domain.GroupName
}

func requireReadsAs(t *testing.T, row domain.RoutingRow, cases []ivReadCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ReadIterationValue(tc.recorded, row)

			if err != nil {
				t.Fatalf("ReadIterationValue(%q): want a value, got error %v", tc.recorded, err)
			}
			if got.ID != tc.wantID || got.Group != tc.wantGroup {
				t.Errorf("ReadIterationValue(%q): want ID %q group %q, got ID %q group %q",
					tc.recorded, tc.wantID, tc.wantGroup, got.ID, got.Group)
			}
		})
	}
}

type ivRejectCase struct {
	name     string
	recorded string
	want     domain.IterationValueKind
}

func requireRejected(t *testing.T, row domain.RoutingRow, cases []ivRejectCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ReadIterationValue(tc.recorded, row)

			var verr *domain.IterationValueError
			if !errors.As(err, &verr) {
				t.Fatalf("ReadIterationValue(%q): want *IterationValueError, got %v", tc.recorded, err)
			}
			if verr.Kind != tc.want {
				t.Errorf("ReadIterationValue(%q): want kind %q, got %q", tc.recorded, tc.want, verr.Kind)
			}
			if verr.Recorded != tc.recorded {
				t.Errorf("error must carry the recorded value verbatim: want %q, got %q", tc.recorded, verr.Recorded)
			}
			if want := domain.WorkflowRowFromIndex(row.Index); verr.Row != want {
				t.Errorf("error must name the row: want %d, got %d", want, verr.Row)
			}
		})
	}
}

func TestReadIterationValue_GroupedRow_ReadsTheThreeOhSpelling(t *testing.T) {
	requireReadsAs(t, ivGroupedRow(), []ivReadCase{
		{"numeric ID", "1.Test", "1", "Test"},
		{"non-numeric ID", "Acme.Test", "Acme", "Test"},
		{"ID with a dash", "b-2.Test", "b-2", "Test"},
		{"ID is returned as spelled", "a.Test", "a", "Test"},
		{"ID equal to the group name", "Test.Test", "Test", "Test"},
	})
}

func TestReadIterationValue_GroupedRow_RejectsThePreThreeOhSpelling(t *testing.T) {
	requireRejected(t, ivGroupedRow(), []ivRejectCase{
		{"numeric ID", "Test.1", domain.IterationValueMalformed},
		{"non-numeric ID", "Test.Acme", domain.IterationValueMalformed},
		{"ID with a dash", "Test.b-2", domain.IterationValueMalformed},
	})
}

func TestReadIterationValue_GroupedRow_RejectsValuesTheRowDoesNotAccept(t *testing.T) {
	requireRejected(t, ivGroupedRow(), []ivRejectCase{
		{"empty value", "", domain.IterationValueMissing},
		{"dash", "-", domain.IterationValueMissing},
		{"ID without group", "1", domain.IterationValueMalformed},
		{"3.0 form with another group", "1.Wrong", domain.IterationValueMalformed},
		{"pre-3.0 form with another group", "Wrong.1", domain.IterationValueMalformed},
		{"group differing in case", "1.test", domain.IterationValueMalformed},
		{"extra segment after the group", "1.Test.x", domain.IterationValueMalformed},
		{"extra segment before the group", "x.1.Test", domain.IterationValueMalformed},
		{"empty group segment", "1.", domain.IterationValueMalformed},
		{"empty ID segment", ".Test", domain.IterationValueMalformed},
		{"ID breaking the ID character set", "-1.Test", domain.IterationValueMalformed},
		{"legacy stage form", "Stage-1", domain.IterationValueMalformed},
	})
}

// "Stage-1" is an ordinary schedule ID (letters, digits and dashes): it is
// read verbatim and never as the ID "1".
func TestReadIterationValue_UngroupedBlockRow_ReadsTheValueVerbatimAsID(t *testing.T) {
	requireReadsAs(t, ivUngroupedRow(), []ivReadCase{
		{"numeric ID", "1", "1", ""},
		{"non-numeric ID", "Acme", "Acme", ""},
		{"ID with a dash", "a-b", "a-b", ""},
		{"ID starting with Stage-", "Stage-1", "Stage-1", ""},
		{"ID starting with Stage- and a non-numeric rest", "Stage-Acme", "Stage-Acme", ""},
		{"Stage- prefix in another case is just an ID", "stage-1", "stage-1", ""},
	})
}

func TestReadIterationValue_UngroupedBlockRow_RejectsValuesTheRowDoesNotAccept(t *testing.T) {
	requireRejected(t, ivUngroupedRow(), []ivRejectCase{
		{"empty value", "", domain.IterationValueMissing},
		{"dash", "-", domain.IterationValueMissing},
		{"group segment on an ungrouped row, 3.0 spelling", "1.Test", domain.IterationValueMalformed},
		{"group segment on an ungrouped row, pre-3.0 spelling", "Test.1", domain.IterationValueMalformed},
		{"ID starting with a dash", "-1", domain.IterationValueMalformed},
		{"ID with a space", "a b", domain.IterationValueMalformed},
	})
}

func TestReadIterationValue_RowOutsideEveryBlock_AcceptsOnlyNoIteration(t *testing.T) {
	for _, recorded := range []string{"", "-"} {
		t.Run("reads "+recorded, func(t *testing.T) {
			got, err := domain.ReadIterationValue(recorded, ivOutsideRow())

			if err != nil {
				t.Fatalf("ReadIterationValue(%q): want the zero value, got error %v", recorded, err)
			}
			if !got.IsZero() {
				t.Errorf("ReadIterationValue(%q): want the zero value, got %+v", recorded, got)
			}
		})
	}
	requireRejected(t, ivOutsideRow(), []ivRejectCase{
		{"an ID", "1", domain.IterationValueOnNonBlockRow},
		{"3.0 value", "1.Test", domain.IterationValueOnNonBlockRow},
		{"pre-3.0 value", "Test.1", domain.IterationValueOnNonBlockRow},
		{"legacy stage form", "Stage-1", domain.IterationValueOnNonBlockRow},
	})
}

// A hand-built row whose phase text names nothing is a block row of the block
// and group its parsed phase carries; the reader never looks at the Phase text.
func TestReadIterationValue_HandBuiltRowWithUnrelatedPhaseText_ReadsLikeTheParsedRow(t *testing.T) {
	grouped := domain.RoutingRow{
		Index: 4, Phase: "something else",
		PhaseParsed: domain.PhaseParsed{IsStaged: true, Group: "Test", Blocks: []domain.BlockName{"Stage"}},
	}
	ungrouped := domain.RoutingRow{
		Index: 5, PhaseParsed: domain.PhaseParsed{IsStaged: true, Blocks: []domain.BlockName{"Stage"}},
	}

	requireReadsAs(t, grouped, []ivReadCase{
		{"grouped 3.0", "A.Test", "A", "Test"},
	})
	requireReadsAs(t, ungrouped, []ivReadCase{{"ungrouped", "A", "A", ""}})
	requireRejected(t, grouped, []ivRejectCase{
		{"grouped without group", "A", domain.IterationValueMalformed},
		{"grouped pre-3.0", "Test.A", domain.IterationValueMalformed},
	})
}

func TestReadIterationValue_GroupNamedLikeAnID_ReadsTheSegmentOfTheRowGroupAsGroup(t *testing.T) {
	row := ivGroupedRow()
	row.PhaseParsed.Group = "Acme"

	requireReadsAs(t, row, []ivReadCase{
		{"the last segment is the group", "Test.Acme", "Test", "Acme"},
		{"both segments equal the group", "Acme.Acme", "Acme", "Acme"},
	})
	requireRejected(t, row, []ivRejectCase{
		{"pre-3.0: the first segment is the group", "Acme.Test", domain.IterationValueMalformed},
	})
}

func TestIterationValue_EqualIgnoresIDCaseAndComparesGroupExactly(t *testing.T) {
	base := domain.IterationValue{ID: "A", Group: "Test"}
	cases := []struct {
		name  string
		other domain.IterationValue
		want  bool
	}{
		{"same", domain.IterationValue{ID: "A", Group: "Test"}, true},
		{"ID in another case", domain.IterationValue{ID: "a", Group: "Test"}, true},
		{"another ID", domain.IterationValue{ID: "B", Group: "Test"}, false},
		{"group in another case", domain.IterationValue{ID: "A", Group: "test"}, false},
		{"no group", domain.IterationValue{ID: "A"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.Equal(tc.other); got != tc.want {
				t.Errorf("%+v.Equal(%+v): want %v, got %v", base, tc.other, tc.want, got)
			}
		})
	}
}

func TestIterationValue_IsZeroOnlyWithoutIDAndGroup(t *testing.T) {
	if !(domain.IterationValue{}).IsZero() {
		t.Errorf("the zero value must report IsZero")
	}
	if (domain.IterationValue{ID: "1"}).IsZero() {
		t.Errorf("a value with an ID must not report IsZero")
	}
}

func TestFormatIterationValue_RendersTheThreeOhSpelling(t *testing.T) {
	cases := []struct {
		name string
		v    domain.IterationValue
		want string
	}{
		{"numeric ID with group", domain.IterationValue{ID: "1", Group: "Test"}, "1.Test"},
		{"numeric ID without group", domain.IterationValue{ID: "1"}, "1"},
		{"non-numeric ID without group", domain.IterationValue{ID: "Acme"}, "Acme"},
		{"non-numeric ID with group", domain.IterationValue{ID: "Acme", Group: "Test"}, "Acme.Test"},
		{"no iteration", domain.IterationValue{}, "-"},
		{"group without an ID names no iteration", domain.IterationValue{Group: "Test"}, "-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.FormatIterationValue(tc.v); got != tc.want {
				t.Errorf("FormatIterationValue(%+v): want %q, got %q", tc.v, tc.want, got)
			}
		})
	}
}

func TestFormatIterationValue_ResultIsReadBackWithTheSameRow(t *testing.T) {
	cases := []struct {
		name string
		row  domain.RoutingRow
		v    domain.IterationValue
	}{
		{"grouped", ivGroupedRow(), domain.IterationValue{ID: "Acme", Group: "Test"}},
		{"ID equal to the group", ivGroupedRow(), domain.IterationValue{ID: "Test", Group: "Test"}},
		{"ungrouped", ivUngroupedRow(), domain.IterationValue{ID: "b-2"}},
		{"outside every block", ivOutsideRow(), domain.IterationValue{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorded := domain.FormatIterationValue(tc.v)

			got, err := domain.ReadIterationValue(recorded, tc.row)

			if err != nil {
				t.Fatalf("ReadIterationValue(%q): %v", recorded, err)
			}
			if !got.Equal(tc.v) {
				t.Errorf("round trip through %q: want %+v, got %+v", recorded, tc.v, got)
			}
		})
	}
}
