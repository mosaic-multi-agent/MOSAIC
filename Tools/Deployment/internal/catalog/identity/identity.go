// Package identity reads and validates a catalog's self-description from its identity file.
//
// Every catalog root carries <root>/catalog-identity.yaml (a small per-catalog YAML file that sits
// next to, and does not replace, the MOSAIC-root marker Catalog/CatalogFilesFormat.md). It names
// the catalog (id, display name, version) and may declare which items the catalog overrides on
// purpose. The file format is documented in Catalog/CatalogFilesFormat.md.
//
// This package imports internal/domain and catalogpaths only, never internal/catalog.
package identity

import (
	"errors"
	"sort"

	"mosaic-deploy/internal/domain"
)

// Identity is one catalog's self-description read from <root>/catalog-identity.yaml.
type Identity struct {
	ID        string    // non-empty; matches ^[a-z0-9][a-z0-9._-]*$
	Name      string    // non-empty display name
	Version   string    // non-empty; the scalar's source text verbatim
	Overrides Overrides // declared intentional overrides; zero value declares nothing
}

// Overrides is the set of items a catalog declares it overrides on purpose.
type Overrides struct {
	items map[domain.CatalogItem]struct{}
}

// NewOverrides returns an Overrides that declares exactly items. Duplicates collapse to one.
// It does not validate kinds or keys (validation belongs to Read); it exists so resolvers and
// their tests can build declared overrides without an identity file.
func NewOverrides(items ...domain.CatalogItem) Overrides {
	if len(items) == 0 {
		return Overrides{}
	}
	set := make(map[domain.CatalogItem]struct{}, len(items))
	for _, it := range items {
		set[it] = struct{}{}
	}
	return Overrides{items: set}
}

// Declares reports whether item was declared. The orchestrator pair is declared only as
// {orchestrator-pair, ""}; an Overrides returned by Read never contains the orchestrator agent
// keys, and one built with NewOverrides matches them only as themselves, never as the pair.
func (o Overrides) Declares(item domain.CatalogItem) bool {
	_, ok := o.items[item]
	return ok
}

// Items returns every declared item, sorted lexicographically by Kind, then by Key.
func (o Overrides) Items() []domain.CatalogItem {
	out := make([]domain.CatalogItem, 0, len(o.items))
	for it := range o.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Key < out[j].Key
	})
	return out
}

var (
	ErrIdentityMissing      = errors.New("catalog identity file missing or unreadable")
	ErrIdentityInvalid      = errors.New("catalog identity file could not be parsed or has an invalid value")
	ErrIdentityFieldMissing = errors.New("catalog identity is missing a required field")
	ErrOverrideInvalid      = errors.New("catalog identity declares an invalid override")
)
