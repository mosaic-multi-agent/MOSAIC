package domain

// CacheFigures are the cache-efficiency figures of one scope (dispatch, agent
// type or run).
type CacheFigures struct {
	HitRatio          Ratio      // CacheRead / TotalContext() of the scope's tokens
	WriteTokens       TokenCount // CacheCreation tokens
	WriteCost         MoneyValue // CacheCreation money
	NetBenefit        MoneyValue // may be negative
	UnreadWriteTokens TokenCount // CacheCreation on the last per-call record of each invocation in scope
	UnreadWriteCost   MoneyValue
	Complete          bool // false when any contributing call's model was unpriced
}
