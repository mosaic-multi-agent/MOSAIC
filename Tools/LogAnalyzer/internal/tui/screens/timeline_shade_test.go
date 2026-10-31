package screens_test

// Shading on the Gantt: each actor's waits on the user are shaded in that actor's
// own lane only, between-session time across every lane, and neither covers a bar
// of another agent. In ASCII mode a bar is '=', a wait '-' and a gap '_'.

import (
	"strings"
	"testing"

	"mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	asciiBar  = '='
	asciiWait = '-'
	asciiGap  = '_'
)

// shadedRun is timelineRun with the given per-actor waits and between-session intervals.
func shadedRun(waits []domain.ActorWaitIntervals, between ...domain.TimeInterval) domain.RunReport {
	run := timelineRun()
	run.WallTime.ActorWaits = waits
	run.WallTime.BetweenSessionIntervals = between
	return run
}

func tlWaitOf(actor domain.ActorRef, spans ...domain.TimeInterval) domain.ActorWaitIntervals {
	return domain.ActorWaitIntervals{Actor: actor, Intervals: spans}
}

// asciiTimeline renders the Gantt in ASCII mode with colour off.
func asciiTimeline(t *testing.T, run domain.RunReport, w, h int) string {
	t.Helper()
	asciiOn(t)
	return screens.NewTimelineScreen(run, w, h, paletteStyles()).View(w, h)
}

// plotPart is the plot area of a lane line: the orchestrator label is the widest
// label, so every plot starts after it and one blank.
func plotPart(line string) string {
	r := []rune(line)
	if cut := len("orchestrator") + 1; len(r) > cut {
		return string(r[cut:])
	}
	return ""
}

// laneGlyphCols are the plot columns of a glyph on a lane.
func laneGlyphCols(line string, glyph rune) []int { return glyphCols(plotPart(line), glyph) }

// assertNoGlyph fails when any of the given lanes holds the glyph in its plot area.
func assertNoGlyph(t *testing.T, view string, glyph rune, what string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		var line string
		if id == "orchestrator" {
			line = orchestratorLane(t, view)
		} else {
			line = laneLine(t, view, id, timelineTypes[id])
		}
		if n := countGlyph(plotPart(line), glyph); n != 0 {
			t.Errorf("%s: the lane of %s must not hold %q, found %d\n%s", what, id, string(glyph), n, stripStyling(view))
		}
	}
}

func TestGanttShading_NothingIsShadedWithoutWaitsOrGaps(t *testing.T) {
	view := asciiTimeline(t, timelineRun(), wideW, tallH)

	for _, l := range laneAndOrchestratorLines(view) {
		if strings.ContainsRune(plotPart(l), asciiWait) || strings.ContainsRune(plotPart(l), asciiGap) {
			t.Errorf("no lane is shaded without waits or gaps: %q", l)
		}
	}
}

func TestGanttShading_SubagentsOwnWait_IsShadedOverItsOwnBarWhichContinuesOnBothSides(t *testing.T) {
	run := shadedRun([]domain.ActorWaitIntervals{tlWaitOf(domain.AgentInstance("beta#3"), tlSpan(50, 60))})
	view := asciiTimeline(t, run, wideW, tallH)

	beta := laneLine(t, view, "beta#3", "tester")
	wait, bar := laneGlyphCols(beta, asciiWait), laneGlyphCols(beta, asciiBar)

	if len(wait) == 0 {
		t.Fatalf("beta#3's wait from 50s to 60s must be shaded on its own lane\n%s", stripStyling(view))
	}
	if bar[0] >= wait[0] || bar[len(bar)-1] <= wait[len(wait)-1] {
		t.Errorf("beta#3's bar must continue on both sides of its wait: bar %d..%d, wait %d..%d",
			bar[0], bar[len(bar)-1], wait[0], wait[len(wait)-1])
	}
	assertNoGlyph(t, view, asciiWait, "a wait of beta#3", "orchestrator", "solo#1", "alpha#2", "rev#4")
}

func TestGanttShading_OrchestratorWaitDuringSubagentWork_IsOnlyInTheOrchestratorLane(t *testing.T) {
	run := shadedRun([]domain.ActorWaitIntervals{tlWaitOf(domain.Orchestrator(), tlSpan(55, 70))})
	view := asciiTimeline(t, run, wideW, tallH)

	if len(laneGlyphCols(orchestratorLane(t, view), asciiWait)) == 0 {
		t.Fatalf("the orchestrator's wait from 55s to 70s must be shaded on its lane\n%s", stripStyling(view))
	}
	// beta#3 (30s-75s) and rev#4 (open from 40s) work through the whole wait.
	assertNoGlyph(t, view, asciiWait, "an orchestrator wait during subagent work", "beta#3", "rev#4", "alpha#2", "solo#1")
}

func TestGanttShading_BarsOfOtherAgentsAreUnchangedByAWait(t *testing.T) {
	plain := asciiTimeline(t, timelineRun(), wideW, tallH)
	run := shadedRun([]domain.ActorWaitIntervals{tlWaitOf(domain.Orchestrator(), tlSpan(0, 75))})
	shaded := asciiTimeline(t, run, wideW, tallH)

	for id, typ := range timelineTypes {
		if id == "delta#5" {
			continue
		}
		a, b := laneGlyphCols(laneLine(t, plain, id, typ), asciiBar), laneGlyphCols(laneLine(t, shaded, id, typ), asciiBar)
		if len(a) == 0 || len(a) != len(b) || a[0] != b[0] || a[len(a)-1] != b[len(b)-1] {
			t.Errorf("the orchestrator's wait must not change the bar of %s: %v vs %v", id, a, b)
		}
	}
	if len(laneGlyphCols(orchestratorLane(t, shaded), asciiWait)) == 0 {
		t.Errorf("a wait over the whole run must be shaded on the orchestrator lane")
	}
}

func TestGanttShading_WaitOfAnActorWithoutALane_IsDrawnAcrossLanesButNotOverBars(t *testing.T) {
	run := shadedRun([]domain.ActorWaitIntervals{tlWaitOf(domain.AgentInstance("ghost#10"), tlSpan(60, 70))})
	view := asciiTimeline(t, run, wideW, tallH)

	for _, id := range []string{"orchestrator", "solo#1", "alpha#2"} {
		line := orchestratorLane(t, view)
		if id != "orchestrator" {
			line = laneLine(t, view, id, timelineTypes[id])
		}
		if len(laneGlyphCols(line, asciiWait)) == 0 {
			t.Errorf("a wait of an actor without a lane is drawn in the lane of %s\n%s", id, stripStyling(view))
		}
	}
	// beta#3 works from 30s to 75s: its bar is never shaded over by another agent's wait.
	assertNoGlyph(t, view, asciiWait, "a wait of an actor without a lane", "beta#3", "rev#4")
}

func TestGanttShading_BetweenSessionsIsShadedAcrossEveryLaneWithoutCoveringBars(t *testing.T) {
	run := shadedRun(nil, tlSpan(12, 18))
	view := asciiTimeline(t, run, wideW, tallH)

	var cols [][]int
	for id, typ := range timelineTypes {
		if id == "delta#5" {
			continue
		}
		cols = append(cols, laneGlyphCols(laneLine(t, view, id, typ), asciiGap))
	}
	cols = append(cols, laneGlyphCols(orchestratorLane(t, view), asciiGap))

	first := cols[0]
	if len(first) == 0 {
		t.Fatalf("the gap from 12s to 18s must be shaded in every lane\n%s", stripStyling(view))
	}
	for _, c := range cols {
		if len(c) != len(first) || c[0] != first[0] || c[len(c)-1] != first[len(first)-1] {
			t.Errorf("between-sessions time covers the same columns in every lane: %v vs %v", c, first)
		}
	}
	solo := laneGlyphCols(laneLine(t, view, "solo#1", "planner"), asciiBar)
	alpha := laneGlyphCols(laneLine(t, view, "alpha#2", "coder"), asciiBar)
	if first[0] <= solo[len(solo)-1] || first[len(first)-1] >= alpha[0] {
		t.Errorf("the gap lies between solo#1's end (%d) and alpha#2's start (%d): shade %d..%d",
			solo[len(solo)-1], alpha[0], first[0], first[len(first)-1])
	}
}

func TestGanttShading_BetweenSessionsNeverReplacesABar(t *testing.T) {
	plain := asciiTimeline(t, timelineRun(), wideW, tallH)
	shaded := asciiTimeline(t, shadedRun(nil, tlSpan(0, 75)), wideW, tallH)

	for id, typ := range timelineTypes {
		if id == "delta#5" {
			continue
		}
		a, b := laneGlyphCols(laneLine(t, plain, id, typ), asciiBar), laneGlyphCols(laneLine(t, shaded, id, typ), asciiBar)
		if len(a) != len(b) {
			t.Errorf("between-sessions shading must not change the bar of %s: %v vs %v", id, a, b)
		}
	}
	if len(laneGlyphCols(laneLine(t, shaded, "solo#1", "planner"), asciiGap)) == 0 {
		t.Errorf("the part of solo#1's lane outside its bar must be shaded")
	}
}

func TestGanttShading_TheRawWaitAndGapListsAreNoLongerDrawn(t *testing.T) {
	run := timelineRun()
	run.WallTime.WaitIntervals = []domain.TimeInterval{tlSpan(55, 70)}
	run.WallTime.SessionGaps = []domain.TimeInterval{tlSpan(12, 18)}
	view := asciiTimeline(t, run, wideW, tallH)

	for _, l := range laneAndOrchestratorLines(view) {
		if strings.ContainsRune(plotPart(l), asciiWait) || strings.ContainsRune(plotPart(l), asciiGap) {
			t.Errorf("shading comes from the per-actor waits and the between-session intervals only: %q", l)
		}
	}
}

func TestGanttShading_WaitAndGapUseTheirOwnFlatShadeRolesInColour(t *testing.T) {
	colourOn(t)
	run := shadedRun([]domain.ActorWaitIntervals{tlWaitOf(domain.Orchestrator(), tlSpan(55, 70))}, tlSpan(12, 18))
	view := screens.NewTimelineScreen(run, wideW, tallH, paletteStyles()).View(wideW, tallH)
	theme := tui.PaletteTheme()
	neutral, cool := theme.Style(tui.RoleShadeNeutral).Render(" "), theme.Style(tui.RoleShadeCool).Render(" ")

	var orch string
	for _, l := range strings.Split(view, "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(stripStyling(l))), "orchestrator") {
			orch = l
		}
	}

	if neutral == " " || cool == " " || neutral == cool {
		t.Fatalf("the two shade roles must be visible and different: %q and %q", neutral, cool)
	}
	if !strings.Contains(orch, neutral) || !strings.Contains(orch, cool) {
		t.Errorf("the orchestrator lane holds the wait and the gap as blank cells in their roles: %q", orch)
	}
	if strings.ContainsRune(orch, '░') {
		t.Errorf("shades are flat, no dotted glyph: %q", orch)
	}
}
