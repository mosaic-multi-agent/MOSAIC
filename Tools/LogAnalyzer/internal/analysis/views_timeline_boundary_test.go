package analysis_test

// Boundary cases of the timeline overlap rule (A.start < B.end && B.start <
// A.end) with zero-length and identical intervals.
//
//	instance  start  end
//	x#1       10     20
//	z#2       15     15   (zero length, inside x#1)
//	e#3       10     10   (zero length, at the start edge of x#1)
//	p#4       30     40
//	q#5       30     40   (identical to p#4)

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func tlBoundaryTimeline(t *testing.T) domain.Timeline {
	t.Helper()
	in := analysis.Input{Streams: []analysis.Stream{
		tlInvocation("x#1", "coder", sec(10), sec(20), "SUCCESS"),
		tlInvocation("z#2", "coder", sec(15), sec(15), "SUCCESS"),
		tlInvocation("e#3", "coder", sec(10), sec(10), "SUCCESS"),
		tlInvocation("p#4", "coder", sec(30), sec(40), "SUCCESS"),
		tlInvocation("q#5", "coder", sec(30), sec(40), "SUCCESS"),
	}}
	return requireRunReport(t, analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil), testRunID).Timeline
}

func TestTimeline_ZeroLengthInvocation_HasZeroDurationAndOverlapsOnlyWhenStrictlyInside(t *testing.T) {
	tl := tlBoundaryTimeline(t)

	z := tlEntry(t, tl, "z#2")
	if !z.HasDuration || z.Duration != 0 {
		t.Errorf("z#2 duration = %v (has %v), want a zero duration", z.Duration, z.HasDuration)
	}
	cases := map[domain.AgentInstanceID][]string{
		"x#1": {"z#2"},
		"z#2": {"x#1"},
		"e#3": {},
	}
	for id, want := range cases {
		if got := tlOverlaps(tlEntry(t, tl, id)); !rmEqualStrings(got, want) {
			t.Errorf("%s overlaps = %v, want %v", id, got, want)
		}
	}
}

func TestTimeline_IdenticalIntervals_OverlapEachOther(t *testing.T) {
	tl := tlBoundaryTimeline(t)

	cases := map[domain.AgentInstanceID][]string{"p#4": {"q#5"}, "q#5": {"p#4"}}
	for id, want := range cases {
		if got := tlOverlaps(tlEntry(t, tl, id)); !rmEqualStrings(got, want) {
			t.Errorf("%s overlaps = %v, want %v", id, got, want)
		}
	}
}
