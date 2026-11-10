package app

// manifest_provenance.go supplies the manifest's provenance data for the write paths: the
// source catalog of each stamped item and, for full runs, the catalogs that supplied the
// protocol and the deployed-sections bundle. A catalog that is not a stack reports none.

import (
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// withProvenance adds the source catalog to every stamp in stamps, in place, and returns stamps.
// Items are found through the plan's artifact refs; an item the catalog does not know, or a
// catalog that reports no provenance, leaves the stamp's catalog zero (unknown).
func (s *service) withProvenance(stamps map[string]domain.VersionStamp, items []domain.PlanItem) map[string]domain.VersionStamp {
	reporter, ok := s.deps.Catalog.(catalog.ProvenanceReporter)
	if !ok {
		return stamps
	}
	for _, item := range items {
		stamp, stamped := stamps[item.TargetPath]
		if !stamped {
			continue
		}
		var kind domain.CatalogItemKind
		switch item.Ref.Kind {
		case domain.ArtifactAgent:
			kind = domain.CatalogItemAgent
		case domain.ArtifactSkill:
			kind = domain.CatalogItemSkill
		case domain.ArtifactHook:
			kind = domain.CatalogItemHook
		default:
			continue
		}
		if layer, known := reporter.Provenance(domain.CatalogItem{Kind: kind, Key: item.Ref.Key}); known {
			stamp.Catalog = layer.Identity
			stamps[item.TargetPath] = stamp
		}
	}
	return stamps
}

// fullRunManifestMeta returns the protocol and bundle suppliers of the resolved stack, for runs
// that write the whole deployment (deploy and update). Partial runs supply none, so the manifest
// keeps the suppliers an earlier full run recorded. A supplier the stack does not name is nil.
func (s *service) fullRunManifestMeta() deploy.ManifestMeta {
	st, ok := s.deps.Catalog.(stack.Catalog)
	if !ok {
		return deploy.ManifestMeta{}
	}
	files := st.SystemFiles()
	var meta deploy.ManifestMeta
	if files.Protocol.Supplied {
		id := files.Protocol.Supplier.Identity
		meta.ProtocolSource = &id
	}
	if files.Bundle.Supplied {
		id := files.Bundle.Supplier.Identity
		meta.BundleSource = &id
	}
	return meta
}
