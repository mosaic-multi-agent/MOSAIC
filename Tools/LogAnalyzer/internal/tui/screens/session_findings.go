package screens

import (
	"fmt"

	"mosaic-log-analyzer/internal/domain"
)

// The findings list of the Session screen: measured facts about the actor, one
// line each. Every line states a figure; none names a cause.

// findingsTitle heads the findings list.
const findingsTitle = "Findings"

// sessionFindingLines are the findings of the actor, headed by a title. An actor
// without findings, or without figures in the report, has no lines.
func sessionFindingLines(st Styles, width int, run domain.RunReport, actor domain.ActorRef) []string {
	session, ok := run.ActorSession(actor)
	if !ok || len(session.Findings) == 0 {
		return nil
	}
	lines := []string{clip(st.Subtitle, width, findingsTitle)}
	for _, f := range session.Findings {
		if text := findingText(st, f, actor); text != "" {
			lines = append(lines, clip(st.Body, width, "  "+text))
		}
	}
	return lines
}

// findingText is the line of one finding.
func findingText(st Styles, f domain.ActorFinding, actor domain.ActorRef) string {
	cur := st.Currency
	switch f.Kind {
	case domain.ActorFindingCostliestCall:
		pct := sharePercentText(f.Share.Numerator, f.Share.Denominator, f.Share.Denominator > 0)
		return fmt.Sprintf("costliest call #%d: %s (%s of this agent)", f.Call+1, FormatMoney(cur, f.Cost), pct)
	case domain.ActorFindingRewrites:
		return fmt.Sprintf("context rewrites: %d, extra cost %s", f.Count, FormatMoney(cur, f.Cost))
	case domain.ActorFindingCompactions:
		return fmt.Sprintf("compactions: %d (%d unapplied), compaction cost %s", f.Count, f.Unapplied, FormatMoney(cur, f.Cost))
	case domain.ActorFindingFailedTool:
		return fmt.Sprintf("failed tool %s: %d errors", toolName(domain.ToolFigures{Tool: f.Tool}), f.Count)
	case domain.ActorFindingPeakContextVsType:
		return fmt.Sprintf("peak context %s vs %s average %s (%d agents)",
			FormatTokens(f.Tokens), typeLabel(actor.Type), FormatTokens(f.TypeAverage), f.TypeMembers)
	case domain.ActorFindingCostVsType:
		return fmt.Sprintf("cost %s vs %s average %s (%d agents)",
			FormatMoney(cur, f.Cost), typeLabel(actor.Type), FormatMoney(cur, f.TypeCost), f.TypeMembers)
	}
	return ""
}
