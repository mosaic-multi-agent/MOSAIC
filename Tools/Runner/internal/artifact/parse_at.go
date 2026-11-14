package artifact

import (
	"errors"

	"mosaic-run/internal/domain"
)

// ParseAt parses as Parse and sets Resource = path on the *domain.RefusalError
// or *domain.ArtifactDamageError it returns (other error types are returned
// unchanged).
func ParseAt(path string, data []byte) (domain.ArtifactState, error) {
	state, err := Parse(data)
	setResource(err, path)
	return state, err
}

// setResource sets the file path on a refusal or damage error in err's chain.
func setResource(err error, path string) {
	var re *domain.RefusalError
	if errors.As(err, &re) {
		re.Resource = path
	}
	var de *domain.ArtifactDamageError
	if errors.As(err, &de) {
		de.Resource = path
	}
}
