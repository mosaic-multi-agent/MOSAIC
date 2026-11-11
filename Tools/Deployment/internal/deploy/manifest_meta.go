package deploy

import (
	"sort"
	"time"

	"mosaic-deploy/internal/domain"
)

// ManifestMeta carries manifest-level data for one run. A nil field means "not supplied":
// the value is preserved from the prior manifest. A non-nil field replaces it.
type ManifestMeta struct {
	ProtocolSource *domain.CatalogIdentity
	BundleSource   *domain.CatalogIdentity
	// CatalogStack is the persisted stack: nil preserves the prior one, non-nil (even a single
	// default entry) replaces it whole.
	CatalogStack []domain.PersistedCatalogLayer
}

// applyManifestMeta sets the manifest-level data of m using the carry-over rule: each
// field is replaced when the request supplies a value (non-nil) and preserved from
// prior when it supplies none. A partial run (agents-only, hooks-only, workflows-only) that
// supplies no suppliers therefore keeps the ones recorded by an earlier full run. Fields that
// no run has ever supplied stay zero (unknown).
func applyManifestMeta(m *domain.Manifest, prior domain.Manifest, harnessID string, meta ManifestMeta) {
	prev, _ := prior.Section(harnessID)
	for _, s := range prior.Harnesses {
		if s.HarnessID != harnessID {
			m.Harnesses = append(m.Harnesses, s)
		}
	}
	m.Harnesses = append(m.Harnesses, domain.HarnessSection{
		HarnessID:      harnessID,
		UpdatedAt:      time.Now(),
		ProtocolSource: carryIdentity(prev.ProtocolSource, meta.ProtocolSource),
		BundleSource:   carryIdentity(prev.BundleSource, meta.BundleSource),
	})
	sort.Slice(m.Harnesses, func(i, j int) bool { return m.Harnesses[i].HarnessID < m.Harnesses[j].HarnessID })
	m.CatalogStack = prior.CatalogStack
	if meta.CatalogStack != nil {
		m.CatalogStack = meta.CatalogStack
	}
}

func carryIdentity(prior domain.CatalogIdentity, supplied *domain.CatalogIdentity) domain.CatalogIdentity {
	if supplied != nil {
		return *supplied
	}
	return prior
}
