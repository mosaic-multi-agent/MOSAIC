package domain_test

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestNopActivityObserver_ReturnsUsableDiscardingFeed(t *testing.T) {
	var o domain.ActivityObserver = domain.NopActivityObserver{}

	feed := o.InvocationStarted(domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "a#1", Harness: "claude-code"})

	if feed == nil {
		t.Fatal("InvocationStarted must never return nil")
	}
	feed.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: "x"})
	feed.Ended()
	feed.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: "after end"})
}
