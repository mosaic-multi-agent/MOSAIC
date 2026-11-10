package main

// stack_construction.go builds the resolved catalog stack from the pre-scanned stack input.
//
// Flag decision: --catalog-folder is repeatable; every occurrence is one user layer and the
// first occurrence is the highest layer. --include-default-catalog (bool) adds the MOSAIC
// default catalog (<mosaicRoot>/Catalog) as the bottom layer wherever it appears among the
// arguments. A run with no stack flags uses the default catalog alone; a run that names a user
// catalog without --include-default-catalog leaves the default layer out;
// --include-default-catalog alone is the explicit way to request the default catalog alone.
//
// File split rule: this file holds only flag-to-stack construction. The TUI catalog reload,
// harness registry construction and persisted-stack reuse each go in their own new file in
// this package and call into this one, so that no file in cmd/mosaic-deploy crosses 500 lines.

import (
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/stackerr"
)

// buildCatalogStack builds the resolved catalog stack for the pre-scanned input. Stack errors
// are returned unchanged so that they name the offending layer.
func buildCatalogStack(mosaicRoot string, in stackInput) (stack.Catalog, error) {
	defaultLayer := stack.Layer{Root: catalog.DefaultCatalogRoot(mosaicRoot), Default: true}
	if !in.Given {
		return stack.New(mosaicRoot, []stack.Layer{defaultLayer})
	}
	layers := make([]stack.Layer, 0, len(in.Layers)+1)
	for _, root := range in.Layers {
		layers = append(layers, stack.Layer{Root: root})
	}
	if in.IncludeDefault {
		layers = append(layers, defaultLayer)
	}
	if len(layers) == 0 {
		return nil, stackerr.ErrEmptyStack
	}
	return stack.New(mosaicRoot, layers)
}

// stackDescription converts the pre-scanned input into the description the app layer persists
// when a write run was started from stack input.
func stackDescription(in stackInput) stackconfig.Description {
	if !in.Given {
		return stackconfig.Description{IncludeDefault: true}
	}
	return stackconfig.Description{UserLayers: in.Layers, IncludeDefault: in.IncludeDefault}
}
