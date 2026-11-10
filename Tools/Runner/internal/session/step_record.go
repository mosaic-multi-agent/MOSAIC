package session

import (
	"context"

	"mosaic-run/internal/domain"
)

// applyAndAnnounce records step in the Execution Log through the store and,
// when the store accepts it, announces the entry exactly as recorded to the
// step-record observer, so a frontend can show one finished row per entry.
// On a store error nothing is announced.
func (s *sessionImpl) applyAndAnnounce(ctx context.Context, state domain.ArtifactState, step domain.CompletedStep) (domain.ArtifactState, error) {
	newState, err := s.deps.Store.Apply(ctx, state, step)
	if err != nil {
		return newState, err
	}
	if n := len(newState.ExecutionLog); n > 0 {
		s.deps.StepRecords.StepRecorded(ctx, newState.ExecutionLog[n-1])
	}
	return newState, nil
}
