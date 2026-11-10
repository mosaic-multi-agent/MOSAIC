package stackconfig

import "mosaic-deploy/internal/domain"

// Origin says where the stack in effect for a run came from.
type Origin string

const (
	// OriginStackInput: built from CLI stack flags (or changed in the TUI).
	OriginStackInput Origin = "stack-input"
	// OriginPersisted: reused from the workspace's persisted stack.
	OriginPersisted Origin = "persisted"
	// OriginDefault: no stack input and no persisted stack; the default layer alone.
	OriginDefault Origin = "default"
)

// ActiveStack is the stack in effect for a run.
type ActiveStack struct {
	Layers []domain.CatalogLayer `json:"layers"` // highest first
	Origin Origin                `json:"origin"`
}
