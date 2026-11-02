package screens_test

// Behaviour of the timeline that holds in both versions: an invocation is never
// its own parallel entry, and the parallel entries named for one invocation are
// exactly the ones whose logged intervals intersect it. The Gantt lanes are in
// timeline_gantt_test.go, the markers and shading in timeline_markers_test.go,
// the cursor and detail strip in timeline_detail_test.go and the list version in
// timeline_list_test.go.

import (
	"strings"
	"testing"
)

func TestTimeline_InvocationIsNeverItsOwnParallelEntry(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)

	for _, c := range []struct{ id, cost string }{
		{"solo#1", "$3.17"}, {"alpha#2", "$2.28"}, {"beta#3", "$1.39"}, {"rev#4", "$0.54"},
	} {
		line := stripLine(t, s.View(wideW, tallH), c.cost)
		if strings.Contains(line, c.id) {
			t.Errorf("the line of %s names itself as a parallel entry: %q", c.id, line)
		}
		press(s, "down")
	}
}

func TestTimeline_ParallelEntriesAreSymmetricBetweenOverlappingInvocations(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)

	press(s, "down") // alpha#2 names beta#3 ...
	alpha := stripLine(t, s.View(wideW, tallH), "$2.28")
	press(s, "down") // ... and beta#3 names alpha#2
	beta := stripLine(t, s.View(wideW, tallH), "$1.39")

	assertContains(t, alpha, "beta#3")
	assertContains(t, beta, "alpha#2")
}
