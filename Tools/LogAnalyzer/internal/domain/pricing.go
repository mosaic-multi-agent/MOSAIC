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
	Model          ModelID
	Standard       RateSet
	LongContext    LongContextPricing // meaningful only when HasLongContext
	HasLongContext bool
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

// PricingTable is an immutable, model-keyed lookup.
type PricingTable struct {
	entries map[ModelID]ModelPricing
}

// NewPricingTable builds a PricingTable from a slice of entries.
func NewPricingTable(entries []ModelPricing) PricingTable {
	m := make(map[ModelID]ModelPricing, len(entries))
	for _, e := range entries {
		m[e.Model] = e
	}
	return PricingTable{entries: m}
}

// EmptyPricingTable returns a table with no entries.
func EmptyPricingTable() PricingTable {
	return PricingTable{entries: make(map[ModelID]ModelPricing)}
}

// Lookup resolves a model. found=false is the explicit not-configured outcome and
// is never represented by a zero-valued ModelPricing.
func (t PricingTable) Lookup(m ModelID) (entry ModelPricing, found bool) {
	entry, found = t.entries[m]
	return
}

// Models returns all model IDs in sorted order.
func (t PricingTable) Models() []ModelID {
	if len(t.entries) == 0 {
		return nil
	}
	models := make([]ModelID, 0, len(t.entries))
	for id := range t.entries {
		models = append(models, id)
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
	m := make(map[ModelID]ModelPricing, len(t.entries)+1)
	for id, e := range t.entries {
		m[id] = e
	}
	m[entry.Model] = entry
	return PricingTable{entries: m}
}
