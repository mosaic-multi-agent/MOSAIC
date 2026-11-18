package domain

// CacheFigures are the cache-efficiency figures of one scope (dispatch, agent
// type or run).
type CacheFigures struct {
	HitRatio          Ratio      // capped cache reads / previous-call contexts over qualifying calls (see analysis cacheHitRatio)
	WriteTokens       TokenCount // CacheCreation tokens
	WriteCost         MoneyValue // CacheCreation money
	NetBenefit        MoneyValue // may be negative
	UnreadWriteTokens TokenCount // CacheCreation on the last per-call record of every session of each actor in scope
	UnreadWriteCost   MoneyValue
	Complete          bool // false when any contributing call's model was unpriced
	Rewrites          RewriteFigures
}
