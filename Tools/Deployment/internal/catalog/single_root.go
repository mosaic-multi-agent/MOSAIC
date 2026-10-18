package catalog

import (
	"errors"
	"fmt"
	"path/filepath"
)

// LoadSingleRoot reads only catalogRoot and never merges the MOSAIC default catalog: agents,
// workflows, skills and hooks all come from that one root (legacy-path scanning and first-wins
// within the root are kept). mosaicRoot is recorded as Root(). A load failure is returned as an
// error wrapping ErrCatalogSourceInvalid. An empty catalogRoot is not defaulted: it returns a nil
// Catalog and an error whose text contains "catalog root is empty".
func LoadSingleRoot(mosaicRoot, catalogRoot string) (Catalog, error) {
	if catalogRoot == "" {
		return nil, errors.New("catalog.LoadSingleRoot: catalog root is empty")
	}
	absCatalogRoot, err := filepath.Abs(catalogRoot)
	if err != nil {
		return nil, fmt.Errorf("catalog.LoadSingleRoot: catalogue root: %w", err)
	}
	return loadCatalog(mosaicRoot, absCatalogRoot)
}
