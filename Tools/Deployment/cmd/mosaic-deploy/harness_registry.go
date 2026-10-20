package main

// harness_registry.go supplies the harness registry for a rebuilt catalog stack.
//
// Rebuild mechanism: the registry built at launch remembers how it was discovered. When the TUI
// changes the stack, the built-in harness modules are constructed again from the new stack's
// per-harness injection directories (registry.Rebuilder), so the injection content and the
// orchestrator injections version follow the stack in effect. The old registry is left
// untouched. A registry that cannot be rebuilt is reused as it is.

import (
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/harness/registry"
)

// registryForStack returns base rebuilt for the stack's injections, or base itself when it
// cannot be rebuilt.
func registryForStack(base registry.Registry, cat stack.Catalog) (registry.Registry, error) {
	rb, ok := base.(registry.Rebuilder)
	if !ok {
		return base, nil
	}
	return rb.RebuildForStack(cat.HarnessInjections())
}
