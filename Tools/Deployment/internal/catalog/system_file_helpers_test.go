package catalog_test

import "path/filepath"

// catalogRootOf returns the default catalog root (<mosaicRoot>/Catalog) of a MOSAIC-root-shaped
// temp directory. The protocol and bundle loaders take a catalog root, so tests that build a
// MOSAIC-root-shaped fixture pass this directory to them.
func catalogRootOf(mosaicRoot string) string {
	return filepath.Join(mosaicRoot, "Catalog")
}
