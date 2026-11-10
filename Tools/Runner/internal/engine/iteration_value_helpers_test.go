package engine_test

// Helpers for the tests of recorded Iteration values read with their row: rows
// built as literals for the log counters, and a converter that spells a run
// written as "{Group}.{ID}" in the 3.0 form "{ID}.{Group}".

import (
	"strings"

	"mosaic-run/internal/domain"
)

// ivOutsideRow is a routing row outside every block at the zero-based index.
func ivOutsideRow(index int) domain.RoutingRow {
	return domain.RoutingRow{Index: index, PhaseParsed: domain.PhaseParsed{Name: "PLANNING"}}
}

// ivGroupedRow is a routing row of group in block Stage at the zero-based index.
func ivGroupedRow(index int, group domain.GroupName) domain.RoutingRow {
	return domain.RoutingRow{Index: index, PhaseParsed: domain.PhaseParsed{
		Name: "EXECUTION", IsStaged: true, Group: group, Blocks: []domain.BlockName{"Stage"},
	}}
}

// ivUngroupedRow is a routing row of the ungrouped block Stage at the index.
func ivUngroupedRow(index int) domain.RoutingRow {
	return domain.RoutingRow{Index: index, PhaseParsed: domain.PhaseParsed{
		Name: "BUILD", IsStaged: true, Blocks: []domain.BlockName{"Stage"},
	}}
}

// ivEntry is an Execution Log entry of the row at the zero-based index with the
// given recorded value and status.
func ivEntry(index int, value string, status domain.StatusCode) domain.ExecutionLogEntry {
	return domain.ExecutionLogEntry{
		Agent: "agent#1", WorkflowRow: domain.WorkflowRowFromIndex(index), Stage: value, Status: status,
	}
}

// ivBlockedE501 is a BLOCKED/E501 entry of the row at index.
func ivBlockedE501(index int, value string) domain.ExecutionLogEntry {
	e := ivEntry(index, value, domain.StatusBLOCKED)
	e.ErrorCode = domain.ErrorTOOL_UNAVAILABLE
	return e
}

// ivKeyToThreeOh rewrites a "row:Group.ID" step key to "row:ID.Group". A key
// without a group part is returned unchanged.
func ivKeyToThreeOh(key string) string {
	row, value, ok := strings.Cut(key, ":")
	if !ok {
		return key
	}
	group, id, grouped := strings.Cut(value, ".")
	if !grouped {
		return key
	}
	return row + ":" + id + "." + group
}

// ivConvertCases returns the resume cases with every step key rewritten by conv.
func ivConvertCases(cases []resumeCase, conv func(string) string) []resumeCase {
	out := make([]resumeCase, 0, len(cases))
	for _, tc := range cases {
		steps := make([]string, 0, len(tc.steps))
		for _, k := range tc.steps {
			steps = append(steps, conv(k))
		}
		tc.steps = steps
		out = append(out, tc)
	}
	return out
}
