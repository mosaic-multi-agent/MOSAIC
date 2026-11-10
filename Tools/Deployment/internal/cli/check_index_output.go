package cli

import (
	"fmt"
	"io"

	"mosaic-deploy/internal/app"
)

// renderCheckIndexHuman writes the index-check result to out, one section per catalog in layer
// order. Each section is headed "catalog <id> (<root>)", or "catalog unknown (<root>)" for a
// catalog without identity, and lists only that catalog's findings.
func renderCheckIndexHuman(out io.Writer, result app.IndexCheckResult) {
	for _, c := range catalogsOf(result) {
		id := c.Catalog.ID
		if id == "" {
			id = "unknown"
		}
		fmt.Fprintf(out, "catalog %s (%s)\n", id, c.CatalogRoot)
		renderCheckIndexCatalog(out, c)
	}
}

// catalogsOf returns the per-catalog entries of result. A result without entries (a service that
// fills only the top-level fields) is shown as one catalog without identity.
func catalogsOf(result app.IndexCheckResult) []app.IndexCheckCatalog {
	if len(result.Catalogs) > 0 {
		return result.Catalogs
	}
	return []app.IndexCheckCatalog{{
		CatalogRoot:  result.CatalogRoot,
		IndexPath:    result.IndexPath,
		IndexPresent: result.IndexPresent,
		FileOrphans:  result.FileOrphans,
		IndexOrphans: result.IndexOrphans,
		DiskCount:    result.DiskCount,
		IndexCount:   result.IndexCount,
		Clean:        result.Clean,
	}}
}

// renderCheckIndexCatalog writes the body of one catalog's section.
func renderCheckIndexCatalog(out io.Writer, c app.IndexCheckCatalog) {
	if !c.IndexPresent {
		fmt.Fprintf(out, "  no index: %s does not exist; nothing to compare\n", c.IndexPath)
		return
	}
	if c.Clean {
		fmt.Fprintf(out, "  clean: no drift between Index.md (%d entries) and disk (%d files)\n", c.IndexCount, c.DiskCount)
		return
	}
	if len(c.FileOrphans) > 0 {
		fmt.Fprintf(out, "  file-orphans: workflow files on disk not listed in Index.md:\n")
		for _, o := range c.FileOrphans {
			fmt.Fprintf(out, "    %s\n", o.Subject)
		}
	}
	if len(c.IndexOrphans) > 0 {
		fmt.Fprintf(out, "  index-orphans: index entries with no file on disk:\n")
		for _, o := range c.IndexOrphans {
			fmt.Fprintf(out, "    %s\n", o.Subject)
		}
	}
}
