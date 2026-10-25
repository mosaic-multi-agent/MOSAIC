package domain

import "time"

// TimelineEntry is one subagent invocation placed on the run timeline.
type TimelineEntry struct {
	Invocation  Invocation
	Duration    time.Duration
	HasDuration bool              // Invocation.Duration() is ok
	Overlaps    []AgentInstanceID // sorted; other entries whose interval intersects
}

// Timeline is the run timeline (V5), built from logged timestamps only.
type Timeline struct {
	// Entries are ordered by StartedAt ascending, then instance id; entries with
	// a zero StartedAt come last, ordered by instance id.
	Entries     []TimelineEntry
	Compactions []Compaction // == RunActivity.Compactions
}
