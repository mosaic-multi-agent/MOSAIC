package analysis_test

// Tests for the timeline axis (the run's logged span) and for overlaps of open
// invocations, which run to the axis end like the Gantt bars.
//
//	instance  start  end
//	w#4       5      20
//	o#1       20     (open)
//	o#2       25     (open)
//	y#2       30     50
//	z#3       55     60
//
// With no run end event the axis is 5..60.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func tlOpen(id domain.AgentInstanceID, start int) analysis.Stream {
	return tlInvocation(id, "coder", sec(start), nil, "")
}

func tlOpenScenario(t *testing.T, extra ...analysis.Stream) domain.Timeline {
	t.Helper()
	streams := []analysis.Stream{
		tlInvocation("w#4", "coder", sec(5), sec(20), "SUCCESS"),
		tlOpen("o#1", 20),
		tlOpen("o#2", 25),
		tlInvocation("y#2", "coder", sec(30), sec(50), "SUCCESS"),
		tlInvocation("z#3", "coder", sec(55), sec(60), "SUCCESS"),
	}
	in := analysis.Input{Streams: append(streams, extra...)}
	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)
	return requireRunReport(t, report, testRunID).Timeline
}

func TestTimelineAxis_RunsFromTheEarliestToTheLatestLoggedTimestamp(t *testing.T) {
	tl := tlTimeline(t)

	if !tl.HasAxis {
		t.Fatal("timeline has no axis, want the logged span")
	}
	if !tl.Axis.Start.Equal(rmAt(10)) || !tl.Axis.End.Equal(rmAt(70)) {
		t.Errorf("axis = %v..%v, want %v..%v", tl.Axis.Start, tl.Axis.End, rmAt(10), rmAt(70))
	}
}

func TestTimelineAxis_IncludesTheRunEndTimestamp(t *testing.T) {
	runEnd := analysis.Stream{Ref: orchStream(testRunRef), Events: []domain.Event{rfRunEnd(rmAt(90), "done")}}

	tl := tlOpenScenario(t, runEnd)

	if !tl.HasAxis || !tl.Axis.End.Equal(rmAt(90)) {
		t.Errorf("axis end = %v (has axis %v), want the run end %v", tl.Axis.End, tl.HasAxis, rmAt(90))
	}
}

func TestTimelineAxis_WithoutAnyTimestamp_IsAbsent(t *testing.T) {
	in := rmOrchestratorInput(rmToolStart("o1", "Read", time.Time{}))

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	tl := requireRunReport(t, report, testRunID).Timeline
	if tl.HasAxis {
		t.Errorf("axis = %v..%v, want none: nothing was logged with a time", tl.Axis.Start, tl.Axis.End)
	}
}

func TestTimelineOverlaps_OpenInvocation_OverlapsEveryIntervalRunningAfterItsStart(t *testing.T) {
	tl := tlOpenScenario(t)

	cases := map[domain.AgentInstanceID][]string{
		"o#1": {"o#2", "y#2", "z#3"},
		"o#2": {"o#1", "y#2", "z#3"},
		"y#2": {"o#1", "o#2"},
		"z#3": {"o#1", "o#2"},
	}
	for id, want := range cases {
		if got := tlOverlaps(tlEntry(t, tl, id)); !rmEqualStrings(got, want) {
			t.Errorf("%s overlaps = %v, want %v", id, got, want)
		}
	}
}

func TestTimelineOverlaps_ClosedIntervalEndingWhenAnOpenOneStarts_DoesNotOverlapIt(t *testing.T) {
	tl := tlOpenScenario(t)

	// w#4 ends at 20 exactly when o#1 starts: touching, so no overlap either way.
	if got := tlOverlaps(tlEntry(t, tl, "w#4")); len(got) != 0 {
		t.Errorf("w#4 overlaps = %v, want none", got)
	}
	listed := tlOverlaps(tlEntry(t, tl, "o#1"))
	if !rmEqualStrings(listed, []string{"o#2", "y#2", "z#3"}) {
		t.Errorf("o#1 overlaps = %v, want [o#2 y#2 z#3] without w#4, which ended when o#1 started", listed)
	}
}

func TestTimelineOverlaps_OpenInvocationsStartingAtTheAxisEnd_OverlapNothingUnlessTheAxisReachesPastThem(t *testing.T) {
	atEnd := analysis.Input{Streams: []analysis.Stream{tlOpen("p#1", 40), tlOpen("p#2", 40)}}
	pastEnd := analysis.Input{Streams: []analysis.Stream{
		tlOpen("p#1", 40), tlOpen("p#2", 40),
		{Ref: orchStream(testRunRef), Events: []domain.Event{rfRunEnd(rmAt(50), "done")}},
	}}
	price := func(in analysis.Input) domain.Timeline {
		report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)
		return requireRunReport(t, report, testRunID).Timeline
	}

	zeroLength := price(atEnd)
	reaching := price(pastEnd)

	if got := tlOverlaps(tlEntry(t, zeroLength, "p#1")); len(got) != 0 {
		t.Errorf("p#1 overlaps = %v, want none: both run 40..40", got)
	}
	if got := tlOverlaps(tlEntry(t, reaching, "p#1")); !rmEqualStrings(got, []string{"p#2"}) {
		t.Errorf("p#1 overlaps = %v, want [p#2]: both run 40..50 up to the run end", got)
	}
}

func TestTimelineOverlaps_OpenInvocationWithoutStart_HasNone(t *testing.T) {
	noStart := analysis.Stream{Ref: agentStream(testRunRef, "n#9"), Events: []domain.Event{rmUsage("n#9", "n-r1", 10)}}

	tl := tlOpenScenario(t, noStart)

	if got := tlOverlaps(tlEntry(t, tl, "n#9")); len(got) != 0 {
		t.Errorf("n#9 overlaps = %v, want none: it has no start", got)
	}
	for _, o := range tlOverlaps(tlEntry(t, tl, "o#1")) {
		if o == "n#9" {
			t.Error("o#1 lists n#9 although n#9 has no start")
		}
	}
}
