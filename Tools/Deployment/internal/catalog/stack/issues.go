package stack

import "mosaic-deploy/internal/catalog"

// attributeIssues concatenates the layers' load issues, highest layer first, and stamps each
// with the identity of the layer that raised it. Issues of shadowed items are kept and stay with
// their own layer.
//
// This is groundwork kept to honour the catalog.Catalog Issues contract (a non-empty list is
// never silently ignored); nothing in the CLI, TUI or plan output displays these issues yet.
func attributeIssues(layers []loadedLayer) []catalog.Issue {
	var out []catalog.Issue
	for _, l := range layers {
		for _, is := range l.cat.Issues() {
			is.Catalog = l.info.Identity
			out = append(out, is)
		}
	}
	return out
}
