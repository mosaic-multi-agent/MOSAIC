// Package systemfile resolves the deploy tool's two system files (the communication protocol and
// the deployed-sections bundle) across the layers of a catalog stack. It checks file existence
// only and never parses a file. It must not import the stack package.
package systemfile

import (
	"errors"

	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// Resolution is the whole-file resolution of one system file.
type Resolution struct {
	Name     string // catalogpaths.RelProtocolFile or catalogpaths.RelBundleFile
	Supplied bool
	Path     string              // absolute path of the winning file; empty when !Supplied
	Supplier domain.CatalogLayer // zero when !Supplied
}

// Err returns nil when Supplied, otherwise a *NotSuppliedError for Name.
func (r Resolution) Err() error {
	if r.Supplied {
		return nil
	}
	return &NotSuppliedError{Name: r.Name}
}

// Result holds both system files and their shadow records.
type Result struct {
	Protocol Resolution
	Bundle   Resolution
	Shadows  []stackrec.Shadow // Kind system-file; at most one per file
}

// Resolve picks, per file, the highest layer whose root has the file.

// ErrNotSupplied is matched by every *NotSuppliedError.
var ErrNotSupplied = errors.New("no catalog layer supplies the system file")

// NotSuppliedError names the system file no layer supplies.
type NotSuppliedError struct{ Name string }

func (e *NotSuppliedError) Error() string { return "no catalog layer supplies " + e.Name }

func (e *NotSuppliedError) Is(target error) bool { return target == ErrNotSupplied }
