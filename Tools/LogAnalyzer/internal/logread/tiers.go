package logread

import (
	"bytes"
	"encoding/json"

	"mosaic-log-analyzer/internal/domain"
)

// tierSplitProblem describes why the cache-write tier fields of wu cannot be
// used as a split, or returns "" when they are valid or both absent. A valid
// split has both tier fields and the cache-write total, with the tiers summing
// to the total. Checks run in the order: total missing, one tier field missing,
// tiers do not sum.
func tierSplitProblem(wu wireUsage) string {
	if wu.CacheCreation5m == nil && wu.CacheCreation1h == nil {
		return ""
	}
	switch {
	case wu.CacheCreation == nil:
		return "cache_creation_tokens total missing while tier fields are present; split ignored"
	case wu.CacheCreation5m == nil || wu.CacheCreation1h == nil:
		return "one tier field missing (cache_creation_5m_tokens and cache_creation_1h_tokens must appear together); split ignored"
	case *wu.CacheCreation5m+*wu.CacheCreation1h != *wu.CacheCreation:
		return "cache_creation_5m_tokens and cache_creation_1h_tokens do not sum to cache_creation_tokens; split ignored"
	}
	return ""
}

// decodeTiers sets the tier fields on u only when the split is valid.
func decodeTiers(u *domain.TokenUsage, wu wireUsage) {
	if wu.CacheCreation5m == nil || wu.CacheCreation1h == nil || tierSplitProblem(wu) != "" {
		return
	}
	u.CacheCreation5m = domain.Tokens(*wu.CacheCreation5m)
	u.CacheCreation1h = domain.Tokens(*wu.CacheCreation1h)
}

// tierFinding returns an invalid_cache_tier_split finding for the raw
// token_usage object when its tier fields are inconsistent.
func tierFinding(raw *json.RawMessage, path string, line int) (domain.Finding, bool) {
	if raw == nil || !(bytes.Contains(*raw, []byte("cache_creation_5m_tokens")) ||
		bytes.Contains(*raw, []byte("cache_creation_1h_tokens"))) {
		return domain.Finding{}, false
	}
	var wu wireUsage
	if json.Unmarshal(*raw, &wu) != nil {
		return domain.Finding{}, false
	}
	problem := tierSplitProblem(wu)
	if problem == "" {
		return domain.Finding{}, false
	}
	return domain.Finding{
		Kind:     domain.FindingInvalidCacheTierSplit,
		Severity: domain.SeverityWarning,
		Path:     path,
		Line:     line,
		Detail:   problem,
	}, true
}
