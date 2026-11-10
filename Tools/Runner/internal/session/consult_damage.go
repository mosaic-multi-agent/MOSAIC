package session

import (
	"context"
	"errors"

	"mosaic-run/internal/domain"
)

// damagedConsultation is returned by consultPrimary when the artifact was found
// damaged right after the orchestrator's consultation ended. It stops the
// consultation's own reply, failure and any fallback or dialogue from being
// acted on: the damage is repaired first.
type damagedConsultation struct {
	damage *domain.ArtifactDamageError
	cause  error // the consultation's own error; nil when it replied
}

func (e *damagedConsultation) Error() string { return e.damage.Error() }

// Unwrap returns the consultation's own error.
func (e *damagedConsultation) Unwrap() error { return e.cause }

// damageAfterConsult reads the artifact strictly after a routing consultation
// ended with consultErr (nil when it replied). It returns the damage the
// consultation left, or nil when the artifact reads cleanly or fails to read
// for another reason; the latter is reported by the later strict re-read.
func (s *sessionImpl) damageAfterConsult(ctx context.Context, consultErr error) *damagedConsultation {
	_, readErr := s.deps.Store.Read(ctx)
	var damage *domain.ArtifactDamageError
	if errors.As(readErr, &damage) {
		return &damagedConsultation{damage: damage, cause: consultErr}
	}
	return nil
}
