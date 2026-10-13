package compat

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// runScope is the scope key of rows outside every block.
const runScope = -1

// rowScopes returns, per row, its scope key: runScope for a run-scope row, else
// the first row of the group (or ungrouped iteration) holding it.
func rowScopes(nRows int, blocks []domain.AdmittedBlock) []int {
	scopes := make([]int, nRows)
	for i := range scopes {
		scopes[i] = runScope
	}
	for _, b := range blocks {
		for _, g := range b.Groups {
			for i := g.StartRow; i < g.EndRow; i++ {
				scopes[i] = g.StartRow
			}
		}
	}
	return scopes
}

// checkScopes implements A11 and A12.
//
// A11: a [Name] entry in On Success names a declared block that is a child of
// the row's scope. Only the run has child blocks (nested blocks are refused
// earlier), so a block row naming any block is refused. An undeclared name
// never reaches this check: the shape check refuses it first.
//
// A12: every target in On Success, On Findings and Waits For names a row of the
// row's own scope.
func checkScopes(table domain.RoutingTable, blocks []domain.AdmittedBlock) error {
	scopes := rowScopes(len(table.Rows), blocks)
	for i, row := range table.Rows {
		at := domain.WorkflowRowFromIndex(i)
		if row.Success.Kind == domain.SuccessBlockEntry && scopes[i] != runScope {
			return refusal(table, "A11", at, string(row.Success.Block),
				fmt.Sprintf("row %d is a block row and enters block %q, which is not a child block of its scope", i+1, row.Success.Block))
		}
		var targets []domain.RowRef
		switch row.Success.Kind {
		case domain.SuccessTarget:
			targets = append(targets, row.Success.Target)
		case domain.SuccessFork:
			targets = append(targets, row.Success.Fork...)
		}
		if row.Findings.Kind == domain.FindingsTarget {
			targets = append(targets, row.Findings.Target)
		}
		targets = append(targets, row.WaitsFor...)
		for _, t := range targets {
			if scopes[t.Index] != scopes[i] {
				return refusal(table, "A12", at, t.Cell,
					fmt.Sprintf("row %d names %q (row %d), which is not a row of its own scope", i+1, t.Cell, t.Index+1))
			}
		}
	}
	return nil
}
