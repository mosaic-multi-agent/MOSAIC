package domain

import (
	"fmt"
	"time"
)

// RewriteThresholdPercent is the share of the previous request's context that a
// call must write again, and must not read from the cache, to count as a rewrite.
const RewriteThresholdPercent = 50

// RewriteKind says what the logs prove about a context rewrite.
type RewriteKind uint8

const (
	RewriteModelChanged RewriteKind = iota
	RewriteColdIdle
	RewritePartial
)

// Key returns the machine key of the kind: "model_changed", "cold_idle" or "partial".
func (k RewriteKind) Key() string {
	switch k {
	case RewriteModelChanged:
		return "model_changed"
	case RewriteColdIdle:
		return "cold_idle"
	}
	return "partial"
}

// ContextRewrite is one call that rewrote its context instead of reading it from
// the cache.
type ContextRewrite struct {
	Call          int // index into ActorContext.Calls
	Kind          RewriteKind
	Pause         time.Duration // this call's start minus the previous call's start
	PreviousModel ModelID       // RewriteModelChanged only
	Tokens        int64         // cache-write tokens of the call
	Cost          MoneyValue    // money of those cache writes
	ExtraCost     MoneyValue    // Cost minus the same tokens at the cache-read rate
}

// Label returns the text shown for the rewrite: "model changed",
// "cold after <N> min idle", "cold after <1 min idle" or "partial rewrite".
func (r ContextRewrite) Label() string {
	switch r.Kind {
	case RewriteModelChanged:
		return "model changed"
	case RewriteColdIdle:
		if minutes := int64(r.Pause / time.Minute); minutes >= 1 {
			return fmt.Sprintf("cold after %d min idle", minutes)
		}
		return "cold after <1 min idle"
	}
	return "partial rewrite"
}

// RewriteFigures are the rewrite totals of one scope.
type RewriteFigures struct {
	ModelChanged, ColdIdle, Partial int
	Tokens                          TokenCount // absent when no rewrite
	Cost, ExtraCost                 MoneyValue
	Complete                        bool // false when any rewrite's model was unpriced
}

// Count is the number of rewrites of every kind.
func (f RewriteFigures) Count() int {
	return f.ModelChanged + f.ColdIdle + f.Partial
}
