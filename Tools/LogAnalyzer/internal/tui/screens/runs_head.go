package screens

import (
	"fmt"
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
)

// Labels of the runs-list head lines, padded to one column.
const (
	headLabelAll    = "All runs"
	headLabelPerRun = "Per run"
	headLabelWidth  = len(headLabelAll) + 2
)

// runsHeadSeparators are the item separator and the date range separator in the
// active render mode.
func runsHeadSeparators() (item, dateRange string) {
	if tuicommon.CurrentRenderMode().Glyphs == tuicommon.GlyphASCII {
		return " - ", " - "
	}
	return " · ", " – "
}

// runsHeadLines renders the three head lines of a runs list that has runs.
func runsHeadLines(st Styles, r domain.Report, width int) []string {
	stats := RunsHeadStatsOf(r)
	line := func(label, text string) string {
		return st.Subtitle.Render(textwidth.Pad(label, headLabelWidth, false)) + st.Body.Render(text)
	}
	totals := runsHeadTotalsText(st.Currency, stats, true)
	if width > 0 && headLabelWidth+textwidth.Of(totals) > width {
		totals = runsHeadTotalsText(st.Currency, stats, false) // the period goes first when the line is too wide
	}
	return []string{
		line(headLabelAll, totals),
		line(headLabelPerRun, "cost  "+runsHeadCostText(st.Currency, stats)),
		line("", "time  "+runsHeadTimeText(stats)),
	}
}

// runsHeadTotalsText is the first line: run count, period, total cost and tokens.
func runsHeadTotalsText(cur domain.CurrencyCode, s RunsHeadStats, withRange bool) string {
	sep, rangeSep := runsHeadSeparators()
	count := countLabel(s.Runs, "run")
	if s.InProgress > 0 {
		count += fmt.Sprintf(" (%d in progress)", s.InProgress)
	}
	items := []string{count}
	if s.HasRange && withRange {
		const dateLayout = "Jan 2"
		items = append(items, s.First.UTC().Format(dateLayout)+rangeSep+s.Last.UTC().Format(dateLayout))
	}
	items = append(items, "total "+FormatTotal(cur, s.TotalCost), FormatTokens(s.Tokens)+" tokens")
	return strings.Join(items, sep)
}

// runsHeadCostText is the cost statistics of the second line.
func runsHeadCostText(cur domain.CurrencyCode, s RunsHeadStats) string {
	sep, _ := runsHeadSeparators()
	c := s.Cost
	if c.Count == 0 {
		return "no priced runs" + runsHeadCoverage(c.Count, s.Runs)
	}
	money := func(m domain.Money) string { return FormatMoney(cur, domain.KnownMoney(m)) }
	items := []string{"avg " + money(c.Avg), "median " + money(c.Median)}
	if c.HasP90 {
		items = append(items, "p90 "+money(c.P90))
	}
	return strings.Join(items, sep) + runsHeadCoverage(c.Count, s.Runs)
}

// runsHeadTimeText is the duration statistics of the third line.
func runsHeadTimeText(s RunsHeadStats) string {
	sep, _ := runsHeadSeparators()
	t := s.Time
	if t.Count == 0 {
		return "no timed runs" + runsHeadCoverage(t.Count, s.Runs)
	}
	items := []string{"avg " + FormatDuration(t.Avg, true), "median " + FormatDuration(t.Median, true)}
	return strings.Join(items, sep) + runsHeadCoverage(t.Count, s.Runs)
}

// runsHeadCoverage states how many of the runs a statistic covers; empty when it
// covers all of them.
func runsHeadCoverage(counted, runs int) string {
	if counted >= runs {
		return ""
	}
	return fmt.Sprintf("   (over %d of %d runs)", counted, runs)
}
