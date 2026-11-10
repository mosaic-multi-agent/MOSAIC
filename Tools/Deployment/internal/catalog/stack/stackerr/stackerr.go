// Package stackerr holds the composition errors of a catalog stack: the sentinels that classify a
// failure and the error type that attributes a failure to one layer.
//
// Later composition rules (duplicate catalog id, orchestrator pair completeness, numeric-id
// collision) add their sentinels and error types here, so package stack itself exports no error.
package stackerr

import (
	"errors"
	"fmt"
	"strings"

	"mosaic-deploy/internal/domain"
)

var (
	// ErrEmptyStack is returned when a stack is composed from no layers.
	ErrEmptyStack = errors.New("catalog stack has no layers")
	// ErrInvalidLayer is returned when the layer list itself is malformed (empty or relative
	// root, misplaced or repeated default layer).
	ErrInvalidLayer = errors.New("catalog stack layer is invalid")
	// ErrLayerIdentity is returned when a layer's root carries no valid catalog identity.
	ErrLayerIdentity = errors.New("catalog layer has no valid identity")
	// ErrLayerLoad is returned when a layer's root is missing, is not a directory, or holds a
	// source file that cannot be interpreted.
	ErrLayerLoad = errors.New("catalog layer failed to load")
	// ErrDuplicateCatalogID is returned when two layers share a catalog id.
	ErrDuplicateCatalogID = errors.New("two catalog layers share a catalog id")
	// ErrOrchestratorPairIncomplete is returned when a layer supplies only one orchestrator file.
	ErrOrchestratorPairIncomplete = errors.New("catalog layer supplies only one orchestrator file")
	// ErrNumericIDCollision is returned when two different resolved agents share a numeric id.
	ErrNumericIDCollision = errors.New("numeric agent id used by two different agents")
)

// LayerError is a failure attributable to one layer.
type LayerError struct {
	Root      string // the layer's catalog root
	CatalogID string // empty when the identity could not be read
	Kind      error  // one of the sentinels above; never nil
	Err       error  // underlying cause; may be nil
}

// Error reads "catalog layer <id> (<root>): <Kind>[: <Err>]". A layer with no known id is
// named by its root only: "catalog layer <root>: <Kind>[: <Err>]".
func (e *LayerError) Error() string {
	name := e.Root
	if e.CatalogID != "" {
		name = fmt.Sprintf("%s (%s)", e.CatalogID, e.Root)
	}
	msg := fmt.Sprintf("catalog layer %s: %v", name, e.Kind)
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

// Unwrap returns Kind and, when present, Err, so errors.Is and errors.As reach both.
func (e *LayerError) Unwrap() []error {
	if e.Err == nil {
		return []error{e.Kind}
	}
	return []error{e.Kind, e.Err}
}

// DuplicateCatalogIDError names the shared catalog id and the colliding layer roots.
type DuplicateCatalogIDError struct {
	CatalogID string
	Roots     []string // the colliding roots, highest first
}

// Error names the id and every colliding root.
func (e *DuplicateCatalogIDError) Error() string {
	return fmt.Sprintf("%v: %s (%s)", ErrDuplicateCatalogID, e.CatalogID, strings.Join(e.Roots, ", "))
}

// Is reports true for ErrDuplicateCatalogID.
func (e *DuplicateCatalogIDError) Is(target error) bool { return target == ErrDuplicateCatalogID }

// NumericIDCollisionError names the numeric id, both agent keys and both catalogs.
type NumericIDCollisionError struct {
	NumericID string
	Keys      [2]string
	Catalogs  [2]domain.CatalogIdentity
}

// Error names the id, both keys and both catalog ids.
func (e *NumericIDCollisionError) Error() string {
	return fmt.Sprintf("%v: id %s held by %s (catalog %s) and %s (catalog %s)", ErrNumericIDCollision,
		e.NumericID, e.Keys[0], e.Catalogs[0].ID, e.Keys[1], e.Catalogs[1].ID)
}

// Is reports true for ErrNumericIDCollision.
func (e *NumericIDCollisionError) Is(target error) bool { return target == ErrNumericIDCollision }
