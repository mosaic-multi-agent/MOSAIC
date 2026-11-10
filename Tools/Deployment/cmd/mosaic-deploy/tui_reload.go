package main

// tui_reload.go supplies the TUI catalogue reload: confirming a stack rebuilds the service from
// a catalog stack built by the same construction the CLI uses.

import (
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/tui"
)

// buildReloadFunc constructs the stack reload capability supplied to the TUI. The service is
// rebuilt from exactly the confirmed stack; stack errors are returned unchanged so the TUI can
// show them on the stack screen. All other dependencies are reused unchanged from baseDeps.
//
// The origin is recorded on the rebuilt service; only the stack-input origin is persisted by the
// write runs.
func buildReloadFunc(mosaicRoot string, baseDeps app.Deps) tui.CatalogReloadFunc {
	return func(d stackconfig.Description, origin stackconfig.Origin) (app.Service, error) {
		stackIn := stackInput{Layers: d.UserLayers, IncludeDefault: d.IncludeDefault, Given: true}
		newCat, err := buildCatalogStack(mosaicRoot, stackIn)
		if err != nil {
			return nil, err
		}
		newDeps := baseDeps
		newDeps.Catalog = newCat
		newDeps.StackDescription, newDeps.StackFromInput = stackDescription(stackIn), origin == stackconfig.OriginStackInput
		newDeps.StackOrigin = origin
		if newDeps.Registry, err = registryForStack(baseDeps.Registry, newCat); err != nil {
			return nil, err
		}
		return app.New(newDeps), nil
	}
}
