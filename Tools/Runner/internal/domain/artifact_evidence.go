package domain

import (
	"errors"
	"time"
)

// ArtifactEvidence gives access to the evidence copies the artifact store keeps
// outside the run folder. Implemented by the file store returned by
// artifact.NewFileStore.
type ArtifactEvidence interface {
	// LastGoodCopy returns the absolute path of the last-good copy and true when
	// one exists on disk; "" and false otherwise (none written yet, or the store
	// has no evidence location).
	LastGoodCopy() (path string, ok bool)

	// SaveDamagedCopy copies the artifact file, as it is on disk now, to a new
	// timestamped file beside the last-good copy and returns its absolute path.
	// The artifact itself is not touched. It never overwrites an earlier copy.
	// Returns ErrNoEvidenceLocation when the store has no evidence location, or
	// the I/O error.
	SaveDamagedCopy(now time.Time) (path string, err error)
}

// ErrNoEvidenceLocation: the store's artifact path is not run-scoped, so no
// evidence location exists.
var ErrNoEvidenceLocation = errors.New("artifact store has no evidence location")

// NopArtifactEvidence has no copies: LastGoodCopy returns "", false;
// SaveDamagedCopy returns "", ErrNoEvidenceLocation.
type NopArtifactEvidence struct{}

// LastGoodCopy reports no copy.
func (NopArtifactEvidence) LastGoodCopy() (string, bool) { return "", false }

// SaveDamagedCopy reports that there is no evidence location.
func (NopArtifactEvidence) SaveDamagedCopy(time.Time) (string, error) {
	return "", ErrNoEvidenceLocation
}
