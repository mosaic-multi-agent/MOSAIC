package settings

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"mosaic-log-analyzer/internal/domain"
)

// keyDefaultCacheWriteTier maps a harness to the tier that prices its cache
// writes carrying no 5m/1h split.
const keyDefaultCacheWriteTier = "default_cache_write_tier"

// applyCacheWriteTiers reads the default_cache_write_tier section: each valid
// harness entry replaces that harness's tier; an invalid entry is reported and
// leaves the harness at the tier set by a lower level or the built-in default.
func applyCacheWriteTiers(settings *domain.AnalysisSettings, value any, invalid func(string, ...any)) {
	if value == nil {
		return
	}
	entries, ok := value.(yaml.MapSlice)
	if !ok {
		invalid("%s must map harness names to a cache-write tier; defaults apply", keyDefaultCacheWriteTier)
		return
	}
	for _, entry := range entries {
		harness := domain.Harness(fmt.Sprint(entry.Key))
		if !harness.IsRecognised() {
			invalid("%s has an unrecognised harness %q; entry ignored", keyDefaultCacheWriteTier, string(harness))
			continue
		}
		tier, ok := entry.Value.(string)
		parsed, valid := domain.ParseCacheWriteTier(tier)
		if !ok || !valid {
			invalid("%s.%s must be one of %s, got %v; default %s applies",
				keyDefaultCacheWriteTier, harness, tierNames(), entry.Value, domain.DefaultCacheWriteTier)
			continue
		}
		if settings.CacheWriteTiers == nil {
			settings.CacheWriteTiers = map[domain.Harness]domain.CacheWriteTier{}
		}
		settings.CacheWriteTiers[harness] = parsed
	}
}

// tierNames lists the allowed tiers for a message.
func tierNames() string {
	var names []string
	for _, t := range domain.CacheWriteTiers() {
		names = append(names, string(t))
	}
	return strings.Join(names, ", ")
}
