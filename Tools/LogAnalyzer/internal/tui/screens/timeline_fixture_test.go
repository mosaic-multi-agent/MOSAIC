package screens_test

// Fixtures for the timeline view. Agent types are unique per entry so a row can
// be found by its type, and every figure of an entry (cost, peak and final
// context) differs from the other entries', so a figure found on screen can only
// have come from the entry the cursor is on.

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

// tlWithStatus sets the status code of an entry.
func tlWithStatus(e domain.TimelineEntry, status string) domain.TimelineEntry {
	e.Invocation.StatusCode = status
	e.Invocation.EndEvents = 1
	return e
}

func tlFigure(tokens int64) domain.ContextFigure {
	return domain.ContextFigure{Tokens: domain.Tokens(tokens), Band: domain.BandOf(tokens)}
}

// tlContext is the context figures of one invocation, with optional marks.
func tlContext(id, typ string, peak, final int64, marks ...domain.CompactionMark) domain.ActorContext {
	return domain.ActorContext{
		Actor:       domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(id), Type: typ},
		Peak:        tlFigure(peak),
		Final:       tlFigure(final),
		Compactions: marks,
	}
}

func tlLogged(sec int) domain.CompactionMark {
	return domain.CompactionMark{Kind: domain.CompactionLogged, At: tlAt(sec), Before: -1, After: -1}
}

func tlDetected(sec int) domain.CompactionMark {
	return domain.CompactionMark{Kind: domain.CompactionDetected, At: tlAt(sec), Before: 0, After: 1,
		BeforeTokens: 200_000, AfterTokens: 50_000}
}

// tlDispatch is the dispatch row of an invocation: its cost and peak context.
func tlDispatch(e domain.TimelineEntry, cents, peak int64) domain.Dispatch {
	d := dispatchOf(string(e.Invocation.Actor.Instance), e.Invocation.Actor.Type, e.Invocation.StatusCode, cents)
	d.Invocation = e.Invocation
	d.PeakContext = tlFigure(peak)
	return d
}

// timelineRun has, in start order (seconds from tlBase, axis 0-75):
//
//	solo#1   planner   0-10   SUCCESS  overlaps nothing         $3.17  peak 120,000 final  60,000
//	alpha#2  coder    20-50   BLOCKED  beta#3, rev#4             $2.28  peak 300,000 final  90,000
//	beta#3   tester   30-75   PARTIALLY_DONE  alpha#2, rev#4     $1.39  peak 380,000 final 210,000
//	rev#4    reviewer 40-open (runs to the axis end) alpha#2, beta#3  $0.54  peak 40,000 final 40,000
//	delta#5  (no type) closed, no status, no timestamps; overlaps nothing, no cost
//
// Logged compactions (sorted as the analysis delivers them): ghost#10 at 15 (no
// entry), orchestrator "autocompact" at 25, alpha#2 at 35, orchestrator "scheduled"
// paired 50-60, beta#3 "interrupted" pre-only at 65, orchestrator "lateness"
// post-only at 72, and ghost#9 "manual" with no time. Detected marks: the
// orchestrator at 10 and beta#3 at 45.
func timelineRun() domain.RunReport {
	run := overviewRun()
	solo := tlWithStatus(tlClosed("solo#1", "planner", tlAt(0), tlAt(10)), "SUCCESS")
	alpha := tlWithStatus(tlClosed("alpha#2", "coder", tlAt(20), tlAt(50), "beta#3", "rev#4"), "BLOCKED")
	beta := tlWithStatus(tlClosed("beta#3", "tester", tlAt(30), tlAt(75), "alpha#2", "rev#4"), "PARTIALLY_DONE")
	rev := domain.TimelineEntry{
		Invocation: tlInvocation("rev#4", "reviewer", domain.InvocationOpen, tlAt(40), time.Time{}),
		Overlaps:   []domain.AgentInstanceID{"alpha#2", "beta#3"},
	}
	delta := tlClosed("delta#5", "", time.Time{}, time.Time{})
	run.Timeline = domain.Timeline{
		Entries: []domain.TimelineEntry{solo, alpha, beta, rev, delta},
		Axis:    domain.TimeInterval{Start: tlAt(0), End: tlAt(75)},
		HasAxis: true,
		Compactions: []domain.Compaction{
			{Actor: domain.AgentInstance("ghost#10"), At: tlAt(15), Trigger: "stray"},
			{Actor: domain.Orchestrator(), At: tlAt(25), Trigger: "autocompact",
				TokensBefore: domain.Tokens(100000), TokensAfter: domain.Tokens(20000)},
			{Actor: domain.AgentInstance("alpha#2"), At: tlAt(35), Trigger: "overflow",
				TokensBefore: domain.Tokens(80000), TokensAfter: domain.Tokens(15000)},
			{Actor: domain.Orchestrator(), At: tlAt(50), EndedAt: tlAt(60), Trigger: "scheduled",
				TokensBefore: domain.Tokens(150000), TokensAfter: domain.Tokens(30000), Shape: domain.CompactionPaired},
			{Actor: domain.AgentInstance("beta#3"), At: tlAt(65), Trigger: "interrupted",
				TokensBefore: domain.Tokens(70000), Shape: domain.CompactionPreOnly},
			{Actor: domain.Orchestrator(), At: tlAt(72), Trigger: "lateness",
				TokensAfter: domain.Tokens(25000), Shape: domain.CompactionPostOnly},
			{Actor: domain.AgentInstance("ghost#9"), Trigger: "manual"},
		},
	}
	run.ByAgentType = []domain.AgentTypeGroup{
		{AgentType: "planner", Dispatches: []domain.Dispatch{tlDispatch(solo, 317, 120_000)}},
		{AgentType: "coder", Dispatches: []domain.Dispatch{tlDispatch(alpha, 228, 300_000)}},
		{AgentType: "tester", Dispatches: []domain.Dispatch{tlDispatch(beta, 139, 380_000)}},
		{AgentType: "reviewer", Dispatches: []domain.Dispatch{tlDispatch(rev, 54, 40_000)}},
	}
	run.Context = domain.RunContext{
		Orchestrator: domain.ActorContext{
			Actor:       domain.Orchestrator(),
			Compactions: []domain.CompactionMark{tlDetected(10), tlLogged(25), tlLogged(50), tlLogged(72)},
		},
		Invocations: []domain.ActorContext{
			tlContext("alpha#2", "coder", 300_000, 90_000, tlLogged(35)),
			tlContext("beta#3", "tester", 380_000, 210_000, tlDetected(45), tlLogged(65)),
			tlContext("rev#4", "reviewer", 40_000, 40_000),
			tlContext("solo#1", "planner", 120_000, 60_000),
		},
	}
	return run
}

func tlSpan(from, to int) domain.TimeInterval {
	return domain.TimeInterval{Start: tlAt(from), End: tlAt(to)}
}

// manyTimelineRun has n closed invocations step-01 ... in start order, each with
// its own type kind-01 ..., so the invocation's strip lines are told from its lane.
func manyTimelineRun(n int) domain.RunReport {
	run := overviewRun()
	for i := 1; i <= n; i++ {
		run.Timeline.Entries = append(run.Timeline.Entries, tlWithStatus(
			tlClosed(fmt.Sprintf("step-%02d", i), fmt.Sprintf("kind-%02d", i), tlAt(i*10), tlAt(i*10+5)), "SUCCESS"))
	}
	run.Timeline.Axis = domain.TimeInterval{Start: tlAt(10), End: tlAt(n*10 + 5)}
	run.Timeline.HasAxis = n > 0
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
	run.Timeline.Axis = domain.TimeInterval{Start: tlAt(0), End: tlAt(310)}
	run.Timeline.HasAxis = true
	return run
}

// groupRun has five invocations in dispatch order b-1 (builder), r-1 (reviewer),
// u-1 (no type), b-2 (builder), r-2 (reviewer). By agent type lists reviewer
// before builder, then the unknown type, as the analysis orders them by cost; b-2
// carries a logged compaction.
func groupRun() domain.RunReport {
	run := overviewRun()
	ents := []domain.TimelineEntry{
		tlClosed("b-1", "builder", tlAt(0), tlAt(10)),
		tlClosed("r-1", "reviewer", tlAt(10), tlAt(20)),
		tlClosed("u-1", "", tlAt(20), tlAt(30)),
		tlClosed("b-2", "builder", tlAt(30), tlAt(40)),
		tlClosed("r-2", "reviewer", tlAt(40), tlAt(50)),
	}
	run.Timeline = domain.Timeline{
		Entries: ents,
		Axis:    tlSpan(0, 50),
		HasAxis: true,
		Compactions: []domain.Compaction{
			{Actor: domain.AgentInstance("b-2"), At: tlAt(35), Trigger: "auto"},
		},
	}
	run.Context.Invocations = []domain.ActorContext{tlContext("b-2", "builder", 100_000, 80_000, tlLogged(35))}
	run.ByAgentType = []domain.AgentTypeGroup{
		{AgentType: "reviewer", Dispatches: []domain.Dispatch{tlDispatch(ents[1], 500, 10_000), tlDispatch(ents[4], 400, 10_000)}},
		{AgentType: "builder", Dispatches: []domain.Dispatch{tlDispatch(ents[0], 300, 10_000), tlDispatch(ents[3], 200, 10_000)}},
		{AgentType: "", Dispatches: []domain.Dispatch{tlDispatch(ents[2], 100, 10_000)}},
	}
	return run
}
