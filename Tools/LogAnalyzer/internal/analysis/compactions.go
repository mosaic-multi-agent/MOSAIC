package analysis

import "mosaic-log-analyzer/internal/domain"

// compactionStream identifies the log stream a compaction event came from;
// halves of a compaction only pair within one stream.
type compactionStream struct {
	kind domain.StreamKind
	hint domain.AgentInstanceID
	path string
}

// compactionFeedEntry is one compaction event in feed order.
type compactionFeedEntry struct {
	stream compactionStream
	actor  domain.ActorRef
	event  domain.CompactionEvent
}

func newCompactionFeedEntry(ref domain.StreamRef, actor domain.ActorRef, ev domain.Event) compactionFeedEntry {
	return compactionFeedEntry{
		stream: compactionStream{kind: ref.Kind, hint: ref.InstanceHint, path: ref.Path},
		actor:  actor,
		event: domain.CompactionEvent{
			SchemaVersion: ev.SchemaVersion,
			At:            ev.Timestamp,
			Fields:        *ev.Compaction,
		},
	}
}

// pairCompactionFeed pairs the feed's events per stream and returns one
// compaction per pairing result, in feed order of each compaction's first half.
func pairCompactionFeed(feed []compactionFeedEntry) []domain.Compaction {
	var order []compactionStream
	positions := make(map[compactionStream][]int)
	for i, e := range feed {
		if _, seen := positions[e.stream]; !seen {
			order = append(order, e.stream)
		}
		positions[e.stream] = append(positions[e.stream], i)
	}

	byFirst := make([]*domain.Compaction, len(feed))
	for _, key := range order {
		idx := positions[key]
		evs := make([]domain.CompactionEvent, len(idx))
		for k, i := range idx {
			evs[k] = feed[i].event
		}
		for _, pc := range domain.PairCompactionEvents(evs) {
			first := idx[pc.First]
			c := pc.Compaction
			c.Actor = feed[first].actor
			byFirst[first] = &c
		}
	}

	var out []domain.Compaction
	for _, c := range byFirst {
		if c != nil {
			out = append(out, *c)
		}
	}
	return out
}
