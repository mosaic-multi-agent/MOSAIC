package domain

import (
	"errors"
	"fmt"
	"sort"
)

// RateSet holds the five per-category rates of one tier.
type RateSet struct {
	Input        Rate
	CacheRead    Rate
	CacheWrite5m Rate
	CacheWrite1h Rate
	Output       Rate
}

// LongContextPricing is the optional single long-context tier.
type LongContextPricing struct {
	Threshold int64 // tokens per request (input + cache read + cache write); > 0
	Rates     RateSet
}

// ModelPricing is the entry for one model: a standard rate set plus an optional
// long-context tier.
type ModelPricing struct {
	Provider       ProviderID // "" = top-level models section
	Model          ModelID
	Standard       RateSet
	LongContext    LongContextPricing // meaningful only when HasLongContext
	HasLongContext bool
	Source         ConfigLevel // level the entry was loaded from
}

// HasLongContextTier reports whether tier selection applies to this model.
func (p ModelPricing) HasLongContextTier() bool { return p.HasLongContext }

// Validate reports why an entry is unusable. All five standard rates are
// required; a long-context block additionally needs a positive threshold and its
// own five rates. Errors name the model and the offending wire field.
func (p ModelPricing) Validate() error {
	if p.Model.IsEmpty() {
		return errors.New("pricing entry has empty model ID")
	}
	if field := p.Standard.firstMissing(""); field != "" {
		return fmt.Errorf("pricing entry for %s has zero %s rate", p.Model, field)
	}
	if !p.HasLongContext {
		return nil
	}
	if p.LongContext.Threshold <= 0 {
		return fmt.Errorf("pricing entry for %s has non-positive long_context.threshold", p.Model)
	}
	if field := p.LongContext.Rates.firstMissing("long_context."); field != "" {
		return fmt.Errorf("pricing entry for %s has zero %s rate", p.Model, field)
	}
	return nil
}

// firstMissing returns the wire name (with prefix) of the first zero rate, or ""
// when all five are set.
func (r RateSet) firstMissing(prefix string) string {
	fields := []struct {
		name string
		rate Rate
	}{
		{"input", r.Input},
		{"cache_read", r.CacheRead},
		{"cache_write_5m", r.CacheWrite5m},
		{"cache_write_1h", r.CacheWrite1h},
		{"output", r.Output},
	}
	for _, f := range fields {
		if f.rate == 0 {
			return prefix + f.name
		}
	}
	return ""
}

// PricingTable is an immutable lookup keyed by (provider, model). Lookup is strict:
// a provider never falls back to the provider-less entries, nor the reverse.
type PricingTable struct {
	entries  map[PricingKey]ModelPricing
	currency CurrencyCode // "" reads as DefaultCurrency
	excluded map[PricingKey]ExcludedPrice
}

// NewPricingTable builds a PricingTable from a slice of entries.
func NewPricingTable(entries []ModelPricing) PricingTable {
	m := make(map[PricingKey]ModelPricing, len(entries))
	for _, e := range entries {
		m[PricingKey{e.Provider, e.Model}] = e
	}
	return PricingTable{entries: m}
}

// EmptyPricingTable returns a table with no entries.
func EmptyPricingTable() PricingTable {
	return PricingTable{entries: make(map[PricingKey]ModelPricing)}
}

// Lookup resolves a model in the provider-less (top-level models) section.
func (t PricingTable) Lookup(m ModelID) (entry ModelPricing, found bool) {
	return t.LookupFor("", m)
}

// LookupFor resolves (provider, model) strictly. found=false is the explicit
// not-configured outcome and is never represented by a zero-valued ModelPricing.
func (t PricingTable) LookupFor(p ProviderID, m ModelID) (entry ModelPricing, found bool) {
	entry, found = t.entries[PricingKey{p, m}]
	return
}

// Models returns the distinct model IDs in sorted order.
func (t PricingTable) Models() []ModelID {
	if len(t.entries) == 0 {
		return nil
	}
	seen := make(map[ModelID]struct{}, len(t.entries))
	models := make([]ModelID, 0, len(t.entries))
	for k := range t.entries {
		if _, dup := seen[k.Model]; dup {
			continue
		}
		seen[k.Model] = struct{}{}
		models = append(models, k.Model)
	}
	sort.Slice(models, func(i, j int) bool {
		return string(models[i]) < string(models[j])
	})
	return models
}

// Len returns the number of entries in the table.
func (t PricingTable) Len() int { return len(t.entries) }

// With returns a copy of the table with the entry added or replaced. The table is
// never mutated in place, so a re-price cannot race a render.
func (t PricingTable) With(entry ModelPricing) PricingTable {
	m := make(map[PricingKey]ModelPricing, len(t.entries)+1)
	for k, e := range t.entries {
		m[k] = e
	}
	key := PricingKey{entry.Provider, entry.Model}
	m[key] = entry
	out := PricingTable{entries: m, currency: t.currency}
	if len(t.excluded) > 0 {
		out.excluded = make(map[PricingKey]ExcludedPrice, len(t.excluded))
		for k, e := range t.excluded {
			if k != key {
				out.excluded[k] = e
			}
		}
	}
	return out
}
