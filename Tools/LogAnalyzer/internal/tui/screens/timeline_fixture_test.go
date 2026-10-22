package screens_test

// Fixture for the timeline view. Agent types are unique per entry so a row can
// be found by its type: an instance id that is listed as an overlap of another
// entry appears in more than one row.

import (
	"fmt"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

var tlBase = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

func tlAt(sec int) time.Time { return tlBase.Add(time.Duration(sec) * time.Second) }

func tlInvocation(id, typ string, state domain.InvocationState, start, end time.Time) domain.Invocation {
	return domain.Invocation{
		Actor:     domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(id), Type: typ},
		State:     state,
		StartedAt: start,
		EndedAt:   end,
	}
}

func tlClosed(id, typ string, start, end time.Time, overlaps ...domain.AgentInstanceID) domain.TimelineEntry {
	e := domain.TimelineEntry{Invocation: tlInvocation(id, typ, domain.InvocationClosed, start, end), Overlaps: overlaps}
	if !start.IsZero() && !end.IsZero() {
		e.Duration, e.HasDuration = end.Sub(start), true
	}
	return e
}

// timelineRun has, in start order: solo#1 (planner, 10s, overlaps nothing),
// alpha#2 (coder, 30s) and beta#3 (tester, 45s) that overlap each other, rev#4
// (reviewer, no end), and delta#5 (closed, no usable timestamps, no type). Compactions: the orchestrator (auto) and
// ghost#9 (manual, time unknown). The base run carries money so tests can prove
// the timeline does not show it.
func timelineRun() domain.RunReport {
	run := overviewRun()
	run.Timeline = domain.Timeline{
		Entries: []domain.TimelineEntry{
			tlClosed("solo#1", "planner", tlAt(0), tlAt(10)),
			tlClosed("alpha#2", "coder", tlAt(20), tlAt(50), "beta#3"),
			tlClosed("beta#3", "tester", tlAt(30), tlAt(75), "alpha#2"),
			{Invocation: tlInvocation("rev#4", "reviewer", domain.InvocationOpen, tlAt(40), time.Time{})},
			tlClosed("delta#5", "", time.Time{}, time.Time{}),
		},
		Compactions: []domain.Compaction{
			{Actor: domain.Orchestrator(), At: tlAt(25), Trigger: "auto",
				TokensBefore: domain.Tokens(100000), TokensAfter: domain.Tokens(20000)},
			{Actor: domain.AgentInstance("ghost#9"), Trigger: "manual"},
		},
	}
	return run
}

// manyTimelineRun has n closed invocations step-01 ... in start order.
func manyTimelineRun(n int) domain.RunReport {
	run := overviewRun()
	for i := 1; i <= n; i++ {
		run.Timeline.Entries = append(run.Timeline.Entries,
			tlClosed(fmt.Sprintf("step-%02d", i), "coder", tlAt(i*10), tlAt(i*10+5)))
	}
	return run
}

// orderRun has three closed invocations whose given order is neither ascending
// nor descending by duration and differs from alphabetical order of the ids.
func orderRun() domain.RunReport {
	run := overviewRun()
	run.Timeline.Entries = []domain.TimelineEntry{
		tlClosed("mid-1", "coder", tlAt(0), tlAt(50)),
		tlClosed("long-2", "tester", tlAt(100), tlAt(200)),
		tlClosed("short-3", "planner", tlAt(300), tlAt(310)),
	}
	return run
}
