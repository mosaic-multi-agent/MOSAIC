package plan

// catalog_staleness.go decides catalog-change staleness: an item whose recorded catalog differs
// from the catalog supplying it now is stale.

import (
	"strings"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// CatalogDeltaField is the VersionDelta.Field for catalog-change staleness. It is distinct from
// every frontmatter stamp name, from WorkflowDeltaFieldPrefix, InfrastructureDeltaFieldPrefix,
// ProtocolDeltaField and BundleDeltaField, so the executor ignores it when stamping versions.
const CatalogDeltaField = "source_catalog"

// CatalogDrift records the catalog recorded for an item and the catalog supplying it now.
// The zero value means no drift.
type CatalogDrift struct {
	Recorded domain.CatalogIdentity // ManifestEntry.Catalog; zero = unknown (pre-stack manifest)
	Current  domain.CatalogIdentity // CurrentCatalog result; zero = unknown (non-stack or not found)
}

// CurrentCatalog returns the identity of the catalog layer that supplies ref in c. ok is false
// (and the identity zero) when c does not report provenance, when ref.Kind is not an agent,
// skill or hook, or when the catalog does not know the item.
func CurrentCatalog(c catalog.Catalog, ref domain.ArtifactRef) (domain.CatalogIdentity, bool) {
	reporter, ok := c.(catalog.ProvenanceReporter)
	if !ok {
		return domain.CatalogIdentity{}, false
	}
	var kind domain.CatalogItemKind
	switch ref.Kind {
	case domain.ArtifactAgent:
		kind = domain.CatalogItemAgent
	case domain.ArtifactSkill:
		kind = domain.CatalogItemSkill
	case domain.ArtifactHook:
		kind = domain.CatalogItemHook
	default:
		return domain.CatalogIdentity{}, false
	}
	layer, known := reporter.Provenance(domain.CatalogItem{Kind: kind, Key: ref.Key})
	if !known {
		return domain.CatalogIdentity{}, false
	}
	return layer.Identity, true
}

// supplierLookup answers which catalog supplies an artifact now (ok false = unknown).
type supplierLookup func(ref domain.ArtifactRef) (domain.CatalogIdentity, bool)

// supplierOf returns the supplier lookup of the catalog in effect.
func supplierOf(c catalog.Catalog) supplierLookup {
	return func(ref domain.ArtifactRef) (domain.CatalogIdentity, bool) { return CurrentCatalog(c, ref) }
}

// entryCatalogDrift compares the catalog recorded in entry with the current supplier of ref.
// An unknown supplier yields the zero drift.
func entryCatalogDrift(look supplierLookup, ref domain.ArtifactRef, entry domain.ManifestEntry) CatalogDrift {
	current, ok := look(ref)
	if !ok {
		return CatalogDrift{}
	}
	return CatalogStaleness(entry.Catalog, current)
}

// withCatalogDrift appends the catalog delta after deltas and the catalog reason after reasons,
// when the drift is stale.
func withCatalogDrift(d CatalogDrift, deltas []domain.VersionDelta, reasons string) ([]domain.VersionDelta, string) {
	delta, ok := d.Delta()
	if !ok {
		return deltas, reasons
	}
	return append(deltas, delta), strings.Join(filterEmpty(reasons, d.Reason()), "; ")
}

// CatalogStaleness compares the catalog recorded in an item's manifest entry with the catalog
// that supplies the item now. Either argument may be the zero identity (unknown).
func CatalogStaleness(recorded, current domain.CatalogIdentity) CatalogDrift {
	return CatalogDrift{Recorded: recorded, Current: current}
}

// Stale reports true only when both ids are non-empty and differ.
func (d CatalogDrift) Stale() bool {
	return d.Recorded.ID != "" && d.Current.ID != "" && d.Recorded.ID != d.Current.ID
}

// Delta returns the catalog delta (Deployed = recorded id, Source = current id); ok is false
// when not stale.
func (d CatalogDrift) Delta() (domain.VersionDelta, bool) {
	if !d.Stale() {
		return domain.VersionDelta{}, false
	}
	return domain.VersionDelta{Field: CatalogDeltaField, Deployed: d.Recorded.ID, Source: d.Current.ID}, true
}

// Reason returns one line naming both catalog ids, or "" when not stale.
func (d CatalogDrift) Reason() string {
	if !d.Stale() {
		return ""
	}
	return `source catalog changed from "` + d.Recorded.ID + `" to "` + d.Current.ID + `"`
}
