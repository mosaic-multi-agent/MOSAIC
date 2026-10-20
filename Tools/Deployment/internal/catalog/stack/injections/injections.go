// Package injections resolves built-in harness injection directories across the layers of a
// catalog stack, one unit per harness directory. It checks directory existence only and never
// parses a file. It must not import the stack package.
package injections

import (
	"errors"

	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// Resolution is the one-unit resolution of one harness's injections.
type Resolution struct {
	HarnessDir string // e.g. catalogpaths.HarnessDirNameClaudeCode
	Supplied   bool
	Dir        string // absolute <supplier root>/HarnessInjections/<HarnessDir>
	Supplier   domain.CatalogLayer
}

// Err returns nil when Supplied, otherwise a *NotSuppliedError for HarnessDir.
func (r Resolution) Err() error {
	if r.Supplied {
		return nil
	}
	return &NotSuppliedError{HarnessDir: r.HarnessDir}
}

// Result covers every harness directory present in any layer.
type Result struct {
	byHarness map[string]Resolution
	// Shadows has one record per shadowed harness directory (Kind harness-injections).
	Shadows []stackrec.Shadow
}

// Lookup returns the resolution for harnessDir.
// A harness no layer supplies yields an unsupplied Resolution whose Err names it.
func (r Result) Lookup(harnessDir string) Resolution {
	if res, ok := r.byHarness[harnessDir]; ok {
		return res
	}
	return Resolution{HarnessDir: harnessDir}
}

// ErrNotSupplied is matched by every *NotSuppliedError.
var ErrNotSupplied = errors.New("no catalog layer supplies the harness injections")

// NotSuppliedError names the harness directory no layer supplies.
type NotSuppliedError struct{ HarnessDir string }

func (e *NotSuppliedError) Error() string {
	return "no catalog layer supplies harness injections HarnessInjections/" + e.HarnessDir
}

// Is reports true for ErrNotSupplied.
func (e *NotSuppliedError) Is(target error) bool { return target == ErrNotSupplied }
