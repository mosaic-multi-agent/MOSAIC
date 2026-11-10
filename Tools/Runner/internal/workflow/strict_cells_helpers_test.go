package workflow_test

// Fixtures and assertions for the routing-cell tests: tables rendered from
// explicit cell values, so one cell can be varied while the rest stays valid.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// cols7 and cols8 are the 2.16 source column sets; cols8 carries Waits For.
var cols7 = []string{"Phase", "Subagent", "HITL", "On Success", "On Findings", "Input", "Output"}
var cols8 = []string{"Phase", "Subagent", "HITL", "On Success", "On Findings", "Waits For", "Input", "Output"}

// row7 is a 7-column data row with Input "-" and Output "out.md".
func row7(phase, agent, hitl, onSuccess, onFindings string) []string {
	return []string{phase, agent, hitl, onSuccess, onFindings, "-", "out.md"}
}

// row8 is an 8-column data row with Input "-" and Output "out.md".
func row8(phase, agent, hitl, onSuccess, onFindings, waitsFor string) []string {
	return []string{phase, agent, hitl, onSuccess, onFindings, waitsFor, "-", "out.md"}
}

// plain7 is a valid plain row: FALSE HITL, NEXT, no findings target.
func plain7(phase, agent string) []string { return row7(phase, agent, "FALSE", "NEXT", "-") }

func cellsLine(cells []string) string { return "| " + strings.Join(cells, " | ") + " |\n" }

// cellsTable renders a routing table: header, separator, then the rows.
func cellsTable(header []string, rows ...[]string) string {
	sep := make([]string, len(header))
	for i := range sep {
		sep[i] = "---"
	}
	var b strings.Builder
	b.WriteString(cellsLine(header))
	b.WriteString(cellsLine(sep))
	for _, r := range rows {
		b.WriteString(cellsLine(r))
	}
	return b.String()
}

// table7 and table8 render a 7- or 8-column routing table.
func table7(rows ...[]string) string { return cellsTable(cols7, rows...) }
func table8(rows ...[]string) string { return cellsTable(cols8, rows...) }

// dropColumn removes the column called name from the header and every row.
func dropColumn(header []string, rows [][]string, name string) ([]string, [][]string) {
	at := -1
	for i, h := range header {
		if h == name {
			at = i
		}
	}
	cut := func(cells []string) []string {
		out := append([]string{}, cells[:at]...)
		return append(out, cells[at+1:]...)
	}
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = cut(r)
	}
	return cut(header), out
}

// ref builds the RowRef a resolved target must carry.
func ref(cell string, index int) domain.RowRef { return domain.RowRef{Cell: cell, Index: index} }

// requireRowIn asserts the refusal names one of the given 1-based rows.
func requireRowIn(t *testing.T, re *domain.RefusalError, rows ...int) {
	t.Helper()
	for _, r := range rows {
		if re.Row == domain.WorkflowRow(r) {
			return
		}
	}
	t.Errorf("Row: want one of %v (1-based), got %d", rows, re.Row)
}

// threeRowsWith builds a plain three-row table with distinct agents and one
// cell of row 2 replaced: column is a name from cols7.
func threeRowsWith(column, value string) string {
	rows := [][]string{plain7("PLANNING", "agent-a"), plain7("PLANNING", "agent-b"), plain7("REVIEW", "agent-c")}
	for i, h := range cols7 {
		if h == column {
			rows[1][i] = value
		}
	}
	return table7(rows...)
}

// joinRows is a fork-and-join table: row 1 forks to rows 2 and 3, row 4 joins.
func joinRows(waitsFor string) [][]string {
	return [][]string{
		row8("PLANNING", "agent-a", "FALSE", "agent-b, agent-c", "-", "-"),
		row8("RESEARCH", "agent-b", "FALSE", "agent-d", "-", "-"),
		row8("RESEARCH", "agent-c", "FALSE", "agent-d", "-", "-"),
		row8("REVIEW", "agent-d", "FALSE", "COMPLETE", "-", waitsFor),
	}
}

// missingColumnTable is a valid two-row table without the named column.
func missingColumnTable(column string) string {
	rows := [][]string{plain7("PLANNING", "agent-a"), plain7("REVIEW", "agent-b")}
	header, rows := dropColumn(cols7, rows, column)
	return cellsTable(header, rows...)
}

// unknownColumnTable is a valid two-row table with an extra Notes column.
func unknownColumnTable() string {
	header := append(append([]string{}, cols7...), "Notes")
	return cellsTable(header,
		append(plain7("PLANNING", "agent-a"), "x"),
		append(plain7("REVIEW", "agent-b"), "y"),
	)
}
