package activity_test

// Tests for the status heartbeat of LineObserver and Begin: exactly one
// Status per stdout line, after that line's items, for dispatches and
// consultations alike, and shielded from a failing feed.

import (
	"fmt"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
	"mosaic-run/internal/testutil"
)

// countingDecoder yields one text item for lines starting with "item" and
// nothing otherwise; its status carries the number of lines seen.
type countingDecoder struct{ lines int }

func (d *countingDecoder) Decode(line []byte) []domain.ActivityItem {
	d.lines++
	if len(line) >= 4 && string(line[:4]) == "item" {
		return []domain.ActivityItem{{Kind: domain.ActivityText, Summary: string(line)}}
	}
	return nil
}

func (d *countingDecoder) Status() domain.ActivityStatus {
	return domain.ActivityStatus{ContextTokens: d.lines, HasContext: true}
}

// orderFeed records the call sequence as "item:<summary>" and "status:<n>".
type orderFeed struct{ calls []string }

func (f *orderFeed) Activity(it domain.ActivityItem) { f.calls = append(f.calls, "item:"+it.Summary) }
func (f *orderFeed) Status(st domain.ActivityStatus) {
	f.calls = append(f.calls, fmt.Sprintf("status:%d", st.ContextTokens))
}
func (f *orderFeed) Ended() {}

func TestLineObserver_CallsStatusOncePerLine_AfterThatLinesItems(t *testing.T) {
	feed := &orderFeed{}
	observe := activity.LineObserver(&countingDecoder{}, feed)

	observe([]byte("item-a"))
	observe([]byte("quiet line without items"))
	observe([]byte("{broken json"))
	observe([]byte("item-b"))

	want := []string{"item:item-a", "status:1", "status:2", "status:3", "item:item-b", "status:4"}
	if fmt.Sprint(feed.calls) != fmt.Sprint(want) {
		t.Errorf("want calls %v, got %v", want, feed.calls)
	}
}

func TestLineObserver_EmptyLine_StillCountsAsAHeartbeat(t *testing.T) {
	feed := &orderFeed{}
	observe := activity.LineObserver(&countingDecoder{}, feed)

	observe([]byte(""))

	if len(feed.calls) != 1 || feed.calls[0] != "status:1" {
		t.Errorf("want exactly one status for an empty line, got %v", feed.calls)
	}
}

func TestLineObserver_ClaudeTranscript_OneStatusPerLineEndingWithTheCost(t *testing.T) {
	lines := fixtureLines(t, "claude_status_transcript.jsonl")
	feed := &collectingFeed{}
	observe := activity.LineObserver(activity.NewClaudeCodeDecoder(), feed)

	for _, l := range lines {
		observe(l)
	}

	if len(feed.statuses) != len(lines) {
		t.Fatalf("want %d statuses (one per line), got %d", len(lines), len(feed.statuses))
	}
	last := feed.statuses[len(feed.statuses)-1]
	if !last.HasCost || !approxEqual(last.CostUSD, 0.25) || last.ContextTokens != 204 {
		t.Errorf("want the final status to carry cost 0.25 and context 204, got %+v", last)
	}
}

func TestBegin_StatusReachesTheFeed_ForDispatchAndConsultationInvocations(t *testing.T) {
	for _, src := range []domain.ActivitySource{domain.ActivitySourceDispatch, domain.ActivitySourceConsultation} {
		t.Run(string(src), func(t *testing.T) {
			obs := &testutil.Observer{}
			onLine, end := activity.Begin(obs, domain.ActivityInvocation{Source: src, Agent: "a#1", Harness: "claude-code"},
				activity.NewClaudeCodeDecoder())

			onLine([]byte(claudeInit))
			onLine([]byte(claudeAssistant("m1", "null", claudeTextBlock, claudeUsage(10, 100, 5))))
			end()

			got := obs.Statuses()
			if len(got) != 2 {
				t.Fatalf("want one status per line (2), got %+v", got)
			}
			want := domain.ActivityStatus{Model: "claude-sonnet-4-5", ContextTokens: 115, HasContext: true}
			if got[1] != want {
				t.Errorf("want %+v, got %+v", want, got[1])
			}
		})
	}
}

// statusPanickingObserver hands out a feed whose Status panics.
type statusPanickingObserver struct{ feed *statusPanickingFeed }

func (o *statusPanickingObserver) InvocationStarted(domain.ActivityInvocation) domain.ActivityFeed {
	o.feed = &statusPanickingFeed{}
	return o.feed
}

type statusPanickingFeed struct{ activity, status, ended int }

func (f *statusPanickingFeed) Activity(domain.ActivityItem) { f.activity++ }
func (f *statusPanickingFeed) Status(domain.ActivityStatus) {
	f.status++
	panic("status failure")
}
func (f *statusPanickingFeed) Ended() { f.ended++ }

func TestBegin_FeedThatPanicsInStatus_IsShieldedAndReceivesNothingFurther(t *testing.T) {
	obs := &statusPanickingObserver{}
	onLine, end := activity.Begin(obs, domain.ActivityInvocation{Source: domain.ActivitySourceDispatch}, &countingDecoder{})

	onLine([]byte("item-a")) // item is delivered, then Status panics
	onLine([]byte("item-b"))
	onLine([]byte("quiet"))
	end()

	f := obs.feed
	if f.activity != 1 || f.status != 1 {
		t.Errorf("after the panic the feed must receive nothing further: activity=%d status=%d", f.activity, f.status)
	}
}
