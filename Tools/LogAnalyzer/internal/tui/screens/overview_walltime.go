package screens

import (
	"fmt"
	"strings"
	"time"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The wall-time block of the Overview: the split of the run's logged span, the
// tools that counted as waits, the open-ended waits and the limits of the
// waiting figure.

// wallShare is one part of the wall-time split.
type wallShare struct {
	name string
	time time.Duration
}

func wallShares(w domain.WallTimeSplit) []wallShare {
	return []wallShare{
		{"waiting on the user", w.Waiting},
		{"subagents working", w.Subagents},
		{"orchestrator only", w.OrchestratorOnly},
		{"between sessions", w.BetweenSessions},
	}
}

// wallTimeLines is the wall-time block. A run without a logged span shows the
// absent marker in place of every duration.
func (s *OverviewScreen) wallTimeLines() []string {
	st, width := s.styles, s.width
	w := s.run.WallTime
	title := "Wall time  " + FormatDuration(w.Span.End.Sub(w.Span.Start), w.HasSpan)
	out := []string{heading(st, width, title)}
	if w.HasSpan {
		out = append(out, s.wallSplitLines(wallShares(w))...)
		out = append(out, s.countedToolLines(w.CountedTools)...)
	}
	out = append(out, clip(st.Body, width, fmt.Sprintf("Open-ended waits: %d (not counted in the waiting figure)", len(w.OpenWaits))))
	for _, l := range strings.Split(tuicommon.Wrap(domain.PermissionPairingNote, width), "\n") {
		out = append(out, clip(st.Muted, width, l))
	}
	if w.MissingContactToolsHint {
		hint := "MCP contact tools may be missing: see the Tools view (key " + KeyViewTools + ")"
		for _, l := range strings.Split(tuicommon.Wrap(hint, width), "\n") {
			out = append(out, clip(st.Warning, width, l))
		}
	}
	return out
}

// wallSplitLines is the split: a stacked bar in the visual version, a table in
// the figures-only one.
func (s *OverviewScreen) wallSplitLines(shares []wallShare) []string {
	if s.figuresOnly {
		return figureTable(wallColumns(), wallRows(shares), s.width, s.styles)
	}
	segs := make([]chart.Segment, len(shares))
	for i, sh := range shares {
		segs[i] = chart.Segment{Label: sh.name + " " + FormatDuration(sh.time, true), Value: float64(sh.time),
			Role: tuicommon.RoleDataTime}
	}
	return stackedLines(segs, s.width, s.styles)
}

func wallColumns() []table.Column {
	right := table.AlignRight
	return []table.Column{
		{Title: "category", Flex: true, MinWidth: 12},
		{Title: "time", Align: right},
		{Title: "share", Align: right},
	}
}

func wallRows(shares []wallShare) [][]table.Cell {
	var total time.Duration
	for _, sh := range shares {
		total += sh.time
	}
	rows := make([][]table.Cell, len(shares))
	for i, sh := range shares {
		rows[i] = []table.Cell{{Text: sh.name}, durationCell(sh.time, true),
			ratioCell(domain.Ratio{Numerator: int64(sh.time), Denominator: int64(total)})}
	}
	return rows
}

// countedToolLines lists the tool names that counted as waits with their calls.
func (s *OverviewScreen) countedToolLines(tools []domain.CountedTool) []string {
	st, width := s.styles, s.width
	if len(tools) == 0 {
		return []string{clip(st.Muted, width, "Tools counted as waits: none")}
	}
	rows := make([][]table.Cell, len(tools))
	for i, t := range tools {
		rows[i] = []table.Cell{{Text: t.Tool, Role: tuicommon.RoleDataTool}, {Text: string(t.Harness)},
			{Text: formatGrouped(int64(t.Calls))}}
	}
	cols := []table.Column{
		{Title: "tools counted as waits", Flex: true, MinWidth: 12},
		{Title: "harness", Priority: 1},
		{Title: "calls", Align: table.AlignRight},
	}
	return figureTable(cols, rows, width, st)
}
