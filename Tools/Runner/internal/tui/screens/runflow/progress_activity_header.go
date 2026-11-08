package runflow

import (
	"fmt"
	"strings"
	"time"

	"mosaic-run/internal/domain"
)

// stallHighlightAfter is the duration from which the stall indicator is highlighted.
const stallHighlightAfter = 5 * time.Minute

// formatContextTokens renders a context size compactly, e.g. 112000 as "112k".
func formatContextTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%dk", (n+500)/1000)
}

// stallIndicator returns the indicator text of a running invocation and
// whether it has lasted long enough to be highlighted. A tool call that has
// started and not finished counts from when it was first reported; otherwise
// the time since the last stream event counts.
func (s *ProgressScreen) stallIndicator(now time.Time) (string, bool) {
	a := &s.activity
	if a.status.RunningTool != "" && !a.toolSince.IsZero() {
		d := now.Sub(a.toolSince)
		return "running " + a.status.RunningTool + " " + formatStall(d), d >= stallHighlightAfter
	}
	last := a.statusAt
	if last.IsZero() {
		last = a.started
	}
	d := now.Sub(last)
	return "quiet " + formatStall(d), d >= stallHighlightAfter
}

func formatStall(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Round(time.Second).String()
}

// activityHeader renders the panel's first line: whose activity, harness and
// model, elapsed time, context size of the latest model call, and either the
// stall indicator (running) or the ended mark and final cost (ended).
func (s *ProgressScreen) activityHeader() string {
	a := &s.activity
	who := "Activity"
	if a.inv.Source == domain.ActivitySourceConsultation {
		who = "Activity (consultation)"
	}
	h := fmt.Sprintf("%s: %s", who, a.inv.Agent)
	switch {
	case a.inv.Harness != "" && a.status.Model != "":
		h += fmt.Sprintf(" (%s, %s)", a.inv.Harness, a.status.Model)
	case a.inv.Harness != "":
		h += fmt.Sprintf(" (%s)", a.inv.Harness)
	}
	now := s.activityNow()
	end := now
	if !a.endedAt.IsZero() {
		end = a.endedAt
	}
	h += "  " + end.Sub(a.started).Round(time.Second).String()
	if !a.endedAt.IsZero() {
		h += " ended"
	}
	if a.status.HasContext {
		h += "  ctx " + formatContextTokens(a.status.ContextTokens)
	}
	if !a.endedAt.IsZero() {
		if a.status.HasCost {
			h += fmt.Sprintf("  $%.2f", a.status.CostUSD)
		}
		return s.styles.Selected.Render(fitWidth(h, s.width))
	}
	ind, hot := s.stallIndicator(now)
	full := h + "  " + ind
	fitted := fitWidth(full, s.width)
	if hot && strings.HasSuffix(fitted, ind) {
		head := strings.TrimSuffix(fitted, ind)
		return s.styles.Selected.Render(head) + s.styles.Warning.Render(ind)
	}
	return s.styles.Selected.Render(fitted)
}
