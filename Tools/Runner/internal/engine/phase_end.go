package engine

import (
	"mosaic-run/internal/domain"
)

// IsLastRowOfIteration reports whether the row at in.RowIndex, having returned
// SUCCESS, ends its iteration: its On Success entry ends the scope of the last
// group the iteration's approach lists (the iteration itself for an ungrouped
// block), so no group follows in that iteration.
//
// The answer follows the table's declared routing in every mode. It returns
// false for a row outside every block, for a block that has no schedule, and
// when the iteration's groups cannot be resolved.
func IsLastRowOfIteration(in BoundaryInput) bool {
	if in.Iteration == "" || in.RowIndex < 0 || in.RowIndex >= len(in.Workflow.Table.Rows) {
		return false
	}
	b, ok := in.Workflow.BlockOf(in.RowIndex)
	if !ok {
		return false
	}
	if _, found := in.Schedules[b.Name]; !found {
		return false
	}
	groups, err := orderedGroupsForIteration(in.Workflow, in.Schedules, b.Name, in.Iteration)
	if err != nil {
		return false
	}
	for i, g := range groups {
		if in.RowIndex < g.StartRow || in.RowIndex >= g.EndRow {
			continue
		}
		return i == len(groups)-1 && endsScope(in, g)
	}
	return false
}

// endsScope reports whether the declared On Success of the row at in.RowIndex
// ends the scope spanning group: COMPLETE, NEXT from the scope's last row, or a
// fork list (Mode 1) whose every target lies outside the scope.
func endsScope(in BoundaryInput, group domain.ExecutionGroup) bool {
	entry := in.Workflow.Table.Rows[in.RowIndex].Success
	switch entry.Kind {
	case domain.SuccessComplete:
		return true
	case domain.SuccessNext:
		return in.RowIndex+1 >= group.EndRow
	case domain.SuccessFork:
		if in.Mode != domain.ExecutionModeOrchestrated || len(entry.Fork) == 0 {
			return false
		}
		for _, t := range entry.Fork {
			if t.Index >= group.StartRow && t.Index < group.EndRow {
				return false
			}
		}
		return true
	}
	return false
}

// IsLastRowOfPhaseForIteration reports whether the row at in.RowIndex, having
// returned SUCCESS, is the last of its phase: the position routing moves to
// next is in another phase, or the run completes.
//
// The position is computed with the routing functions Next uses, over the
// Execution Log in in.Log (which holds the completed step): a block row ends its
// scope, its iteration is the last one remaining and the block exits to a row of
// another phase or completes the run. In Mode 1 the declared On Success is
// followed as if the run were engine-routed; a fork list leaves the phase only
// when every listed target does, and in any other mode a fork row is never a
// phase end. A block without a schedule has no routed position, so the answer
// is false.
func IsLastRowOfPhaseForIteration(in BoundaryInput) bool {
	rows := in.Workflow.Table.Rows
	if in.RowIndex < 0 || in.RowIndex >= len(rows) {
		return false
	}
	row := rows[in.RowIndex]
	r := newRouter(in.Workflow, in.Schedules, true).withScopes(in.Log)
	phase := row.PhaseParsed.Name

	if _, inBlock := in.Workflow.BlockOf(in.RowIndex); inBlock {
		adv, err := r.afterScopeRow(in.RowIndex, in.Iteration)
		return err == nil && advanceLeavesPhase(in.Workflow, adv, phase)
	}

	entry := row.Success
	switch entry.Kind {
	case domain.SuccessComplete:
		return true
	case domain.SuccessNext:
		adv, err := r.enterRow(in.RowIndex + 1)
		return err == nil && advanceLeavesPhase(in.Workflow, adv, phase)
	case domain.SuccessTarget:
		return targetLeavesPhase(in.Workflow, entry.Target, phase)
	case domain.SuccessBlockEntry:
		b, ok := in.Workflow.BlockByName(entry.Block)
		if !ok {
			return false
		}
		adv, err := r.enterBlock(b)
		return err == nil && advanceLeavesPhase(in.Workflow, adv, phase)
	case domain.SuccessFork:
		if in.Mode != domain.ExecutionModeOrchestrated || len(entry.Fork) == 0 {
			return false
		}
		for _, t := range entry.Fork {
			if !targetLeavesPhase(in.Workflow, t, phase) {
				return false
			}
		}
		return true
	}
	return false
}

// advanceLeavesPhase reports whether the computed position is the run's
// completion or a row of a phase other than phase. A block without a schedule
// has no position and does not leave.
func advanceLeavesPhase(workflow domain.AdmittedWorkflow, adv advance, phase string) bool {
	if adv.Complete {
		return true
	}
	if adv.NoSchedule != "" {
		return false
	}
	return targetLeavesPhase(workflow, domain.RowRef{Index: adv.RowIndex}, phase)
}

// targetLeavesPhase reports whether the row ref names is in a phase other than
// phase. A target outside the table does not leave.
func targetLeavesPhase(workflow domain.AdmittedWorkflow, ref domain.RowRef, phase string) bool {
	rows := workflow.Table.Rows
	if ref.Index < 0 || ref.Index >= len(rows) {
		return false
	}
	return rows[ref.Index].PhaseParsed.Name != phase
}
