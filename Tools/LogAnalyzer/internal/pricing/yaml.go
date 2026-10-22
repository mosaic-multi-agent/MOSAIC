package pricing

import (
	"fmt"
	"strings"

	"mosaic-log-analyzer/internal/domain"
)

// pricingFileYAML is the top-level wire type for the pricing YAML file.
//
// Wire schema:
//
//	models:
//	  <model-id>:
//	    input: "3.00"
//	    cache_read: "0.30"
//	    cache_write_5m: "3.75"
//	    cache_write_1h: "6.00"
//	    output: "15.00"
//	    long_context:            # optional, one tier per model
//	      threshold: 200000
//	      input: "6.00"
//	      cache_read: "0.60"
//	      cache_write_5m: "7.50"
//	      cache_write_1h: "12.00"
//	      output: "22.50"
//
// All rate values are decimal USD per 1,000,000 tokens stored as quoted strings
// so YAML float parsing cannot silently lose precision.
type pricingFileYAML struct {
	Models map[string]modelEntryYAML `yaml:"models"`
}

// RateSetYAML is the five-rate wire group shared by the standard and the
// long-context tier.
type RateSetYAML struct {
	Input        string `yaml:"input"`
	CacheRead    string `yaml:"cache_read"`
	CacheWrite5m string `yaml:"cache_write_5m"`
	CacheWrite1h string `yaml:"cache_write_1h"`
	Output       string `yaml:"output"`
}

// longContextYAML is the optional long-context block: a threshold plus its own
// rate set.
type longContextYAML struct {
	Threshold   int64 `yaml:"threshold"`
	RateSetYAML `yaml:",inline"`
}

// modelEntryYAML is the per-model wire type. The retired-schema keys are decoded
// only to recognise an old-format entry; they are never written for new entries.
type modelEntryYAML struct {
	RateSetYAML `yaml:",inline"`
	LongContext *longContextYAML `yaml:"long_context,omitempty"`

	OldCachedInput    any `yaml:"cached_input,omitempty"`
	OldCacheWrite     any `yaml:"cache_write,omitempty"`
	OldOutputUnder    any `yaml:"output_under_threshold,omitempty"`
	OldOutputAt       any `yaml:"output_at_threshold,omitempty"`
	OldContextLengthT any `yaml:"context_length_threshold,omitempty"`
}

// oldKeys returns the retired-schema keys present on the entry.
func (e modelEntryYAML) oldKeys() []string {
	candidates := []struct {
		name string
		val  any
	}{
		{"cached_input", e.OldCachedInput},
		{"cache_write", e.OldCacheWrite},
		{"output_under_threshold", e.OldOutputUnder},
		{"output_at_threshold", e.OldOutputAt},
		{"context_length_threshold", e.OldContextLengthT},
	}
	var found []string
	for _, c := range candidates {
		if c.val != nil {
			found = append(found, c.name)
		}
	}
	return found
}

// toDomain converts the wire entry to a domain.ModelPricing. It returns an error
// if the entry uses the old format or any rate string is unparseable so the
// caller can emit a per-entry finding and skip without aborting the whole load.
func (e modelEntryYAML) toDomain(modelID string) (domain.ModelPricing, error) {
	if old := e.oldKeys(); len(old) > 0 {
		return domain.ModelPricing{}, fmt.Errorf(
			"old pricing format (retired keys: %s); use input, cache_read, cache_write_5m, cache_write_1h, output and an optional long_context block",
			strings.Join(old, ", "))
	}
	standard, err := e.RateSetYAML.toDomain("")
	if err != nil {
		return domain.ModelPricing{}, err
	}
	p := domain.ModelPricing{Model: domain.ModelID(modelID), Standard: standard}
	if e.LongContext != nil {
		rates, err := e.LongContext.RateSetYAML.toDomain("long_context.")
		if err != nil {
			return domain.ModelPricing{}, err
		}
		p.HasLongContext = true
		p.LongContext = domain.LongContextPricing{Threshold: e.LongContext.Threshold, Rates: rates}
	}
	return p, nil
}

// toDomain parses the five rates; errors name the wire field with the prefix.
func (r RateSetYAML) toDomain(prefix string) (domain.RateSet, error) {
	var out domain.RateSet
	fields := []struct {
		name string
		raw  string
		dst  *domain.Rate
	}{
		{"input", r.Input, &out.Input},
		{"cache_read", r.CacheRead, &out.CacheRead},
		{"cache_write_5m", r.CacheWrite5m, &out.CacheWrite5m},
		{"cache_write_1h", r.CacheWrite1h, &out.CacheWrite1h},
		{"output", r.Output, &out.Output},
	}
	for _, f := range fields {
		rate, err := domain.ParseRate(f.raw)
		if err != nil {
			return domain.RateSet{}, fmt.Errorf("%s%s: %w", prefix, f.name, err)
		}
		*f.dst = rate
	}
	return out, nil
}

// fromDomain converts a domain.ModelPricing to the wire type for YAML serialisation.
// Rate values are written using Rate.String() so the round-trip through
// ParseRate is lossless in nanodollar terms.
func fromDomain(p domain.ModelPricing) modelEntryYAML {
	e := modelEntryYAML{RateSetYAML: rateSetToYAML(p.Standard)}
	if p.HasLongContext {
		e.LongContext = &longContextYAML{
			Threshold:   p.LongContext.Threshold,
			RateSetYAML: rateSetToYAML(p.LongContext.Rates),
		}
	}
	return e
}

func rateSetToYAML(r domain.RateSet) RateSetYAML {
	return RateSetYAML{
		Input:        r.Input.String(),
		CacheRead:    r.CacheRead.String(),
		CacheWrite5m: r.CacheWrite5m.String(),
		CacheWrite1h: r.CacheWrite1h.String(),
		Output:       r.Output.String(),
	}
}
