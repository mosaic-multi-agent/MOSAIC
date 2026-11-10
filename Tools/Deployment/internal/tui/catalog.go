package tui

import (
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
)

// CatalogReloadFunc rebuilds an app.Service from a confirmed catalog stack. The implementation
// is supplied by the composition root: it builds the stack described by d with the same rules
// as CLI stack input, rebuilds app.Deps around it (reusing every other dependency), records
// origin as the origin of the rebuilt service's stack, and returns app.New(deps).
//
// It returns a non-nil error when the stack cannot be built. On error the caller keeps its
// existing service; a partially-built service is never returned alongside an error.
type CatalogReloadFunc func(d stackconfig.Description, origin stackconfig.Origin) (app.Service, error)
