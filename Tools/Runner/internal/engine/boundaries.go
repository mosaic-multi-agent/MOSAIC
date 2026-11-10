package engine

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// BoundaryInput bundles what the boundary answers need. The session builds it
// once per completed step.
type BoundaryInput struct {
	Workflow  domain.AdmittedWorkflow
	Schedules domain.BlockSchedules
	RowIndex  int                // zero-based row of the completed step
	Iteration domain.IterationID // "" outside every block

	// Log is the Execution Log including the completed step's entry; it is
	// read only to derive which iterations have completed.
	Log []domain.ExecutionLogEntry
	// Mode is the run mode. Only ExecutionModeOrchestrated selects the
	// declared-routing answer for a fork row; every other value, the zero
	// value included, gives the Modes 2 and 3 answer.
	Mode domain.ExecutionMode
}

// orderedGroupsForIteration returns the execution groups in the order they run
// for the given iteration of block, resolved from the workflow's approach table.
//
// Groups and approach rows are those of the block, so sibling blocks order
// their groups independently. When the block declares no groups, the approach is
// ignored entirely and the block's single implicit group is returned without
// error.
//
// Returns *domain.UnresolvableApproachError when the iteration's Approach value
// has no matching table row. It never falls back to the declared order, to a
// default approach, or to running every group.
func orderedGroupsForIteration(
	workflow domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	block domain.BlockName,
	id domain.IterationID,
) ([]domain.ExecutionGroup, error) {
	groups, grouped, approaches := blockGroupModel(workflow, block)

	// No groups declared: ignore the approach entirely.
	if !grouped {
		return groups, nil
	}

	// Grouped block: look up the approach for this iteration.
	sched, ok := schedules[block]
	if !ok {
		return nil, fmt.Errorf("stage %s: staged workflow requires a stage set but none is available", iterationText(id))
	}
	it, ok := sched.Find(id)
	if !ok {
		return nil, fmt.Errorf("stage %s has no entry in stage set", iterationText(id))
	}

	// Look up the approach in the block's approach rows.
	groupNames, found := approaches.Sequence(it.Approach)
	if !found {
		return nil, &domain.UnresolvableApproachError{
			WorkflowID: workflow.Table.Info.ID,
			Block:      block,
			Iteration:  id,
			Approach:   it.Approach,
			Declared:   approaches.Approaches(),
		}
	}

	// Map group names to the block's execution groups in sequence order.
	result := make([]domain.ExecutionGroup, 0, len(groupNames))
	for _, name := range groupNames {
		g, ok := groupByName(groups, name)
		if !ok {
			// This should be unreachable for an admitted workflow (guaranteed by A4),
			// but surface an error rather than silently skipping.
			return nil, fmt.Errorf("approach %q references group %q, which no row of block %s declares", it.Approach, name, block)
		}
		result = append(result, g)
	}
	return result, nil
}
