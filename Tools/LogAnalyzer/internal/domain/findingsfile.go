package domain

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// FindingsFileSuffix is the required extension of a findings export file.
const FindingsFileSuffix = ".json"

// IsFindingsFileName reports whether path's base name ends in ".json"
// (case-insensitive), is longer than the suffix, and is NOT a digest file name
// (IsDigestFileName false), so that a findings file is never mistaken for a digest.
func IsFindingsFileName(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return len(base) > len(FindingsFileSuffix) && strings.HasSuffix(base, FindingsFileSuffix) && !IsDigestFileName(path)
}

// ErrFindingsFileExists is returned (wrapped or not; test with errors.Is) when the
// target exists and overwrite is false.
var ErrFindingsFileExists = errors.New("findings file already exists")

// FindingsFileWriter writes a findings export document.
// Implementations: findingsfile.Writer (production), in-memory fake (tests).
type FindingsFileWriter interface {
	// WriteFindings writes the findings document for findings, in the given
	// order, to path. It creates missing parent directories. When path exists and
	// overwrite is false, it returns ErrFindingsFileExists and leaves the file
	// untouched. On any error path holds its previous content or nothing (no
	// partial file).
	WriteFindings(ctx context.Context, path string, findings []Finding, overwrite bool) error
}
