package engine

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// afterScopeRow routes after the block row at rowIdx returned SUCCESS in
// iteration current, by its typed On Success entry within its scope: the group
// of a grouped block, the iteration of an ungrouped one.
//
//   - a target dispatches that row of the scope, above or below;
//   - NEXT dispatches the row below when it is in the scope, otherwise the scope
//     ends;
//   - COMPLETE ends the scope.
//
// Ending a group's scope starts the next group the iteration's Approach lists;
// after the last one the iteration has completed and the next iteration is
// chosen, or the block exits. An entry that routes nowhere in the scope is
// reported as a *domain.UnroutableEntryError.
func (r router) afterScopeRow(rowIdx int, current domain.IterationID) (advance, error) {
	b, ok := r.workflow.BlockOf(rowIdx)
	if !ok {
		return advance{}, fmt.Errorf("row %d belongs to no block of the workflow", rowIdx+1)
	}
	groups, err := orderedGroupsForIteration(r.workflow, r.schedules, b.Name, current)
	if err != nil {
		return advance{}, err
	}
	gi := -1
	for i, g := range groups {
		if rowIdx >= g.StartRow && rowIdx < g.EndRow {
			gi = i
			break
		}
	}
	if gi < 0 {
		return advance{}, &domain.RowNotInGroupError{
			WorkflowID: r.workflow.Table.Info.ID,
			RowIndex:   rowIdx,
			Block:      b.Name,
			Iteration:  current,
			Groups:     groups,
		}
	}
	group := groups[gi]
	entry := r.workflow.Table.Rows[rowIdx].Success
	unroutable := &domain.UnroutableEntryError{
		Row: domain.WorkflowRowFromIndex(rowIdx), Column: columnOnSuccess, Kind: string(entry.Kind),
	}

	switch entry.Kind {
	case domain.SuccessTarget:
		target := entry.Target.Index
		if target < group.StartRow || target >= group.EndRow {
			return advance{}, unroutable
		}
		return advance{RowIndex: target, Iteration: current}, nil
	case domain.SuccessNext:
		if rowIdx+1 < group.EndRow {
			return advance{RowIndex: rowIdx + 1, Iteration: current}, nil
		}
	case domain.SuccessComplete:
	default:
		return advance{}, unroutable
	}

	// The scope ended.
	if gi+1 < len(groups) {
		return advance{RowIndex: groups[gi+1].StartRow, Iteration: current}, nil
	}
	return r.startIteration(b, current)
}
