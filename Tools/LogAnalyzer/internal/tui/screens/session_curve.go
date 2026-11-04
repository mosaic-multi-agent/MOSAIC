package screens

import (
	"fmt"
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"

	"mosaic-log-analyzer/internal/domain"
)

// The context part of the Session screen: the actor's context figures with band
// names, its compactions, and the context curve with band guides and markers.

// maxCurveHeight is the tallest the context curve is drawn.
const maxCurveHeight = 12

// minCurveHeight is the shortest curve worth drawing; below it the chart is left out.
const minCurveHeight = 4

// contextFigureLines are the lines that describe an actor's context: its figures
// with the band of each named as text, its compactions and the definitions behind them.
func contextFigureLines(st Styles, width int, a domain.ActorContext) []string {
	var head []string
	paint := func(role tuicommon.StyleRole, text string) string { return st.Theme.Style(role).Render(text) }
	figure := func(name string, f domain.ContextFigure) {
		head = append(head, name+" "+contextWithBand(f, paint))
	}
	figure("Peak context:", a.Peak)
	figure("Final context:", a.Final)
	figure("First write context*:", a.FirstWrite)
	for _, l := range compactionLines(st.Currency, a) {
		head = append(head, clip(st.Body, width, l))
	}
	for _, note := range []string{
		"* heuristic. Final = " + domain.FinalContextDefinition + ".",
		"Bands: under 250,000 optimal, 250,000 to 350,000 above optimal, from 350,000 degraded.",
	} {
		for _, l := range strings.Split(tuicommon.Wrap(note, width), "\n") {
			head = append(head, st.Muted.Render(l))
		}
	}
	return head
}

// contextCurve is the curve of the actor's call contexts with a guide per band
// and a marker for the first write and for each compaction.
func contextCurve(a domain.ActorContext) chart.Curve {
	c := chart.Curve{YLabel: func(v float64) string { return fmt.Sprintf("%dk", int(v/1000+0.5)) }}
	// Only main calls are drawn; mainIndex maps a call to its point (-1: none).
	mainIndex := make([]int, len(a.Calls))
	for i, call := range a.Calls {
		mainIndex[i] = -1
		if isMainCall(call) {
			mainIndex[i] = len(c.Values)
			c.Values = append(c.Values, float64(call.Context.ValueOr(0)))
		}
	}
	for _, r := range domain.ContextBandRanges() {
		c.Guides = append(c.Guides, chart.Guide{From: float64(r.From), To: float64(r.To), Role: BandRole(r.Band), Label: r.Band.Label()})
	}
	for i, call := range a.Calls {
		if call.FirstWrite && mainIndex[i] >= 0 {
			c.Markers = append(c.Markers, chart.Marker{Index: mainIndex[i], Kind: chart.MarkerPrimary})
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
		if idx >= 0 && idx < len(mainIndex) && mainIndex[idx] >= 0 {
			c.Markers = append(c.Markers, chart.Marker{Index: mainIndex[idx], Kind: kind})
		}
	}
	return c
}
