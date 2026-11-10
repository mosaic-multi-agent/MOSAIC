// Package stackconfig reads, validates and encodes the catalog stack persisted with a
// deployment. Exports are limited to roughly fifteen symbols; everything else stays unexported.
//
// Storage: the stack lives in the workspace manifest (<workspace>/.mosaic/manifest.yaml) under
// the top-level key catalog_stack, because only the manifest is saved inside the executor's
// atomic journal. It is written only by a deploy run's journalled manifest save, never by a
// separate write. A supplied stack replaces the persisted one as a whole.
//
// Sharing: the stack is per workspace. When the manifest is committed with the workspace, the
// stack travels with the checkout; the stored paths are absolute, so a checkout on another
// machine or at another location reports its user layers as unresolvable (a named validation
// error, never a silent drop) until a write run stores the stack again.
//
// Paths: every user layer path is stored absolute and cleaned; relative input is resolved
// against the working directory when encoded. The default layer is a path-less entry that is
// always last.
package stackconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/stackerr"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// Description is an app-level stack request: user layers highest first, plus the
// default-layer choice. A Description with no user layers and IncludeDefault false is empty
// (invalid).
type Description struct {
	UserLayers     []string // absolute, cleaned catalog roots; [0] is highest
	IncludeDefault bool     // default catalog as bottom layer
}

// Stored is a persisted stack as read from or written to the manifest.
//
// Structural invariants of a non-empty Layers (a Stored that holds them is "well-formed"):
//   - at most one entry has Default true, and when present it is the last entry
//   - a Default entry has empty Path and empty CatalogID
//   - every other entry (user layer) has a non-empty absolute Path and a non-empty CatalogID
//
// nil Layers and a non-nil empty Layers both mean "no persisted stack" and are well-formed.
type Stored struct {
	Layers []domain.PersistedCatalogLayer // highest first; nil or empty = no persisted stack
}

var (
	// ErrLayerUnresolvable is wrapped by every UnresolvableLayerError.
	ErrLayerUnresolvable = errors.New("persisted catalog layer no longer resolves")
	// ErrStoredStackMalformed is wrapped by Read and Validate when a persisted stack breaks a
	// Stored invariant.
	ErrStoredStackMalformed = errors.New("persisted catalog stack is malformed")
)

// UnresolvableLayerError names a persisted layer that no longer resolves.
type UnresolvableLayerError struct {
	Path       string // stored path ("" for the default layer)
	ExpectedID string
	Err        error // missing folder / identity error / "catalog id is now <x>"; always non-nil
}

func (e *UnresolvableLayerError) Error() string {
	return "persisted catalog layer " + e.Path + " (" + e.ExpectedID + ") no longer resolves: " + e.Err.Error()
}

// Unwrap returns exactly {ErrLayerUnresolvable, Err}.
func (e *UnresolvableLayerError) Unwrap() []error { return []error{ErrLayerUnresolvable, e.Err} }

// Description converts a well-formed stored stack into a request, in the same order.
func (s Stored) Description() Description {
	var d Description
	for _, l := range s.Layers {
		if l.Default {
			d.IncludeDefault = true
			continue
		}
		d.UserLayers = append(d.UserLayers, l.Path)
	}
	return d
}

// Read returns the persisted stack of workspace (the directory holding .mosaic/manifest.yaml).
// ok is false when the workspace has no manifest or the manifest carries no stack.
func Read(workspace string) (s Stored, ok bool, err error) {
	snap, err := manifest.NewStore().Load(workspace)
	if err != nil {
		return Stored{}, false, fmt.Errorf("read manifest of %s: %w", workspace, err)
	}
	switch snap.State {
	case manifest.StateAbsent, manifest.StateEmpty:
		return Stored{}, false, nil
	case manifest.StateCorrupt, manifest.StateFuture:
		return Stored{}, false, fmt.Errorf("manifest %s: %w", snap.Path, snap.Err)
	}
	if len(snap.Manifest.CatalogStack) == 0 {
		return Stored{}, false, nil
	}
	s = Stored{Layers: snap.Manifest.CatalogStack}
	if err := checkInvariants(s); err != nil {
		return Stored{}, false, fmt.Errorf("manifest %s: %w", snap.Path, err)
	}
	return s, true, nil
}

// Validate checks the Stored invariants, then that every user layer still resolves. The
// first failing layer, highest first, is reported as an *UnresolvableLayerError.
func Validate(s Stored, mosaicRoot string) error {
	if err := checkInvariants(s); err != nil {
		return err
	}
	for _, l := range s.Layers {
		if l.Default {
			continue
		}
		cause := resolveCause(l.Path, l.CatalogID)
		if cause != nil {
			return &UnresolvableLayerError{Path: l.Path, ExpectedID: l.CatalogID, Err: cause}
		}
	}
	return nil
}

// Encode turns d into the value the write runs hand to the executor. Each user layer must
// resolve now; its catalog id is read from its identity file.
func Encode(d Description) (Stored, error) {
	if len(d.UserLayers) == 0 && !d.IncludeDefault {
		return Stored{}, stackerr.ErrEmptyStack
	}
	var layers []domain.PersistedCatalogLayer
	for _, p := range d.UserLayers {
		abs, err := filepath.Abs(p)
		if err != nil {
			return Stored{}, &UnresolvableLayerError{Path: p, Err: err}
		}
		if _, err := os.Stat(abs); err != nil {
			return Stored{}, &UnresolvableLayerError{Path: abs, Err: err}
		}
		id, err := identity.Read(abs)
		if err != nil {
			return Stored{}, &UnresolvableLayerError{Path: abs, Err: err}
		}
		layers = append(layers, domain.PersistedCatalogLayer{Path: abs, CatalogID: id.ID})
	}
	if d.IncludeDefault {
		layers = append(layers, domain.PersistedCatalogLayer{Default: true})
	}
	return Stored{Layers: layers}, nil
}

// FindWorkspace returns path itself when it is a directory containing .mosaic/manifest.yaml,
// otherwise the nearest ancestor that does; ok false when none.
func FindWorkspace(path string) (workspace string, ok bool) {
	if path == "" {
		return "", false
	}
	dir, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, manifest.Dir, manifest.FileName)); err == nil && info.Mode().IsRegular() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// resolveCause returns why the layer at path no longer resolves to wantID, or nil.
func resolveCause(path, wantID string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	id, err := identity.Read(path)
	if err != nil {
		return err
	}
	if id.ID != wantID {
		return fmt.Errorf("catalog id is now %s", id.ID)
	}
	return nil
}

func checkInvariants(s Stored) error {
	for i, l := range s.Layers {
		last := i == len(s.Layers)-1
		switch {
		case l.Default && !last:
			return fmt.Errorf("%w: default layer must be last", ErrStoredStackMalformed)
		case l.Default && (l.Path != "" || l.CatalogID != ""):
			return fmt.Errorf("%w: default layer carries a path or catalog id", ErrStoredStackMalformed)
		case l.Default:
		case l.Path == "" || !filepath.IsAbs(l.Path):
			return fmt.Errorf("%w: user layer path %q is not absolute", ErrStoredStackMalformed, l.Path)
		case l.CatalogID == "":
			return fmt.Errorf("%w: user layer %s has no catalog id", ErrStoredStackMalformed, l.Path)
		}
	}
	return nil
}
