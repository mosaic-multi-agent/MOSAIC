package engine

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// advance is a position the run moves to: a row, and the iteration it runs in.
type advance struct {
	RowIndex  int                // row to dispatch; undefined when Complete is true
	Iteration domain.IterationID // "" for a row outside every block
	Complete  bool               // no further rows to dispatch
	// NoSchedule names a block that was entered but has no usable schedule.
	// RowIndex is then the block's first row and Iteration is "".
	NoSchedule domain.BlockName
}

// router computes positions over the per-block admitted model and the
// per-block schedules. It is the one routing used by Next and by resume.
type router struct {
	workflow  domain.AdmittedWorkflow
	schedules domain.BlockSchedules
	// exitEmpty makes a block with an empty schedule exit at once. When it is
	// false an empty schedule counts like a missing one.
	exitEmpty bool
	// scoped makes the router follow WorkflowFormat 2.16 block semantics:
	// iterations in Depends On order, skipping the ones the log shows completed.
	// A router that leaves it false keeps the schedule's fixed sequence.
	scoped bool
	log    []domain.ExecutionLogEntry
}

// newRouter returns the router for workflow. Next routes with exitEmpty set.
func newRouter(workflow domain.AdmittedWorkflow, schedules domain.BlockSchedules, exitEmpty bool) router {
	return router{workflow: workflow, schedules: schedules, exitEmpty: exitEmpty}
}

// withScopes returns r routing blocks by scopes and iteration dependencies,
// with the completed iterations derived from log.
func (r router) withScopes(log []domain.ExecutionLogEntry) router {
	r.scoped = true
	r.log = log
	return r
}

// enterRow moves the run to the run-scope row at rowIdx: past the table the
// run is complete; a row that begins a block enters that block; any other row
// is dispatched as it is.
func (r router) enterRow(rowIdx int) (advance, error) {
	if rowIdx >= len(r.workflow.Table.Rows) {
		return advance{Complete: true}, nil
	}
	if b, ok := r.workflow.BlockOf(rowIdx); ok && b.StartRow == rowIdx {
		return r.enterBlock(b)
	}
	return advance{RowIndex: rowIdx}, nil
}

// enterBlock starts the first iteration of b in schedule order (a scoped
// router: the first one that has not completed and whose dependencies have) at
// the first row of the first group its Approach lists. An empty schedule exits the block
// at once; a block with no usable schedule is reported through NoSchedule.
func (r router) enterBlock(b domain.AdmittedBlock) (advance, error) {
	sched, found := r.schedules[b.Name]
	if !found || (len(sched.Iterations) == 0 && !r.exitEmpty) {
		return advance{RowIndex: b.StartRow, NoSchedule: b.Name}, nil
	}
	if len(sched.Iterations) == 0 {
		return r.enterRow(b.EndRow)
	}
	if r.scoped {
		return r.startIteration(b, "")
	}
	first := sched.Iterations[0].ID
	groups, err := orderedGroupsForIteration(r.workflow, r.schedules, b.Name, first)
	if err != nil {
		return advance{}, err
	}
	if len(groups) == 0 {
		return advance{}, fmt.Errorf("no execution groups for iteration %s of block %s", iterationText(first), b.Name)
	}
	return advance{RowIndex: groups[0].StartRow, Iteration: first}, nil
}

// groupOfRow returns the group holding rowIdx within the row's own block.
func groupOfRow(workflow domain.AdmittedWorkflow, rowIdx int) (domain.ExecutionGroup, bool) {
	if b, ok := workflow.BlockOf(rowIdx); ok {
		return b.GroupOf(rowIdx)
	}
	return domain.ExecutionGroup{}, false
}

// groupIndexOfRow returns the index of the row's group within its own block's
// groups (first-appearance order), or -1 when the row is in no group.
func groupIndexOfRow(workflow domain.AdmittedWorkflow, rowIdx int) int {
	b, ok := workflow.BlockOf(rowIdx)
	if !ok {
		return -1
	}
	for i, g := range b.Groups {
		if rowIdx >= g.StartRow && rowIdx < g.EndRow {
			return i
		}
	}
	return -1
}

// blockGroupModel returns the groups, the grouped flag and the approach rows
// of block. A name the workflow's per-block model does not hold has none.
func blockGroupModel(
	workflow domain.AdmittedWorkflow,
	block domain.BlockName,
) ([]domain.ExecutionGroup, bool, domain.ApproachTable) {
	if b, ok := workflow.BlockByName(block); ok {
		return b.Groups, b.Grouped, b.Approaches
	}
	return nil, false, domain.ApproachTable{}
}

// groupByName returns the group with the given name among groups.
func groupByName(groups []domain.ExecutionGroup, name domain.GroupName) (domain.ExecutionGroup, bool) {
	for _, g := range groups {
		if g.Name == name {
			return g, true
		}
	}
	return domain.ExecutionGroup{}, false
}
