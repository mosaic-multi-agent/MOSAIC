package domain

import "strings"

// CacheWriteTier is a cache-write retention tier of the log format, named by the
// <tier> of its cache_creation_<tier>_tokens field.
type CacheWriteTier string

const (
	CacheWriteTier5m CacheWriteTier = "5m"
	CacheWriteTier1h CacheWriteTier = "1h"
	// DefaultCacheWriteTier prices cache writes that carry no reported split when
	// no tier is configured for the harness.
	DefaultCacheWriteTier = CacheWriteTier5m
)

// cacheWriteTiers is the one list of tiers the log format defines
// (cache_creation_<tier>_tokens); adding a tier here allows it in the setting.
var cacheWriteTiers = []CacheWriteTier{CacheWriteTier5m, CacheWriteTier1h}

// CacheWriteTiers returns the tiers the log format defines, in order.
func CacheWriteTiers() []CacheWriteTier {
	return append([]CacheWriteTier(nil), cacheWriteTiers...)
}

// ParseCacheWriteTier returns the tier named s; ok is false for any s that is
// not one of CacheWriteTiers().
func ParseCacheWriteTier(s string) (CacheWriteTier, bool) {
	for _, t := range cacheWriteTiers {
		if string(t) == s {
			return t, true
		}
	}
	return "", false
}

// AssumedTierLabel names the tiers behind assumed cache-write tokens, in the
// order given: "assumed 1h tier", "assumed 5m/1h tiers". No tiers names the
// default tier.
func AssumedTierLabel(tiers []CacheWriteTier) string {
	if len(tiers) == 0 {
		tiers = []CacheWriteTier{DefaultCacheWriteTier}
	}
	names := make([]string, len(tiers))
	for i, t := range tiers {
		names[i] = string(t)
	}
	noun := "tier"
	if len(tiers) > 1 {
		noun = "tiers"
	}
	return "assumed " + strings.Join(names, "/") + " " + noun
}

// MergeCacheWriteTiers returns the distinct tiers across lists in
// CacheWriteTiers() order; nil when there are none.
func MergeCacheWriteTiers(lists ...[]CacheWriteTier) []CacheWriteTier {
	present := map[CacheWriteTier]bool{}
	for _, l := range lists {
		for _, t := range l {
			present[t] = true
		}
	}
	var out []CacheWriteTier
	for _, t := range cacheWriteTiers {
		if present[t] {
			out = append(out, t)
		}
	}
	return out
}
