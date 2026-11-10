package workflow_test

// Tests for the optional Row column of workflow.Parse.
//
// A deployed workflow table may carry a Row column holding each data row's
// 1-based position. Authored tables do not have it. The column is validated,
// never stored: RoutingRow.Index stays the row identity. A wrong value is
// refused with rule T4, the 1-based position of the row and the cell text.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// rowColumnData is the fixed table content (Phase, Subagent, HITL, On Success, On Findings, Input, Output)
// shared by every Row-column test; only the Row column varies.
var rowColumnData = [][]string{
	{"PLANNING", "planner", "FALSE", "NEXT", "-", "-", "Plan.md"},
	{"PLANNING", "plan-review", "FALSE", "NEXT", "-", "Plan.md", "plan-review.md"},
	{"REVIEW", "final-review", "FALSE", "NEXT", "-", "Plan.md", "final.md"},
	{"REVIEW", "summariser", "FALSE", "COMPLETE", "-", "final.md", "summary.md"},
}

var rowColumnHeader = []string{"Phase", "Subagent", "HITL", "On Success", "On Findings", "Input", "Output"}

// buildRowColumnTable renders the shared table. When pos is negative no Row
// column is emitted; otherwise a Row column is inserted at index pos and data
// row i holds values[i].
func buildRowColumnTable(pos int, values []string) string {
	insert := func(cells []string, v string) []string {
		if pos < 0 {
			return cells
		}
		out := append([]string{}, cells[:pos]...)
		out = append(out, v)
		return append(out, cells[pos:]...)
	}
	line := func(cells []string) string { return "| " + strings.Join(cells, " | ") + " |\n" }

	sep := make([]string, len(rowColumnHeader))
	for i := range sep {
		sep[i] = "---"
	}

	var b strings.Builder
	b.WriteString(line(insert(rowColumnHeader, "Row")))
	b.WriteString(line(insert(sep, "---")))
	for i, data := range rowColumnData {
		v := ""
		if pos >= 0 {
			v = values[i]
		}
		b.WriteString(line(insert(data, v)))
	}
	return b.String()
}

func correctRowValues() []string {
	vals := make([]string, len(rowColumnData))
	for i := range vals {
		vals[i] = fmt.Sprint(i + 1)
	}
	return vals
}

func TestParse_RowColumnAbsent_ParsesAllRowsInOrder(t *testing.T) {
	table := mustParseStrict(t, buildRowColumnTable(-1, nil))

	if len(table.Rows) != len(rowColumnData) {
		t.Fatalf("want %d rows, got %d", len(rowColumnData), len(table.Rows))
	}
	for i, row := range table.Rows {
		if row.Index != i || row.Agent != rowColumnData[i][1] {
			t.Errorf("Rows[%d]: want Index %d agent %q, got Index %d agent %q",
				i, i, rowColumnData[i][1], row.Index, row.Agent)
		}
	}
}

func TestParse_RowColumnCorrect_AnyPosition_SameRoutingModelAsWithout(t *testing.T) {
	without := mustParseStrict(t, buildRowColumnTable(-1, nil))

	positions := map[string]int{"first": 0, "middle": 2, "last": len(rowColumnHeader)}
	for name, pos := range positions {
		t.Run(name, func(t *testing.T) {
			with := mustParseStrict(t, buildRowColumnTable(pos, correctRowValues()))

			if !reflect.DeepEqual(with.Rows, without.Rows) {
				t.Errorf("routing rows differ with a Row column at %d:\nwith:    %+v\nwithout: %+v",
					pos, with.Rows, without.Rows)
			}
		})
	}
}

func TestParse_RowColumnWrongValue_ReturnsRefusalNamingWorkflowAndRow(t *testing.T) {
	cases := []struct {
		name    string
		values  []string
		wantRow int // 1-based row holding the first wrong value
	}{
		{"non-numeric", []string{"1", "2", "abc", "4"}, 3},
		{"gap", []string{"1", "2", "4", "5"}, 3},
		{"duplicate", []string{"1", "2", "3", "3"}, 4},
		{"out-of-order", []string{"1", "3", "2", "4"}, 2},
		{"wrong-start-number", []string{"2", "3", "4", "5"}, 1},
		{"zero-based", []string{"0", "1", "2", "3"}, 1},
		{"leading-zero", []string{"1", "2", "03", "4"}, 3},
		{"explicit-plus-sign", []string{"1", "2", "+3", "4"}, 3},
		{"decimal-fraction", []string{"1.0", "2", "3", "4"}, 1},
	}

	for _, tc := range cases {
		for _, pos := range []int{0, 2} {
			t.Run(fmt.Sprintf("%s/position-%d", tc.name, pos), func(t *testing.T) {
				_, err := parseStrict(buildRowColumnTable(pos, tc.values))

				re := requireRefusal(t, err, refusalWant{rule: "T4", row: tc.wantRow})
				requireRowOrToken(t, re)
			})
		}
	}
}

func TestParse_RowColumnMissingValue_IsRefusedAtTheEmptyCell(t *testing.T) {
	// An empty Row cell is an empty cell (T12) or a wrong Row value (T4); either
	// way the refusal points at row 3.
	_, err := parseStrict(buildRowColumnTable(0, []string{"1", "2", "", "4"}))

	requireRefusal(t, err, refusalWant{anyOf: []string{"T4", "T12"}, row: 3})
}
