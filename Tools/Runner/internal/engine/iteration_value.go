package engine

import "mosaic-run/internal/domain"

// tableRow returns the routing-table row at idx, and false when idx is outside
// the table.
func tableRow(workflow domain.AdmittedWorkflow, idx int) (domain.RoutingRow, bool) {
	if idx < 0 || idx >= len(workflow.Table.Rows) {
		return domain.RoutingRow{}, false
	}
	return workflow.Table.Rows[idx], true
}

// entryRowForValue is the row an entry's recorded value is read with: the
// entry's recorded row, or the fallback row for an entry that records none.
// ok is false when the entry records a row outside the table, or records none
// and no fallback applies.
func entryRowForValue(
	workflow domain.AdmittedWorkflow,
	e domain.ExecutionLogEntry,
	fallback *domain.RoutingRow,
) (row domain.RoutingRow, ok bool) {
	if e.WorkflowRow == domain.NoWorkflowRow {
		if fallback == nil {
			return domain.RoutingRow{}, false
		}
		return *fallback, true
	}
	return tableRow(workflow, e.WorkflowRow.Index())
}

// entryValueEquals reports whether the entry's recorded value reads with its
// row and equals want. An unreadable value equals nothing.
func entryValueEquals(
	workflow domain.AdmittedWorkflow,
	e domain.ExecutionLogEntry,
	fallback *domain.RoutingRow,
	want domain.IterationValue,
) bool {
	row, ok := entryRowForValue(workflow, e, fallback)
	if !ok {
		return false
	}
	v, err := domain.ReadIterationValue(e.Stage, row)
	return err == nil && v.Equal(want)
}

// entryHasValue reports whether the entry's recorded value reads with row and
// equals want.
func entryHasValue(e domain.ExecutionLogEntry, row domain.RoutingRow, want domain.IterationValue) bool {
	v, err := domain.ReadIterationValue(e.Stage, row)
	return err == nil && v.Equal(want)
}
