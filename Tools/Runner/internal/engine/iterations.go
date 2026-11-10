package engine

import (
	"fmt"
	"strings"

	"mosaic-run/internal/domain"
)

// iterationCompleted reports whether iteration id of block has completed,
// derived from the Execution Log alone: the last group the iteration's Approach
// lists (the one scope of an ungrouped block) ended with SUCCESS. The most
// recent log entry of the iteration in that group decides, so a route-back
// inside the iteration or a re-dispatch of a row does not count as completion.
// An iteration whose approach cannot be resolved has not completed.
func iterationCompleted(
	workflow domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	block domain.BlockName,
	id domain.IterationID,
	log []domain.ExecutionLogEntry,
) bool {
	groups, err := orderedGroupsForIteration(workflow, schedules, block, id)
	if err != nil || len(groups) == 0 {
		return false
	}
	last := groups[len(groups)-1]
	for i := len(log) - 1; i >= 0; i-- {
		pos, posErr := resolvePosition(workflow, log[i])
		if posErr != nil || pos.rowIdx < last.StartRow || pos.rowIdx >= last.EndRow {
			continue
		}
		if !pos.iteration.Equal(id) {
			continue
		}
		return log[i].Status == domain.StatusSUCCESS && successEndsScope(workflow, pos.rowIdx, last)
	}
	return false
}

// successEndsScope reports whether the On Success entry of the row at rowIdx
// ends the scope spanning group: COMPLETE, or NEXT from the scope's last row.
func successEndsScope(workflow domain.AdmittedWorkflow, rowIdx int, group domain.ExecutionGroup) bool {
	switch workflow.Table.Rows[rowIdx].Success.Kind {
	case domain.SuccessComplete:
		return true
	case domain.SuccessNext:
		return rowIdx+1 >= group.EndRow
	}
	return false
}

// startIteration moves block b to the first iteration, in schedule order, that
// has not completed and whose Depends On iterations have all completed. current
// is the iteration that just ended and counts as completed, "" when entering the
// block. Without such an iteration the block exits.
//
// When uncompleted iterations remain but none is runnable (a Depends On cycle,
// or an ID the schedule does not list) the run stops: completing the block would
// skip work, and dispatching a blocked iteration would break the dependency.
func (r router) startIteration(b domain.AdmittedBlock, current domain.IterationID) (advance, error) {
	sched, found := r.schedules[b.Name]
	if !found {
		return advance{RowIndex: b.StartRow, NoSchedule: b.Name}, nil
	}
	done := make([]bool, len(sched.Iterations))
	for i, it := range sched.Iterations {
		done[i] = (current != "" && it.ID.Equal(current)) ||
			iterationCompleted(r.workflow, r.schedules, b.Name, it.ID, r.log)
	}
	var waiting []string
	for i, it := range sched.Iterations {
		if done[i] {
			continue
		}
		if !dependenciesCompleted(sched, it, done) {
			waiting = append(waiting, string(it.ID))
			continue
		}
		groups, err := orderedGroupsForIteration(r.workflow, r.schedules, b.Name, it.ID)
		if err != nil {
			return advance{}, err
		}
		if len(groups) == 0 {
			return advance{}, fmt.Errorf("no execution groups for iteration %s of block %s", iterationText(it.ID), b.Name)
		}
		return advance{RowIndex: groups[0].StartRow, Iteration: it.ID}, nil
	}
	if len(waiting) > 0 {
		return advance{}, fmt.Errorf(
			"block %s has no runnable iteration: %s have not completed and wait on each other or on an iteration the schedule does not list",
			b.Name, strings.Join(waiting, ", "))
	}
	return r.enterRow(b.EndRow)
}

// dependenciesCompleted reports whether every Depends On ID of it names an
// iteration of sched that done marks completed. IDs match ignoring case.
func dependenciesCompleted(sched domain.BlockSchedule, it domain.ScheduleIteration, done []bool) bool {
	for _, dep := range it.DependsOn {
		idx := iterationIndex(sched, dep)
		if idx < 0 || !done[idx] {
			return false
		}
	}
	return true
}
