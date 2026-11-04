package screens

import (
	"time"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
)

// The Gantt version of the timeline: an axis line, the orchestrator lane, one
// lane per invocation, the legend and the detail strip of the lane under the cursor.

// orchestratorLaneLabel labels the lane of the orchestrator's compactions.
const orchestratorLaneLabel = "orchestrator"

// ganttFixedLines are the body lines besides the invocation lanes: the axis, the
// orchestrator lane and the three lines of the detail strip.
const ganttFixedLines = 2 + stripLines

// ganttBody renders the Gantt's body lines for the terminal size and the scroll
// hint of its lanes. Only the visible lanes are drawn.
func (s *TimelineScreen) ganttBody(width, height int) ([]string, string) {
	st := s.styles
	tl := s.run.Timeline
	if len(tl.Entries) == 0 && len(tl.Compactions) == 0 {
		return []string{clip(st.Body, width, "No invocations or compactions in this run.")}, ""
	}
	entries := s.orderedEntries()
	axis, shades := s.ganttAxis(), s.ganttShades()
	labelWidth := ganttLabelWidth(entries, width)

	out := []string{gantt.RenderAxis(axis, labelWidth, width, st.Theme),
		gantt.RenderLane(axis, s.orchestratorLane(), shades, labelWidth, width, false, st.Theme)}
	if len(entries) == 0 {
		return out, ""
	}
	s.cursor = clampIndex(s.cursor, len(entries))
	legend := s.ganttLegend(width)
	room := FrameBodyHeight(height) - ganttFixedLines
	if room-len(legend) < 1 {
		legend = nil
	}
	w := ScrollWindow(s.offset, s.cursor, len(entries), max(room-len(legend), 1))
	s.offset = w.Offset
	for i := w.Offset; i < w.End(); i++ {
		out = append(out, gantt.RenderLane(axis, s.invocationLane(entries[i]), shades, labelWidth, width, i == s.cursor, st.Theme))
	}
	out = append(out, legend...)
	out = append(out, s.detailStrip(entries[s.cursor])...)
	return out, FormatScrollHint(w, len(entries))
}

// ganttLabelWidth is the width of the lane labels: the widest label, but at most
// a third of the terminal so the plot area keeps most of the width.
func ganttLabelWidth(entries []domain.TimelineEntry, width int) int {
	w := textwidth.Of(orchestratorLaneLabel)
	for _, e := range entries {
		w = max(w, textwidth.Of(e.Invocation.Actor.Label()))
	}
	return min(w, max(width/3, 4))
}

// ganttAxis is the run's logged span. A span of no length is widened to one
// second so an instant still has a cell to be drawn in.
func (s *TimelineScreen) ganttAxis() gantt.Axis {
	tl := s.run.Timeline
	if !tl.HasAxis {
		return gantt.Axis{}
	}
	end := tl.Axis.End
	if !end.After(tl.Axis.Start) {
		end = tl.Axis.Start.Add(time.Second)
	}
	return gantt.Axis{Start: tl.Axis.Start, End: end, Collapsed: s.collapsedStretches()}
}

// orchestratorLane carries one marker per compaction of the orchestrator.
func (s *TimelineScreen) orchestratorLane() gantt.Lane {
	return gantt.Lane{Label: orchestratorLaneLabel, LabelRole: tuicommon.RoleDataActor,
		Points: markerPoints(s.run.Context.Orchestrator.Compactions), Shades: s.actorWaitShades(domain.Orchestrator())}
}

// invocationLane is one invocation's lane: its bar and its own compactions.
func (s *TimelineScreen) invocationLane(e domain.TimelineEntry) gantt.Lane {
	inv := e.Invocation
	lane := gantt.Lane{Label: inv.Actor.Label(), LabelRole: tuicommon.RoleDataActor, Shades: s.actorWaitShades(inv.Actor)}
	if iv, ok := invocationInterval(inv); ok {
		lane.Intervals = []gantt.Interval{iv}
	}
	if ctx, ok := s.run.Context.Find(inv.Actor.Instance); ok {
		lane.Points = markerPoints(ctx.Compactions)
	}
	return lane
}

// invocationInterval is the bar of an invocation in its status role. An open one
// runs to the end of the axis; one without a logged start or end has no bar.
func invocationInterval(inv domain.Invocation) (gantt.Interval, bool) {
	if inv.StartedAt.IsZero() {
		return gantt.Interval{}, false
	}
	role := StatusRole(inv)
	if inv.State == domain.InvocationOpen {
		return gantt.Interval{Start: inv.StartedAt, Open: true, Role: role}, true
	}
	if inv.EndedAt.IsZero() {
		return gantt.Interval{}, false
	}
	return gantt.Interval{Start: inv.StartedAt, End: inv.EndedAt, Role: role}, true
}

// markerPoints turns compaction marks into lane markers: a logged compaction and
// a detected one use different marker kinds.
func markerPoints(marks []domain.CompactionMark) []gantt.Point {
	points := make([]gantt.Point, 0, len(marks))
	for _, m := range marks {
		kind := chart.MarkerSecondary
		if m.Kind == domain.CompactionDetected {
			kind = chart.MarkerTertiary
		}
		points = append(points, gantt.Point{At: m.At, Kind: kind, Role: tuicommon.RoleBody})
	}
	return points
}

// orderedEntries are the timeline entries in the order of the lanes: dispatch
// order, or grouped by agent type in the order of the by-type view with dispatch
// order inside a group.
func (s *TimelineScreen) orderedEntries() []domain.TimelineEntry {
	entries := s.run.Timeline.Entries
	if !s.grouped {
		return entries
	}
	var types []string
	seen := map[string]bool{}
	for _, g := range s.run.ByAgentType {
		types, seen[g.AgentType] = append(types, g.AgentType), true
	}
	for _, e := range entries {
		if t := e.Invocation.Actor.Type; !seen[t] {
			types, seen[t] = append(types, t), true
		}
	}
	out := make([]domain.TimelineEntry, 0, len(entries))
	for _, t := range types {
		for _, e := range entries {
			if e.Invocation.Actor.Type == t {
				out = append(out, e)
			}
		}
	}
	return out
}

// collapsedStretches are the wall-time analysis stretches the axis draws as markers.
func (s *TimelineScreen) collapsedStretches() []gantt.Collapse {
	var out []gantt.Collapse
	for _, iv := range s.run.WallTime.Collapsible {
		out = append(out, gantt.Collapse{Start: iv.Start, End: iv.End})
	}
	return out
}
