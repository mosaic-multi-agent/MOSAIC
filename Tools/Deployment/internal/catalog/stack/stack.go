// Package stack composes an ordered list of catalog layers into one resolved catalog.
//
// Each layer is a catalog root with an identity, loaded from its own root only. A higher layer
// (lower index) takes precedence: keyed content (agents of every role, workflows, skills, hooks)
// resolves per key to the highest layer that has it, and a skill or hook bundle is replaced whole.
// The orchestrator pair resolves as one unit from the highest layer that supplies either file.
//
// Export plan. This package stays within the ~15 exported symbols of docs/CODING-GUIDELINES.md
// and exports only what another package uses: the constructor New, the input type Layer and the
// companion interface Catalog, plus (in a later stage) the per-layer index-check entry point.
// Everything else is unexported. Composition errors live in the stackerr sub-package, record
// types that frontends need live in internal/domain (or a records sub-package), and any later
// concern with its own exported surface (system files, harness injections) is a sub-package that
// receives the ordered layers as plain values. Import direction: stack imports those
// sub-packages to hook their results into Catalog; a sub-package must never import stack.
package stack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/catalog/stack/stackerr"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/catalog/stack/systemfile"
	"mosaic-deploy/internal/domain"
)

// Layer is one requested stack layer (input to New).
type Layer struct {
	Root    string // absolute catalog root
	Default bool   // true for the MOSAIC default catalog layer (<mosaicRoot>/Catalog)
}

// Catalog is the resolved stack, the stack's companion interface: a catalog.Catalog that also
// exposes its ordered layers. The app layer reaches Layers by a type assertion on the catalog it
// holds (catalog.LayerLister), so frontends never import internal/catalog; a plain catalog.Load
// result does not implement it. Later stack concerns (provenance, shadow records, attributed
// issues, system files, harness injections) are added to this interface, never to a second one.
//
// Provenance (from catalog.ProvenanceReporter) answers which layer supplied an item; both
// orchestrator agent keys answer with the layer of the orchestrator pair. Shadows returns one
// record per item present in more than one layer (the orchestrator pair is one item), and none
// for unshadowed items. Issues() stamps each issue with the catalog (id, name, version) of the
// layer that raised it.
//
// The catalog.Catalog methods have these resolved meanings:
//   - Root() is the MOSAIC root.
//   - CatalogRoot() is the root of the highest layer (Layers()[0].Root). This is an interim
//     meaning; its consumers are reworked in later stages.
//   - Agent(key) looks up the key across all roles; the highest layer that has the key wins, so
//     a worker in a higher layer shadows a utility of the same key in a lower one.
//   - Workflows() is the union across layers with per-id shadowing; WorkflowCategories() groups
//     folders of the same name across layers (alphabetical; ids ascending).
//   - ReadSource serves only paths emitted by winning items.
//   - Tiers() and AgentByNumericID are computed over the resolved agents.
//   - Issues() is the plain concatenation of the layers' issues, highest layer first.
type Catalog interface {
	catalog.Catalog
	catalog.LayerLister
	catalog.ProvenanceReporter
	// Shadows returns the shadow records, ordered by item kind then key.
	Shadows() []stackrec.Shadow
	// SystemFiles returns where the protocol and the bundle resolve from, one whole file each.
	SystemFiles() systemfile.Result
	// HarnessInjections returns where each harness's injection directory resolves from, one
	// whole directory per harness.
	HarnessInjections() injections.Result
}

// New composes layers into one resolved catalog. layers[0] is the highest layer. Every layer is
// loaded with catalog.LoadSingleRoot from its own root (no merge with the MOSAIC default
// catalog) and must carry a valid identity. New returns exactly one error, from the first
// failing check, and a nil Catalog on error; a layer is never dropped silently.
func New(mosaicRoot string, layers []Layer) (Catalog, error) {
	if len(layers) == 0 {
		return nil, stackerr.ErrEmptyStack
	}
	if err := checkShape(mosaicRoot, layers); err != nil {
		return nil, err
	}
	loaded := make([]loadedLayer, len(layers))
	for i, l := range layers {
		ll, err := loadLayer(mosaicRoot, l)
		if err != nil {
			return nil, err
		}
		loaded[i] = ll
	}
	if err := checkDuplicateIDs(loaded); err != nil {
		return nil, err
	}
	if err := checkOrchestratorPairs(loaded); err != nil {
		return nil, err
	}
	r := resolve(loaded)
	if err := checkNumericIDs(r); err != nil {
		return nil, err
	}
	return r, nil
}

// checkShape validates the layer list without touching the disk.
func checkShape(mosaicRoot string, layers []Layer) error {
	defaultRoot := filepath.Clean(catalog.DefaultCatalogRoot(mosaicRoot))
	defaultSeen := false
	for i, l := range layers {
		switch {
		case l.Root == "":
			return invalidLayer(l.Root, "root is empty")
		case !filepath.IsAbs(l.Root):
			return invalidLayer(l.Root, "root is not an absolute path")
		case !l.Default:
			continue
		case defaultSeen:
			return invalidLayer(l.Root, "more than one default layer")
		case i != len(layers)-1:
			return invalidLayer(l.Root, "the default layer must be the last layer")
		case filepath.Clean(l.Root) != defaultRoot:
			return invalidLayer(l.Root, fmt.Sprintf("the default layer must be %s", defaultRoot))
		}
		defaultSeen = true
	}
	return nil
}

func invalidLayer(root, rule string) error {
	return &stackerr.LayerError{Root: root, Kind: stackerr.ErrInvalidLayer, Err: errors.New(rule)}
}

// loadedLayer is one layer after a successful single-root load.
type loadedLayer struct {
	info      domain.CatalogLayer
	cat       catalog.Catalog
	overrides identity.Overrides // items the layer's identity declares it overrides on purpose
}

// loadLayer checks the root, reads the identity and loads the layer from its own root only.
func loadLayer(mosaicRoot string, l Layer) (loadedLayer, error) {
	info, err := os.Stat(l.Root)
	if err != nil {
		return loadedLayer{}, &stackerr.LayerError{Root: l.Root, Kind: stackerr.ErrLayerLoad, Err: err}
	}
	if !info.IsDir() {
		return loadedLayer{}, &stackerr.LayerError{Root: l.Root, Kind: stackerr.ErrLayerLoad, Err: errors.New("not a directory")}
	}
	id, err := identity.Read(l.Root)
	if err != nil {
		return loadedLayer{}, &stackerr.LayerError{Root: l.Root, Kind: stackerr.ErrLayerIdentity, Err: err}
	}
	cat, err := catalog.LoadSingleRoot(mosaicRoot, l.Root)
	if err != nil {
		return loadedLayer{}, &stackerr.LayerError{Root: l.Root, CatalogID: id.ID, Kind: stackerr.ErrLayerLoad, Err: err}
	}
	return loadedLayer{
		info:      domain.CatalogLayer{Identity: domain.CatalogIdentity{ID: id.ID, Name: id.Name, Version: id.Version}, Root: l.Root, Default: l.Default},
		cat:       cat,
		overrides: id.Overrides,
	}, nil
}
