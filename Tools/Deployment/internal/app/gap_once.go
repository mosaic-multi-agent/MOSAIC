package app

import (
	"sync"

	"mosaic-deploy/internal/domain"
)

// gapOnce records gaps in the TODO output at most once per owner and kind for one run, however
// many times content is produced for the same item (the deployment-root probe and the write
// both invoke the content callback).
type gapOnce struct {
	mu   sync.Mutex
	seen map[string]bool
}

// newGapOnce returns an empty per-run de-duplicator.
func newGapOnce() *gapOnce {
	return &gapOnce{seen: make(map[string]bool)}
}

// first reports whether g is the first gap of its owner and kind seen in this run.
func (o *gapOnce) first(g domain.Gap) bool {
	key := g.Owner + "\x00" + string(g.Kind)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.seen[key] {
		return false
	}
	o.seen[key] = true
	return true
}
