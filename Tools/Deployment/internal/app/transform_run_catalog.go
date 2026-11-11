package app

// transform_run_catalog.go finds the catalog identity (id, name, version) a transform copies
// from the source deployment into the entries it writes. The source entries come from the
// reconciled manifest and only entries of the source harness count, so an entry of another
// harness at the same path or under the same key is never mistaken for the source.

import (
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/domain"
)

// sourceEntries returns the reconciled manifest entries of the source harness.
func (r *transformRun) sourceEntries() []domain.ManifestEntry {
	return r.sourceManifest.Manifest.Entries
}

// workspaceRel returns the lower-cased workspace-relative slash form of a path, or "" when the
// path lies outside the workspace.
func (r *transformRun) workspaceRel(p string) string {
	if !filepath.IsAbs(p) {
		return strings.ToLower(filepath.ToSlash(filepath.Clean(p)))
	}
	rel, err := filepath.Rel(r.workspace, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return strings.ToLower(filepath.ToSlash(rel))
}

// sourceFileCatalog is the catalog identity of the source harness's entry of the given kind at
// the source file path; the zero identity when there is none.
func (r *transformRun) sourceFileCatalog(kind domain.ArtifactKind, sourcePath string) domain.CatalogIdentity {
	want := r.workspaceRel(sourcePath)
	if want == "" {
		return domain.CatalogIdentity{}
	}
	for _, e := range r.sourceEntries() {
		if e.Ref.Kind == kind && strings.ToLower(filepath.ToSlash(filepath.Clean(e.TargetPath))) == want {
			return e.Catalog
		}
	}
	return domain.CatalogIdentity{}
}

// sourceBundleCatalog is the catalog identity of the source harness's entries of the hook bundle
// with the given key; the first entry that carries an id wins, the zero identity when none does.
func (r *transformRun) sourceBundleCatalog(key string) domain.CatalogIdentity {
	for _, e := range r.sourceEntries() {
		if e.Ref.Kind == domain.ArtifactHook && e.Ref.Key == key && e.Catalog.ID != "" {
			return e.Catalog
		}
	}
	return domain.CatalogIdentity{}
}

// retiredHarness is the harness whose manifest records the run drops: the source harness of a
// move between the two GHCP targets, which share their files; empty for every other transform.
func (r *transformRun) retiredHarness() string {
	if isGhcpPair(r.srcID, r.tgtID) {
		return r.srcID
	}
	return ""
}
