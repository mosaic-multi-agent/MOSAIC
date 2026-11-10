package artifact

import (
	"errors"

	"mosaic-run/internal/domain"
)

// writeRefusedError is returned by a write the store refuses because a read of
// the artifact found it damaged. It matches domain.ErrWriteAfterDamage and
// carries the damage.
type writeRefusedError struct {
	damage *domain.ArtifactDamageError
}

func (e *writeRefusedError) Error() string {
	return domain.ErrWriteAfterDamage.Error() + ": " + e.damage.Error()
}

func (e *writeRefusedError) Unwrap() []error {
	return []error{domain.ErrWriteAfterDamage, e.damage}
}

// noteRead records the outcome of a read in the write guard: damage sets it,
// a successful read clears it, any other result leaves it unchanged.
func (f *fileStore) noteRead(err error) {
	var damage *domain.ArtifactDamageError
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case err == nil:
		f.damage = nil
	case errors.As(err, &damage):
		f.damage = damage
	}
}

// guardedDamage returns the refusal for a write while the guard is set, or nil.
func (f *fileStore) guardedDamage() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.damage == nil {
		return nil
	}
	return &writeRefusedError{damage: f.damage}
}

// readForWrite is the re-read of a re-reading writer. A damaged re-read sets
// the guard and is returned as a refused write.
func (f *fileStore) readForWrite(data []byte) (domain.ArtifactState, error) {
	state, err := ParseAt(f.path, data)
	f.noteRead(err)
	var damage *domain.ArtifactDamageError
	if errors.As(err, &damage) {
		return domain.ArtifactState{}, &writeRefusedError{damage: damage}
	}
	return state, err
}

// writeArtifact writes data atomically and keeps the last-good copy current.
func (f *fileStore) writeArtifact(data []byte) error {
	if err := atomicWrite(f.path, data); err != nil {
		return err
	}
	f.refreshLastGood(data)
	return nil
}
