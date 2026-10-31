package artifact

import (
	"errors"

	"mosaic-run/internal/domain"
)

// ParseAt parses as Parse and sets Resource = path on every *domain.RefusalError
// it returns (other error types are returned unchanged).
func ParseAt(path string, data []byte) (domain.ArtifactState, error) {
	state, err := Parse(data)
	var re *domain.RefusalError
	if errors.As(err, &re) {
		re.Resource = path
	}
	return state, err
}
