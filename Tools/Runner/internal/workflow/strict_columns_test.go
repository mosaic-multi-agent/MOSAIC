package workflow_test

// Strict parsing of the routing table's columns: the required set (T2), no
// other column but Waits For and the deployed Row column (T11), no empty cell
// (T12), Waits For only when some row joins (T13), and the Row column (T4).

import (
	"fmt"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

func TestParseStrict_MissingRequiredColumn_IsRefusedNamingTheColumn(t *testing.T) {
	for _, column := range cols7 {
		t.Run(column, func(t *testing.T) {
			rows := [][]string{plain7("PLANNING", "agent-a"), plain7("REVIEW", "agent-b")}
			header, rows := dropColumn(cols7, rows, column)

			_, err := parseStrict(cellsTable(header, rows...))

			re := requireRefusal(t, err, refusalWant{rule: "T2", token: column})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_UnknownColumn_IsRefusedNamingTheColumn(t *testing.T) {
	_, err := parseStrict(unknownColumnTable())

	re := requireRefusal(t, err, refusalWant{rule: "T11", token: "Notes"})
	requireRowOrToken(t, re)
}

func TestParseStrict_WaitsForColumnWhenSomeRowJoins_IsAccepted(t *testing.T) {
	table := mustParseStrict(t, table8(joinRows("agent-b, agent-c")...))

	if len(table.Rows) != 4 {
		t.Fatalf("want 4 rows, got %d", len(table.Rows))
	}
	if got := table.Rows[3].WaitsFor; len(got) != 2 {
		t.Errorf("join row WaitsFor: want 2 entries, got %+v", got)
	}
}

func TestParseStrict_WaitsForColumnWithoutAnyJoin_IsRefused(t *testing.T) {
	cases := map[string][][]string{
		"every cell is -": {
			row8("PLANNING", "agent-a", "FALSE", "NEXT", "-", "-"),
			row8("REVIEW", "agent-b", "FALSE", "COMPLETE", "-", "-"),
		},
		"single row": {
			row8("PLANNING", "agent-a", "FALSE", "COMPLETE", "-", "-"),
		},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(table8(rows...))

			re := requireRefusal(t, err, refusalWant{rule: "T13", token: "Waits For"})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_EmptyCell_IsRefusedNamingTheRow(t *testing.T) {
	cases := []struct {
		column string
		anyOf  []string
	}{
		{"On Findings", []string{"T12"}},
		{"Input", []string{"T12"}},
		{"Output", []string{"T12"}},
		{"On Success", []string{"T5", "T12"}},
		{"HITL", []string{"T7", "T12"}},
		{"Subagent", []string{"T6", "T12"}},
		{"Phase", []string{"T12", "P12"}},
	}
	for _, tc := range cases {
		t.Run(tc.column, func(t *testing.T) {
			_, err := parseStrict(threeRowsWith(tc.column, ""))

			re := requireRefusal(t, err, refusalWant{anyOf: tc.anyOf, row: 2})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_EmptyWaitsForCell_IsRefused(t *testing.T) {
	_, err := parseStrict(table8(joinRows("")...))

	requireRefusal(t, err, refusalWant{rule: "T12", row: 4})
}

// ---- Row column (deployed tables) ----

// withRowColumn renders cols7 rows with a Row column inserted at pos holding
// values[i] for data row i.
func withRowColumn(pos int, values []string, rows ...[]string) string {
	insert := func(cells []string, v string) []string {
		out := append([]string{}, cells[:pos]...)
		out = append(out, v)
		return append(out, cells[pos:]...)
	}
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = insert(r, values[i])
	}
	return cellsTable(insert(cols7, "Row"), out...)
}

func fourPlainRows() [][]string {
	return [][]string{
		plain7("PLANNING", "agent-a"), plain7("PLANNING", "agent-b"),
		plain7("REVIEW", "agent-c"), plain7("REVIEW", "agent-d"),
	}
}

func TestParseStrict_CorrectRowColumn_AnyPosition_SameModelAsWithout(t *testing.T) {
	without := mustParseStrict(t, table7(fourPlainRows()...))
	positions := map[string]int{"first": 0, "middle": 3, "last": len(cols7)}
	for name, pos := range positions {
		t.Run(name, func(t *testing.T) {
			content := withRowColumn(pos, []string{"1", "2", "3", "4"}, fourPlainRows()...)

			with := mustParseStrict(t, content)

			if !reflect.DeepEqual(with.Rows, without.Rows) {
				t.Errorf("rows differ with a Row column at %d:\nwith:    %+v\nwithout: %+v", pos, with.Rows, without.Rows)
			}
		})
	}
}

func TestParseStrict_WrongRowColumnValue_IsRefusedWithRowAndCellText(t *testing.T) {
	cases := []struct {
		name    string
		values  []string
		wantRow int
		token   string
	}{
		{"non-numeric", []string{"1", "2", "abc", "4"}, 3, "abc"},
		{"gap", []string{"1", "2", "4", "5"}, 3, "4"},
		{"duplicate", []string{"1", "2", "3", "3"}, 4, "3"},
		{"zero-based", []string{"0", "1", "2", "3"}, 1, "0"},
		{"leading-zero", []string{"1", "2", "03", "4"}, 3, "03"},
	}
	for _, tc := range cases {
		for _, pos := range []int{0, 3} {
			t.Run(fmt.Sprintf("%s/position-%d", tc.name, pos), func(t *testing.T) {
				_, err := parseStrict(withRowColumn(pos, tc.values, fourPlainRows()...))

				requireRefusal(t, err, refusalWant{rule: "T4", row: tc.wantRow, token: tc.token})
			})
		}
	}
}

func TestParseStrict_RowColumnOnJoinTable_KeepsTypedEntries(t *testing.T) {
	// The Row column is neither an unknown column (T11) nor part of the
	// typed routing model: targets still resolve to zero-based row indices.
	header := []string{"Row", "Phase", "Subagent", "HITL", "On Success", "On Findings", "Waits For", "Input", "Output"}
	rows := joinRows("agent-b, agent-c")
	for i := range rows {
		rows[i] = append([]string{fmt.Sprint(i + 1)}, rows[i]...)
	}

	table := mustParseStrict(t, cellsTable(header, rows...))

	want := []domain.RowRef{ref("agent-b", 1), ref("agent-c", 2)}
	if !reflect.DeepEqual(table.Rows[3].WaitsFor, want) {
		t.Errorf("join WaitsFor: want %+v, got %+v", want, table.Rows[3].WaitsFor)
	}
	if table.Rows[0].Success.Kind != domain.SuccessFork {
		t.Errorf("row 1 Success.Kind: want fork, got %q", table.Rows[0].Success.Kind)
	}
}
