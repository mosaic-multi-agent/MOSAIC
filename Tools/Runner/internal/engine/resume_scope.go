package engine

import (
	"errors"
	"fmt"

	"mosaic-run/internal/domain"
)

// resumeAfterScopedRow is the resume point after a cleanly completed row of a
// WorkflowFormat 2.16 table: the position scope routing gives for the row's
// declared On Success, over the iterations the Execution Log shows completed.
// It is the position Next dispatches at in Modes 2 and 3; in Mode 1, where the
// consultant routes, it is the declared route's position.
//
// A fork list resumes at its first listed target. An entry that routes nowhere
// the engine can name falls back to the row below.
func resumeAfterScopedRow(
	workflow domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	log []domain.ExecutionLogEntry,
	pos position,
	seq int,
) (domain.ResumeInfo, error) {
	rowIdx := pos.rowIdx
	row := workflow.Table.Rows[rowIdx]
	r := newRouter(workflow, schedules, true).withScopes(log)

	var adv advance
	var err error
	switch _, inBlock := workflow.BlockOf(rowIdx); {
	case inBlock && row.Success.Kind == domain.SuccessFork:
		adv = advance{RowIndex: forkFirstTarget(row, rowIdx), Iteration: pos.iteration}
	case inBlock:
		block := blockOfRow(workflow, rowIdx)
		adv, err = r.afterScopeRow(rowIdx, scheduleSpelling(schedules, block, pos.iteration))
	default:
		adv, err = scopedRunScopeAdvance(r, workflow, row, rowIdx)
	}
	var unroutable *domain.UnroutableEntryError
	if errors.As(err, &unroutable) {
		return resumeAfterNonExecution(workflow, schedules, rowIdx, seq), nil
	}
	if err != nil {
		return domain.ResumeInfo{}, fmt.Errorf("resume: %w", err)
	}
	if adv.NoSchedule != "" {
		// The routed-to block has no usable schedule: the block's first row
		// stands, with no iteration and no group.
		next := workflow.Table.Rows[adv.RowIndex]
		return domain.ResumeInfo{
			RowIndex:   adv.RowIndex,
			Phase:      next.Phase,
			Block:      blockOfRow(workflow, adv.RowIndex),
			GroupIndex: -1,
			Seq:        seq,
		}, nil
	}
	return resumeInfoAt(workflow, adv, seq), nil
}

// scopedRunScopeAdvance is the position after a run-scope row by its typed On
// Success entry, as Next routes it.
func scopedRunScopeAdvance(r router, workflow domain.AdmittedWorkflow, row domain.RoutingRow, rowIdx int) (advance, error) {
	entry := row.Success
	switch entry.Kind {
	case domain.SuccessComplete:
		return advance{Complete: true}, nil
	case domain.SuccessNext:
		return r.enterRow(rowIdx + 1)
	case domain.SuccessTarget:
		if entry.Target.Index < 0 || entry.Target.Index >= len(workflow.Table.Rows) {
			return advance{}, &domain.UnroutableEntryError{
				Row: domain.WorkflowRowFromIndex(rowIdx), Column: columnOnSuccess, Kind: string(entry.Kind),
			}
		}
		return advance{RowIndex: entry.Target.Index}, nil
	case domain.SuccessBlockEntry:
		if b, ok := workflow.BlockByName(entry.Block); ok {
			return r.enterBlock(b)
		}
	case domain.SuccessFork:
		if len(entry.Fork) > 0 && entry.Fork[0].Index >= 0 && entry.Fork[0].Index < len(workflow.Table.Rows) {
			return advance{RowIndex: entry.Fork[0].Index}, nil
		}
	}
	return advance{}, &domain.UnroutableEntryError{
		Row: domain.WorkflowRowFromIndex(rowIdx), Column: columnOnSuccess, Kind: string(entry.Kind),
	}
}

// forkFirstTarget is the first target a fork row lists, or the row itself when
// it lists none.
func forkFirstTarget(row domain.RoutingRow, rowIdx int) int {
	if len(row.Success.Fork) == 0 {
		return rowIdx
	}
	return row.Success.Fork[0].Index
}
