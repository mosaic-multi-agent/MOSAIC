package systemfile

import (
	"os"
	"path/filepath"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// Resolve picks, per file, the highest layer whose root has the file (layers[0] is the highest).
// Each file resolves whole from one layer; nothing from a lower layer's copy is selected.
func Resolve(layers []stackrec.LayerInput) Result {
	var res Result
	var shadow *stackrec.Shadow
	res.Protocol, shadow = resolveFile(layers, catalogpaths.RelProtocolFile)
	if shadow != nil {
		res.Shadows = append(res.Shadows, *shadow)
	}
	res.Bundle, shadow = resolveFile(layers, catalogpaths.RelBundleFile)
	if shadow != nil {
		res.Shadows = append(res.Shadows, *shadow)
	}
	return res
}

// resolveFile resolves one named file and returns its shadow record when more than one layer has it.
func resolveFile(layers []stackrec.LayerInput, name string) (Resolution, *stackrec.Shadow) {
	res := Resolution{Name: name}
	var shadow *stackrec.Shadow
	item := domain.CatalogItem{Kind: domain.CatalogItemSystemFile, Key: name}
	for _, l := range layers {
		path := filepath.Join(l.Layer.Root, name)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		if !res.Supplied {
			res.Supplied, res.Path, res.Supplier = true, path, l.Layer
			shadow = &stackrec.Shadow{Item: item, Winner: l.Layer, Declared: l.Overrides.Declares(item)}
			continue
		}
		shadow.Shadowed = append(shadow.Shadowed, l.Layer)
	}
	if shadow != nil && len(shadow.Shadowed) == 0 {
		shadow = nil
	}
	return res, shadow
}
