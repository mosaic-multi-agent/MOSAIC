package session

import (
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// triggerBoundaries bundles what the ITERATION_END and PHASE_END answers need: the
// admitted workflow and run schedule, the completed step's table row, and the
// run mode. The Execution Log is supplied when the answers are asked.
type triggerBoundaries struct {
	Workflow domain.AdmittedWorkflow
	Stages   domain.BlockSchedules
	RowIndex int
	Mode     domain.ExecutionMode
}

// row is the completed step's table row, which a recorded Iteration value is
// read with. A row index outside the table gives a row that accepts no
// iteration value.
func (b triggerBoundaries) row() domain.RoutingRow {
	if b.RowIndex >= 0 && b.RowIndex < len(b.Workflow.Table.Rows) {
		return b.Workflow.Table.Rows[b.RowIndex]
	}
	return domain.RoutingRow{Index: b.RowIndex}
}

// readValue reads the completed step's recorded value with its row, the same
// way position resolution reads it.
func (b triggerBoundaries) readValue(recorded string) (domain.IterationValue, error) {
	return domain.ReadIterationValue(recorded, b.row())
}

// input is the engine boundary input for the completed step at iteration. log
// is the Execution Log including that step.
func (b triggerBoundaries) input(iteration domain.IterationID, log []domain.ExecutionLogEntry) engine.BoundaryInput {
	return engine.BoundaryInput{
		Workflow:  b.Workflow,
		Schedules: b.Stages,
		RowIndex:  b.RowIndex,
		Iteration: iteration,
		Log:       log,
		Mode:      b.Mode,
	}
}

// endsIteration reports whether the completed step ends its iteration.
func (b triggerBoundaries) endsIteration(iteration domain.IterationID, log []domain.ExecutionLogEntry) bool {
	return engine.IsLastRowOfIteration(b.input(iteration, log))
}

// endsPhase reports whether the completed step is the last of its phase.
func (b triggerBoundaries) endsPhase(iteration domain.IterationID, log []domain.ExecutionLogEntry) bool {
	return engine.IsLastRowOfPhaseForIteration(b.input(iteration, log))
}
