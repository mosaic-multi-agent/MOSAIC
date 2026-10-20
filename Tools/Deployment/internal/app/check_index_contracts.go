package app

// check_index_contracts.go declares the public contract for the workflow index staleness
// check: the result types returned to callers outside the app layer, and the method
// implementation on service.

import (
	"context"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

// IndexCheckResult reports workflow index staleness to a caller outside the app layer.
// It exists so internal/cli never needs to import internal/catalog. JSON tags follow
// the lowerCamelCase convention already used by PromoteResult.
type IndexCheckResult struct {
	// CatalogRoot is the absolute catalogue root inspected.
	CatalogRoot string `json:"catalogRoot"`
	// IndexPath is the absolute path of the index file inspected.
	IndexPath string `json:"indexPath"`
	// IndexPresent is false when no index file exists.
	IndexPresent bool `json:"indexPresent"`
	// FileOrphans lists files on disk missing from the index, sorted by Subject.
	FileOrphans []IndexCheckOrphan `json:"fileOrphans,omitempty"`
	// IndexOrphans lists index rows with no file on disk, sorted by Subject.
	IndexOrphans []IndexCheckOrphan `json:"indexOrphans,omitempty"`
	// DiskCount is the number of eligible workflow files on disk.
	DiskCount int `json:"diskCount"`
	// IndexCount is the number of index rows parsed.
	IndexCount int `json:"indexCount"`
	// Clean is true when no drift was found. An absent index is clean.
	Clean bool `json:"clean"`
	// Catalogs is the per-catalog view, highest layer first; never empty on success. The
	// top-level fields above are the aggregate over these entries.
	Catalogs []IndexCheckCatalog `json:"catalogs"`
}

// IndexCheckCatalog is one catalog's index check.
type IndexCheckCatalog struct {
	// Catalog is zero for a catalog that exposes no layer list.
	Catalog      domain.CatalogIdentity `json:"catalog"`
	CatalogRoot  string                 `json:"catalogRoot"`
	IndexPath    string                 `json:"indexPath"`
	IndexPresent bool                   `json:"indexPresent"`
	FileOrphans  []IndexCheckOrphan     `json:"fileOrphans,omitempty"`
	IndexOrphans []IndexCheckOrphan     `json:"indexOrphans,omitempty"`
	DiskCount    int                    `json:"diskCount"`
	IndexCount   int                    `json:"indexCount"`
	Clean        bool                   `json:"clean"`
}

// IndexCheckOrphan is one drift finding as reported outside the app layer.
type IndexCheckOrphan struct {
	// Code is "file-orphan" or "index-orphan".
	Code string `json:"code"`
	// Subject is the relative workflow path or the index row's workflow id.
	Subject string `json:"subject"`
	// Message is the human-readable explanation.
	Message string `json:"message"`
	// Path is the absolute path of the file involved.
	Path string `json:"path"`
}

// CheckWorkflowIndex implements Service.CheckWorkflowIndex. It reports the index check per
// catalog: when the loaded catalog is a stack (it implements stack.Catalog, which exposes the
// layer list) the result holds one entry per layer, highest first, and an error names the layer
// that failed. A catalog without a layer list (a plain catalog.Load result) yields exactly one
// entry for CatalogRoot() with no catalog identity, carrying the findings of the single-root
// check. The top-level fields aggregate the entries: counts are sums, CatalogRoot and IndexPath
// are the highest entry's, IndexPresent is true when any entry has an index, orphans are
// concatenated highest first, and Clean means
// every entry is clean. Results are mapped into app-owned types so internal/cli never imports
// internal/catalog.
func (s *service) CheckWorkflowIndex(_ context.Context) (IndexCheckResult, error) {
	var entries []IndexCheckCatalog
	if st, ok := s.deps.Catalog.(stack.Catalog); ok {
		reports, err := stack.CheckIndex(st)
		if err != nil {
			return IndexCheckResult{}, err
		}
		for _, r := range reports {
			entries = append(entries, indexCheckEntry(r.Layer.Identity, r.Report))
		}
	} else {
		report, err := catalog.CheckWorkflowIndex(s.deps.Catalog.CatalogRoot())
		if err != nil {
			return IndexCheckResult{}, err
		}
		entries = append(entries, indexCheckEntry(domain.CatalogIdentity{}, report))
	}
	return aggregateIndexEntries(entries), nil
}

func indexCheckEntry(id domain.CatalogIdentity, report catalog.WorkflowIndexReport) IndexCheckCatalog {
	return IndexCheckCatalog{
		Catalog:      id,
		CatalogRoot:  report.CatalogRoot,
		IndexPath:    report.IndexPath,
		IndexPresent: report.IndexPresent,
		FileOrphans:  mapOrphans(report.FileOrphans),
		IndexOrphans: mapOrphans(report.IndexOrphans),
		DiskCount:    report.DiskCount,
		IndexCount:   report.IndexCount,
		Clean:        report.Clean(),
	}
}

func mapOrphans(in []catalog.WorkflowIndexOrphan) []IndexCheckOrphan {
	var out []IndexCheckOrphan
	for _, o := range in {
		out = append(out, IndexCheckOrphan{Code: o.Code, Subject: o.Subject, Message: o.Message, Path: o.Path})
	}
	return out
}

func aggregateIndexEntries(entries []IndexCheckCatalog) IndexCheckResult {
	top := entries[0]
	result := IndexCheckResult{
		CatalogRoot: top.CatalogRoot,
		IndexPath:   top.IndexPath,
		Clean:       true,
		Catalogs:    entries,
	}
	for _, e := range entries {
		result.IndexPresent = result.IndexPresent || e.IndexPresent
		result.DiskCount += e.DiskCount
		result.IndexCount += e.IndexCount
		result.Clean = result.Clean && e.Clean
		result.FileOrphans = append(result.FileOrphans, e.FileOrphans...)
		result.IndexOrphans = append(result.IndexOrphans, e.IndexOrphans...)
	}
	return result
}
