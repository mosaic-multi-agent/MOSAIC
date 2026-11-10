package injections

import (
	"os"
	"sort"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// Resolve picks, per harness directory, the highest layer that has it (layers[0] is the highest).
// A harness resolves whole from one layer; nothing is merged across layers.
func Resolve(layers []stackrec.LayerInput) Result {
	res := Result{byHarness: make(map[string]Resolution)}
	shadows := make(map[string]*stackrec.Shadow)
	for _, l := range layers {
		for _, name := range harnessDirs(l.Layer.Root) {
			if _, ok := res.byHarness[name]; !ok {
				res.byHarness[name] = Resolution{
					HarnessDir: name,
					Supplied:   true,
					Dir:        catalogpaths.HarnessInjectionsDir(l.Layer.Root, name),
					Supplier:   l.Layer,
				}
				item := domain.CatalogItem{Kind: domain.CatalogItemHarnessInjections, Key: name}
				shadows[name] = &stackrec.Shadow{Item: item, Winner: l.Layer, Declared: l.Overrides.Declares(item)}
			} else {
				shadows[name].Shadowed = append(shadows[name].Shadowed, l.Layer)
			}
		}
	}
	names := make([]string, 0, len(shadows))
	for name, s := range shadows {
		if len(s.Shadowed) > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		res.Shadows = append(res.Shadows, *shadows[name])
	}
	return res
}

// harnessDirs lists the names of the harness directories under a catalog root's injections
// directory. Non-directory entries are ignored.
func harnessDirs(root string) []string {
	entries, err := os.ReadDir(catalogpaths.HarnessInjectionsDir(root, ""))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}
