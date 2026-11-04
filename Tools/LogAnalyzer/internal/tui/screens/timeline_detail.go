package screens

import (
	"strings"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// stripLines is the fixed line count of the Gantt's detail strip.
const stripLines = 3

// detailStrip renders the three lines for the invocation under the cursor:
// instance and type; logged start, end, duration and status; cost, the
// invocations it ran in parallel with, and peak and final context.
func (s *TimelineScreen) detailStrip(e domain.TimelineEntry) []string {
	inv := e.Invocation
	paint := func(role tuicommon.StyleRole, text string) string {
		return s.styles.Theme.Style(role).Render(text)
	}
	end := formatClock(inv.EndedAt)
	if inv.State == domain.InvocationOpen {
		end = OpenLabel
	}
	first := paint(tuicommon.RoleDataActor, inv.Actor.Label()) + "  " + typeLabel(inv.Actor.Type)
	second := paint(tuicommon.RoleDataTime, formatClock(inv.StartedAt)+" → "+end) + "  " +
		paint(tuicommon.RoleDataTime, FormatDuration(e.Duration, e.HasDuration)) + "  " +
		paint(StatusRole(inv), statusText(inv))

	d, hasDispatch := s.dispatchOf(inv.Actor.Instance)
	cost := mutedCell(AbsentMarker)
	if hasDispatch {
		cost = moneyTotalCell(s.styles.Currency, d.Totals.Money)
	}
	ctx, hasContext := s.run.Context.Find(inv.Actor.Instance)
	peak, final := domain.ContextFigure{}, domain.ContextFigure{}
	if hasContext {
		peak, final = ctx.Peak, ctx.Final
	}
	if hasDispatch {
		peak = d.PeakContext
	}
	third := "cost " + paint(cost.Role, cost.Text) + "  parallel with " + parallelText(e) +
		"  peak " + contextWithBand(peak, paint) + "  final " + contextWithBand(final, paint)
	return []string{first, second, third}
}

// dispatchOf finds the dispatch row of an invocation among the by-type groups.
func (s *TimelineScreen) dispatchOf(id domain.AgentInstanceID) (domain.Dispatch, bool) {
	for _, g := range s.run.ByAgentType {
		for _, d := range g.Dispatches {
			if d.Invocation.Actor.Instance == id {
				return d, true
			}
		}
	}
	return domain.Dispatch{}, false
}

// parallelText lists the invocations that ran in parallel with an entry, as the
// analysis counts them.
func parallelText(e domain.TimelineEntry) string {
	if len(e.Overlaps) == 0 {
		return "none"
	}
	ids := make([]string, len(e.Overlaps))
	for i, id := range e.Overlaps {
		ids[i] = string(id)
	}
	return strings.Join(ids, ", ")
}

// contextWithBand renders a context size with its band name, both in the band's
// role; an absent figure shows the absent marker.
func contextWithBand(f domain.ContextFigure, paint func(tuicommon.StyleRole, string) string) string {
	c := contextCell(f)
	if !f.Tokens.IsPresent() {
		return paint(c.Role, c.Text)
	}
	return paint(c.Role, c.Text+" ("+f.Band.Label()+")")
}
