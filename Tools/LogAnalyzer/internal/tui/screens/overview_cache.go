package screens

import (
	"fmt"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"

	"mosaic-log-analyzer/internal/domain"
)

// The cache block of the Overview: the hit ratio (with a gauge in the visual
// version), the net cache benefit and the run's unread cache writes.

// gaugeWidth is the width of the cache hit gauge.
const gaugeWidth = 20

// cacheLines is the cache block; the run-level figures only, never the
// orchestrator's own.
func (s *OverviewScreen) cacheLines() []string {
	st, width := s.styles, s.width
	ratio := s.run.Overview.CacheHitRatio
	hit := st.Body.Render("Cache hit ratio  " + FormatRatio(ratio))
	if !s.figuresOnly {
		hit += "  " + chart.Bar(float64(ratio.Numerator), float64(ratio.Denominator), gaugeWidth,
			tuicommon.RoleDataTokens, st.Theme)
	}
	return append([]string{
		heading(st, width, "Cache"),
		hit,
		netBenefitLine(s.run.Cache, st),
		unreadWritesLine(s.run.Cache, st),
	}, rewriteLines(s.run, st)...)
}

// netBenefitLine shows the net cache benefit; a negative benefit is drawn in the
// bad role and keeps its sign as text.
func netBenefitLine(c domain.CacheFigures, st Styles) string {
	cell := moneyCell(st.Currency, c.NetBenefit, c.Complete)
	role := cell.Role
	if n, ok := moneyNanos(c.NetBenefit); ok && n < 0 {
		role = tuicommon.RoleSignalBad
	}
	return st.Body.Render("Net cache benefit  ") + st.Theme.Style(role).Render(cell.Text)
}

// unreadWritesLine shows the tokens and cost of the cache writes nothing read
// afterwards, labelled as reported data.
func unreadWritesLine(c domain.CacheFigures, st Styles) string {
	tokens := FormatTokens(c.UnreadWriteTokens)
	if c.UnreadWriteTokens.IsPresent() {
		tokens += " tokens"
	}
	cost := moneyCell(st.Currency, c.UnreadWriteCost, c.Complete)
	return st.Body.Render("Unread cache writes  "+tokens+"  ") + st.Theme.Style(cost.Role).Render(cost.Text) +
		st.Muted.Render("  (reported data)")
}

// rewriteLines shows the rewrite figures of the run and of the orchestrator
// beside the cache figures; a scope without rewrites gets no line.
func rewriteLines(run domain.RunReport, st Styles) []string {
	var lines []string
	if l := rewriteLine("Context rewrites", run.Cache.Rewrites, st); l != "" {
		lines = append(lines, l)
	}
	if l := rewriteLine("Orchestrator rewrites", run.OrchestratorCache.Rewrites, st); l != "" {
		lines = append(lines, l)
	}
	return lines
}

func rewriteLine(name string, f domain.RewriteFigures, st Styles) string {
	if f.Count() == 0 {
		return ""
	}
	cost := moneyCell(st.Currency, f.Cost, f.Complete)
	extra := moneyCell(st.Currency, f.ExtraCost, f.Complete)
	return st.Body.Render(fmt.Sprintf("%s  %d (model changed %d, cold %d, partial %d)  %s tokens  cost ",
		name, f.Count(), f.ModelChanged, f.ColdIdle, f.Partial, FormatTokens(f.Tokens))) +
		st.Theme.Style(cost.Role).Render(cost.Text) + st.Body.Render("  extra ") +
		st.Theme.Style(extra.Role).Render(extra.Text)
}
