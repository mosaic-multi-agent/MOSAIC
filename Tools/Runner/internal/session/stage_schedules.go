package session

// stageSchedules presents one schedule to the engine and the deviation package
// keyed by block: it holds the schedule under every block that has a staged
// row. The run itself holds one schedule per block, read from each block's
// declared Source (see block_sources.go).

import (
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// stagedRowBlock returns the block of workflow row rowIdx when the row is
// staged, and "" otherwise.
func stagedRowBlock(workflow domain.AdmittedWorkflow, rowIdx int) domain.BlockName {
	row := workflow.Table.Rows[rowIdx]
	if !row.PhaseParsed.IsStaged {
		return ""
	}
	if b, ok := workflow.BlockOf(rowIdx); ok {
		return b.Name
	}
	name, _ := row.Block()
	return name
}

// stageSchedules holds the schedule under every block that has a staged row,
// so that a row of any of them finds its block's schedule. A nil schedule, or a
// workflow with no staged row, gives nil.
func stageSchedules(workflow domain.AdmittedWorkflow, sched *domain.BlockSchedule) domain.BlockSchedules {
	if sched == nil {
		return nil
	}
	var out domain.BlockSchedules
	for i := range workflow.Table.Rows {
		block := stagedRowBlock(workflow, i)
		if _, done := out[block]; block == "" || done {
			continue
		}
		if out == nil {
			out = domain.BlockSchedules{}
		}
		held := *sched
		held.Block = block
		out[block] = held
	}
	return out
}

// resolveRowDefaults resolves the default payload of row at iteration with the
// schedules in force, keyed by the row's own block.
func resolveRowDefaults(
	row domain.RoutingRow,
	iteration domain.IterationID,
	stages domain.BlockSchedules,
) (domain.DispatchDefaults, error) {
	return engine.ResolveRowDefaultsForIteration(row, iteration, engine.ScheduleContext{Schedules: stages})
}
