package session

// Resume refuses a run artifact in the pre-3.0 form before any dispatch: one
// with an Execution Log Stage column or a current_state.stage key, and one
// whose grouped row records a value in the pre-3.0 "{Group}.{ID}" form, which
// cannot be read with its row. Other readers of run artifacts keep their
// existing handling of an artifact they cannot read.

import (
	"errors"
	"fmt"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

// resumeLayoutRefusal returns the refusal of a resumed run whose artifact could
// not be read because it is in the pre-3.0 layout, or nil when readErr is not
// that error. The refusal names the run and carries the artifact error as cause.
func resumeLayoutRefusal(readErr error, runID string) *domain.RefusalError {
	if !errors.Is(readErr, artifact.ErrPre30Layout) {
		return nil
	}
	return &domain.RefusalError{
		Component: "session",
		Resource:  runID,
		Reason: fmt.Sprintf("run %s was written in the pre-3.0 run artifact layout (Stage column or "+
			"current_state.stage) and cannot be resumed", runID),
		Cause: readErr,
	}
}

// resumeValueRefusal checks every recorded Iteration value of a resumed run's
// Execution Log against the row it was recorded for. It returns a refusal
// naming the run for an entry whose grouped row records a value in no accepted
// form (the pre-3.0 "{Group}.{ID}"); otherwise nil. An entry recorded against a
// row outside the table is left to the engine's position handling.
func resumeValueRefusal(state domain.ArtifactState, table domain.RoutingTable, runID string) *domain.RefusalError {
	for _, entry := range state.ExecutionLog {
		if entry.WorkflowRow == domain.NoWorkflowRow {
			continue
		}
		row, ok := rowAtIndex(table, entry.WorkflowRow.Index())
		if !ok || row.PhaseParsed.Group == "" {
			continue
		}
		_, err := domain.ReadIterationValue(entry.Stage, row)
		var valueErr *domain.IterationValueError
		if !errors.As(err, &valueErr) || valueErr.Kind != domain.IterationValueMalformed {
			continue
		}
		return &domain.RefusalError{
			Component: "session",
			Resource:  runID,
			Reason: fmt.Sprintf("run %s cannot be resumed: its log entry %d records the iteration %q, "+
				"which workflow row %d does not accept (the pre-3.0 \"{Group}.{ID}\" form is no longer read)",
				runID, entry.Seq, entry.Stage, valueErr.Row),
			Cause: valueErr,
			Row:   valueErr.Row,
		}
	}
	return nil
}
