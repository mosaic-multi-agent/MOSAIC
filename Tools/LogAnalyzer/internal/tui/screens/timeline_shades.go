package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"

	"mosaic-log-analyzer/internal/domain"
)

// Names of the two shades in the Overview's terms.
const (
	waitingShadeName = "waiting on the user"
	betweenShadeName = "between sessions"
)

// waitShade is the flat shade of a wait on the user.
func waitShade(iv domain.TimeInterval) gantt.Shade {
	return gantt.Shade{Start: iv.Start, End: iv.End, Role: tuicommon.RoleShadeNeutral, Flat: true, ASCII: '-'}
}

// betweenShade is the flat shade of time between sessions.
func betweenShade(iv domain.TimeInterval) gantt.Shade {
	return gantt.Shade{Start: iv.Start, End: iv.End, Role: tuicommon.RoleShadeCool, Flat: true, ASCII: '_'}
}

// actorWaitShades are the waits of one actor as lane shades.
func (s *TimelineScreen) actorWaitShades(actor domain.ActorRef) []gantt.Shade {
	var out []gantt.Shade
	for _, w := range s.run.WallTime.ActorWaits {
		if sameActor(w.Actor, actor) {
			for _, iv := range w.Intervals {
				out = append(out, waitShade(iv))
			}
		}
	}
	return out
}

func sameActor(a, b domain.ActorRef) bool {
	return a.Kind == b.Kind && a.Instance == b.Instance
}

// hasLane reports whether the actor has a Timeline lane: the orchestrator or an
// invocation of the timeline.
func (s *TimelineScreen) hasLane(actor domain.ActorRef) bool {
	if actor.Kind != domain.ActorAgentInstance {
		return true
	}
	for _, e := range s.run.Timeline.Entries {
		if e.Invocation.Actor.Instance == actor.Instance {
			return true
		}
	}
	return false
}

// ganttShades are the shades drawn across every lane: the time between sessions
// and the waits of actors that have no lane. Other waits are shaded per lane.
func (s *TimelineScreen) ganttShades() []gantt.Shade {
	wt := s.run.WallTime
	var out []gantt.Shade
	for _, w := range wt.ActorWaits {
		if s.hasLane(w.Actor) {
			continue
		}
		for _, iv := range w.Intervals {
			out = append(out, waitShade(iv))
		}
	}
	for _, iv := range wt.BetweenSessionIntervals {
		out = append(out, betweenShade(iv))
	}
	return out
}

// ganttLegend names the two shades and the compaction markers the screen draws.
func (s *TimelineScreen) ganttLegend(width int) []string {
	theme := s.styles.Theme
	var entries []gantt.LegendEntry
	entries = append(entries,
		gantt.LegendEntry{Swatch: gantt.ShadeSwatch(waitShade(domain.TimeInterval{}), theme), Label: waitingShadeName},
		gantt.LegendEntry{Swatch: gantt.ShadeSwatch(betweenShade(domain.TimeInterval{}), theme), Label: betweenShadeName})
	logged, detected := s.drawnMarkerKinds()
	if logged {
		entries = append(entries, gantt.LegendEntry{
			Swatch: gantt.PointSwatch(gantt.Point{Kind: chart.MarkerSecondary, Role: tuicommon.RoleBody}, theme), Label: "compaction (logged)"})
	}
	if detected {
		entries = append(entries, gantt.LegendEntry{
			Swatch: gantt.PointSwatch(gantt.Point{Kind: chart.MarkerTertiary, Role: tuicommon.RoleBody}, theme), Label: "compaction (detected)"})
	}
	return gantt.RenderLegend(entries, width, theme)
}

// drawnMarkerKinds reports which compaction kinds the lanes draw.
func (s *TimelineScreen) drawnMarkerKinds() (logged, detected bool) {
	note := func(marks []domain.CompactionMark) {
		for _, m := range marks {
			if m.Kind == domain.CompactionDetected {
				detected = true
			} else {
				logged = true
			}
		}
	}
	note(s.run.Context.Orchestrator.Compactions)
	for _, e := range s.run.Timeline.Entries {
		if ctx, ok := s.run.Context.Find(e.Invocation.Actor.Instance); ok {
			note(ctx.Compactions)
		}
	}
	return logged, detected
}
