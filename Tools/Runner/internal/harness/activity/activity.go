// Package activity turns harness stdout lines into domain activity items.
// Imported only by the three harness adapters.
package activity

import "mosaic-run/internal/domain"

// Decoder turns one harness stdout line into zero or more activity items.
// One Decoder per invocation; not safe for concurrent use. Unknown, partial
// or malformed lines yield nil, never an error or panic.
type Decoder interface {
	Decode(line []byte) []domain.ActivityItem
	// Status returns the cumulative ActivityStatus after every line passed to
	// Decode so far. The zero value before the first line.
	Status() domain.ActivityStatus
}

// LineObserver returns a callback for RunOptions/SpawnRequest.OnStdoutLine
// that decodes each line with dec and reports every item to feed.
func LineObserver(dec Decoder, feed domain.ActivityFeed) func(line []byte) {
	return func(line []byte) {
		for _, item := range dec.Decode(line) {
			feed.Activity(item)
		}
		feed.Status(dec.Status())
	}
}

// Begin announces one invocation to obs (nil selects the no-op observer) and
// returns the stdout line callback feeding it plus the function that ends the
// invocation. Observer failures never reach the caller: a panic inside the
// observer or its feed is swallowed, and a feed that panicked receives
// nothing further.
func Begin(obs domain.ActivityObserver, inv domain.ActivityInvocation, dec Decoder) (onLine func([]byte), end func()) {
	if obs == nil {
		obs = domain.NopActivityObserver{}
	}
	feed := startFeed(obs, inv)
	return LineObserver(dec, feed), feed.Ended
}

// safeFeed shields the caller from a misbehaving feed.
type safeFeed struct {
	inner  domain.ActivityFeed
	failed bool
}

func startFeed(obs domain.ActivityObserver, inv domain.ActivityInvocation) (feed *safeFeed) {
	feed = &safeFeed{}
	defer func() {
		if recover() != nil {
			feed.inner, feed.failed = nil, true
		}
	}()
	feed.inner = obs.InvocationStarted(inv)
	feed.failed = feed.inner == nil
	return feed
}

func (f *safeFeed) Activity(item domain.ActivityItem) {
	if f.failed {
		return
	}
	defer func() {
		if recover() != nil {
			f.failed = true
		}
	}()
	f.inner.Activity(item)
}

func (f *safeFeed) Status(st domain.ActivityStatus) {
	if f.failed {
		return
	}
	defer func() {
		if recover() != nil {
			f.failed = true
		}
	}()
	f.inner.Status(st)
}

func (f *safeFeed) Ended() {
	if f.inner == nil {
		return
	}
	defer func() { _ = recover() }()
	f.inner.Ended()
}
