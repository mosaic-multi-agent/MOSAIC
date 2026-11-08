package activity_test

// Tests for LineObserver: decoded items reach the feed in order.

import (
	"bytes"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

type scriptedDecoder struct {
	status domain.ActivityStatus
	byLine map[string][]domain.ActivityItem
	seen   []string
}

func (d *scriptedDecoder) Status() domain.ActivityStatus { return d.status }

func (d *scriptedDecoder) Decode(line []byte) []domain.ActivityItem {
	d.seen = append(d.seen, string(bytes.Clone(line)))
	return d.byLine[string(line)]
}

type collectingFeed struct {
	items    []domain.ActivityItem
	statuses []domain.ActivityStatus
	ended    int
}

func (f *collectingFeed) Activity(item domain.ActivityItem) { f.items = append(f.items, item) }
func (f *collectingFeed) Status(st domain.ActivityStatus)   { f.statuses = append(f.statuses, st) }
func (f *collectingFeed) Ended()                            { f.ended++ }

func TestLineObserver_ReportsDecodedItemsToFeedInOrder(t *testing.T) {
	dec := &scriptedDecoder{byLine: map[string][]domain.ActivityItem{
		"l1": {{Kind: domain.ActivityText, Summary: "a"}, {Kind: domain.ActivityToolCall, Name: "Read"}},
		"l3": {{Kind: domain.ActivitySubagent, Name: "reviewer"}},
	}}
	feed := &collectingFeed{}
	observe := activity.LineObserver(dec, feed)

	observe([]byte("l1"))
	observe([]byte("l2")) // decodes to nothing
	observe([]byte("l3"))

	if len(dec.seen) != 3 {
		t.Errorf("want every line decoded, decoder saw %v", dec.seen)
	}
	want := []domain.ActivityKind{domain.ActivityText, domain.ActivityToolCall, domain.ActivitySubagent}
	if len(feed.items) != len(want) {
		t.Fatalf("want %d items, got %+v", len(want), feed.items)
	}
	for i, k := range want {
		if feed.items[i].Kind != k {
			t.Errorf("item %d: want kind %q, got %q", i, k, feed.items[i].Kind)
		}
	}
	if feed.ended != 0 {
		t.Errorf("LineObserver must not end the feed; Ended called %d times", feed.ended)
	}
}
