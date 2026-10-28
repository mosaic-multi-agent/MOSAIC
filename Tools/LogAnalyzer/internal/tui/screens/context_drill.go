package screens

import (
	"fmt"
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"

	"mosaic-log-analyzer/internal/domain"
)

// The drill-down of one actor of the Context view: its figures with band names,
// the context curve with band guides and markers, and the table of its calls.

// maxCurveHeight is the tallest the context curve is drawn.
const maxCurveHeight = 12

// minCurveHeight is the shortest curve worth drawing; below it the chart is left out.
const minCurveHeight = 4

// viewDrill renders the drill-down of the actor chosen in the table.
func (s *ContextScreen) viewDrill() string {
	st, width := s.styles, s.width
	a := s.drillActor
	body := s.drillHead(a)
	budget := bodyBudget(s.height)

	var w Window
	if len(a.Calls) == 0 {
		body = append(body, st.Body.Render("No per-call records for this actor."))
	} else {
		curveH := min((budget-len(body))/2, maxCurveHeight)
		if curveH >= minCurveHeight {
			body = append(body, strings.Split(chart.RenderCurve(contextCurve(a), width, curveH, st.Theme), "\n")...)
		}
		var lines []string
		lines, w = callTableLines(a, s.drillSort.Active(), s.drillCursor, &s.drillOffset, width, budget-len(body)-1, st)
		body = append(body, lines...)
	}
	spec := frameSpec{run: s.run, active: ScreenContext, keys: s.helpKeys(), screenKeys: s.screenKeys(), escDesc: s.escDesc(),
		hint: FormatScrollHint(w, len(a.Calls)), sort: s.drillSort.Label()}
	return s.render(spec, body, width, s.height, st)
}

// drillHead are the lines above the curve: who the actor is, its figures with the
// band of each named as text, its compactions and the definitions behind them.
func (s *ContextScreen) drillHead(a domain.ActorContext) []string {
	st := s.styles
	label := orchestratorRowLabel
	if a.Actor.Kind != domain.ActorOrchestrator && a.Actor.Label() != orchestratorRowLabel {
		label = a.Actor.Label() + " (" + typeLabel(a.Actor.Type) + ")"
	}
	if models := modelsOf(a.Calls); len(models) > 0 {
		label += "  " + strings.Join(models, ",")
	}
	head := []string{clip(st.Subtitle, s.width, label)}
	paint := func(role tuicommon.StyleRole, text string) string { return st.Theme.Style(role).Render(text) }
	figure := func(name string, f domain.ContextFigure) {
		head = append(head, name+" "+contextWithBand(f, paint))
	}
	figure("Peak context:", a.Peak)
	figure("Final context:", a.Final)
	figure("First write context*:", a.FirstWrite)
	head = append(head, clip(st.Body, s.width, fmt.Sprintf("Compactions: %d logged, %d detected",
		a.LoggedCount(), a.DetectedCount())))
	for _, note := range []string{
		"* heuristic. Final = " + domain.FinalContextDefinition + ".",
		"Bands: under 100,000 below optimal, 100,000 to 250,000 optimal, 250,000 to 350,000 above optimal, from 350,000 degraded.",
	} {
		for _, l := range strings.Split(tuicommon.Wrap(note, s.width), "\n") {
			head = append(head, st.Muted.Render(l))
		}
	}
	return head
}

// contextCurve is the curve of the actor's call contexts with a guide per band
// and a marker for the first write and for each compaction.
func contextCurve(a domain.ActorContext) chart.Curve {
	c := chart.Curve{YLabel: func(v float64) string { return fmt.Sprintf("%dk", int(v/1000+0.5)) }}
	for _, call := range a.Calls {
		c.Values = append(c.Values, float64(call.Context.ValueOr(0)))
	}
	for _, r := range domain.ContextBandRanges() {
		c.Guides = append(c.Guides, chart.Guide{From: float64(r.From), To: float64(r.To), Role: BandRole(r.Band), Label: r.Band.Label()})
	}
	for i, call := range a.Calls {
		if call.FirstWrite {
			c.Markers = append(c.Markers, chart.Marker{Index: i, Kind: chart.MarkerPrimary})
		}
	}
	for _, m := range a.Compactions {
		kind := chart.MarkerSecondary
		if m.Kind == domain.CompactionDetected {
			kind = chart.MarkerTertiary
		}
		idx := m.After
		if idx < 0 {
			idx = m.Before
		}
		c.Markers = append(c.Markers, chart.Marker{Index: idx, Kind: kind})
	}
	return c
}
