package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// runsTitle is the title of the runs list page.
const runsTitle = "Log Analyzer — Runs"

// runsKeys are the short help keys of the runs list.
var runsKeys = []tuicommon.HelpEntry{
	{Key: "↑/↓", Desc: "navigate"}, {Key: "enter", Desc: "open"}, {Key: KeySort, Desc: "sort"},
	{Key: KeyMarkRun, Desc: "mark run"}, {Key: KeyCompare, Desc: "compare marked"},
	{Key: KeyAcrossRuns, Desc: "across runs"}, {Key: KeyExportRun, Desc: "export run"}, {Key: KeyExportAll, Desc: "export all"},
	{Key: "s", Desc: "source"}, {Key: "p", Desc: "pricing"}, {Key: "esc", Desc: "quit"}, {Key: "ctrl+c", Desc: "quit"},
}

// runsOverlayGroups are the keys listed by the help overlay of the runs list.
func runsOverlayGroups() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		{Title: "This screen", Entries: []tuicommon.HelpEntry{
			sortKeyEntry,
			{Key: KeyAcrossRuns, Desc: "open the across-runs view"},
			{Key: KeyMarkRun, Desc: "mark or unmark the selected run for a comparison (A, then B)"},
			{Key: KeyCompare, Desc: "compare the two marked runs"},
			{Key: "s", Desc: "choose another log source"},
			{Key: "p", Desc: "enter model prices"},
		}},
		{Title: "Export", Entries: []tuicommon.HelpEntry{
			{Key: KeyExportRun, Desc: "export the selected run as a digest file"},
			{Key: KeyExportAll, Desc: "export all runs as a digest file"},
		}},
		{Title: "Navigation", Entries: []tuicommon.HelpEntry{
			{Key: "↑/↓ j/k", Desc: "move the selection"},
			{Key: "enter", Desc: "open the selected run"},
			{Key: "esc", Desc: "quit"},
			{Key: "q", Desc: "same as esc"},
			{Key: KeyHelp, Desc: "show or hide this help"},
			{Key: "ctrl+c", Desc: "quit"},
		}},
	}
}

// Columns of the runs table, in display order.
const (
	rcMark = iota
	rcRun
	rcCost
	rcInvocations
	rcDuration
	rcMarkers
)

// runsColumns lists the columns with the priority at which each is dropped on a
// narrow terminal: the run and its cost stay.
func runsColumns() []table.Column {
	return []table.Column{
		rcMark:        {Title: "cmp"},
		rcRun:         {Title: "run", Flex: true, MinWidth: 12},
		rcCost:        {Title: "cost", Align: table.AlignRight},
		rcInvocations: {Title: "invocations", Align: table.AlignRight, Priority: 1},
		rcDuration:    {Title: "duration", Align: table.AlignRight, Priority: 2},
		rcMarkers:     {Title: "markers", Priority: 1},
	}
}

// RunsScreen lists the named runs as a table with their cost, invocations and
// duration, under the all-runs totals. Provisional runs are marked. The
// unattributable bucket is in its own section below the table, excluded from the
// named-run total.
//
// Navigation contract:
//   - ↑/↓  navigate runs.
//   - o     cycle the sort order.
//   - Enter -> Done() == true, SelectedRunID() returns the chosen run id.
//   - Esc   -> Back() == true.
type RunsScreen struct {
	report domain.Report
	sort   SortCycle
	cursor int // row of the sorted list
	offset int // scroll offset, clamped inside View
	width  int
	height int
	styles Styles
	done   bool
	back   bool
	marks  runMarks
	pageChrome
}

// NewRunsScreen creates the run-list screen for the given report.
func NewRunsScreen(report domain.Report, width, height int, styles Styles) *RunsScreen {
	return &RunsScreen{
		report: report,
		sort:   NewSortCycle(SortOrder{SortCost, true}, SortOrder{SortInvocations, true}, SortOrder{SortDuration, true}, SortOrder{SortName, false}),
		width:  width,
		height: height,
		styles: styles,
	}
}

// SetReport replaces the report (used after a reprice without re-navigating).
func (s *RunsScreen) SetReport(r domain.Report) {
	s.report = r
	s.done = false
	s.back = false
	s.marks.clear()
	s.cursor = clampIndex(s.cursor, len(r.Runs))
}

// ID implements Screen.
func (s *RunsScreen) ID() ScreenID { return ScreenRuns }

// Init implements Screen.
func (s *RunsScreen) Init() tea.Cmd { return nil }

// Update processes key messages for navigation and sorting.
func (s *RunsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	k := key.String()
	if s.help.handle(k) {
		return s, nil
	}
	n := len(s.report.Runs)
	s.marks.status = ""
	switch k {
	case "up", "k":
		s.cursor = clampIndex(s.cursor-1, n)
	case "down", "j":
		s.cursor = clampIndex(s.cursor+1, n)
	case KeySort:
		s.sort = s.sort.Next()
	case KeyMarkRun:
		s.toggleMark()
	case KeyCompare:
		s.marks.request()
	case "enter":
		if n > 0 {
			s.done = true
		}
	case "esc", "q":
		s.back = true
	}
	return s, nil
}

// order returns the indexes of the runs in the order of the active sort.
func (s *RunsScreen) order() []int {
	runs := s.report.Runs
	return sortedOrder(len(runs), s.sort.Active(),
		func(i int) string { return runs[i].Run.Label() },
		func(key SortKey, i int) (int64, bool) {
			r := runs[i]
			switch key {
			case SortCost:
				return moneyNanos(r.Totals.Money.Total)
			case SortInvocations:
				return int64(r.Overview.Subagents.Invocations), true
			case SortDuration:
				return int64(runDuration(r)), r.Timeline.HasAxis
			}
			return 0, false
		})
}

// runDuration is the length of the run's timeline axis.
func runDuration(r domain.RunReport) time.Duration {
	return r.Timeline.Axis.End.Sub(r.Timeline.Axis.Start)
}

// runsSortColumn is the column that shows the sort key.
func runsSortColumn(key SortKey) int {
	switch key {
	case SortCost:
		return rcCost
	case SortInvocations:
		return rcInvocations
	case SortDuration:
		return rcDuration
	case SortName:
		return rcRun
	}
	return table.NoSort
}

// runCells builds the cells of one run row.
func runCells(r domain.RunReport, marker string) []table.Cell {
	dur := durationCell(runDuration(r), r.Timeline.HasAxis)
	var markers []string
	if r.Provisional {
		markers = append(markers, ProvisionalLabel)
	}
	if len(r.UnpricedModels) > 0 {
		markers = append(markers, UnpricedMarker+" models")
	}
	if a := FormatAssumedTier(r.Totals.AssumedTierCacheWrite); a != "" {
		markers = append(markers, a)
	}
	return []table.Cell{
		rcMark:        markCell(marker),
		rcRun:         {Text: r.Run.Label(), Role: tuicommon.RoleSubtitle},
		rcCost:        moneyTotalCell(r.Totals.Money),
		rcInvocations: {Text: formatGrouped(int64(r.Overview.Subagents.Invocations))},
		rcDuration:    dur,
		rcMarkers:     warningCell(strings.Join(markers, "; ")),
	}
}

// View renders the run-list page, clipped to the given height. The scroll offset
// is clamped against the passed-in height on every render so that terminal
// resizes are handled implicitly on the next call.
func (s *RunsScreen) View(width, height int) string {
	s.width, s.height = width, height
	st := s.styles
	n := len(s.report.Runs)

	head := []string{st.Subtitle.Render("All runs") + st.Body.Render("  "+FormatTotalsLine(s.report.AllRuns))}
	tail := s.tailLines()

	var body []string
	var w Window
	order := s.sort.Active()
	if n == 0 {
		body = append(append(head, st.Muted.Render("(no named runs)")), tail...)
	} else {
		sorted := s.order()
		rows := make([][]table.Cell, n)
		for i, idx := range sorted {
			run := s.report.Runs[idx]
			rows[i] = runCells(run, s.marks.markerOf(run.Run.ID))
		}
		budget := PageBodyHeight(height) - len(head) - len(tail) - 1
		w = ScrollWindow(s.offset, s.cursor, n, budget)
		s.offset = w.Offset
		t := table.Table{Columns: runsColumns(), Rows: rows, Cursor: s.cursor, Offset: w.Offset,
			SortColumn: runsSortColumn(order.Key), SortDesc: order.Desc}
		body = append(append(head, renderTableLines(t, width, w.Count, st)...), tail...)
	}

	page := Page{Title: runsTitle, Badges: s.badges(), Keys: runsKeys, Help: true, Hint: FormatScrollHint(w, n)}
	return s.render(page, runsOverlayGroups(), body, width, height, st)
}

// badges are the page header badges: the count of named runs, the findings and
// the active sort.
func (s *RunsScreen) badges() []PageBadge {
	badges := []PageBadge{{Text: countLabel(len(s.report.Runs), "run"), Role: tuicommon.RoleDataActor}}
	if f := len(s.report.Quality.Findings); f > 0 {
		role := tuicommon.RoleSignalWarning
		if s.report.Quality.Incomplete() {
			role = tuicommon.RoleSignalBad
		}
		badges = append(badges, PageBadge{Text: countLabel(f, "finding"), Role: role})
	}
	return append(badges, PageBadge{Text: s.sort.Label(), Role: tuicommon.RoleSignalMuted})
}

// tailLines are the text lines below the table: the unattributable section and the
// notices about unpriced models and assumed cache-write tiers.
func (s *RunsScreen) tailLines() []string {
	st := s.styles
	var lines []string
	if s.marks.status != "" {
		lines = append(lines, st.Warning.Render(s.marks.status))
	}
	if u := s.report.Unattributable; u != nil {
		lines = append(lines, st.Warning.Render(UnattributableLabel)+st.Body.Render("  "+FormatTotalsLine(u.Totals)))
	}
	if len(s.report.UnpricedModels) > 0 {
		lines = append(lines, st.Warning.Render(fmt.Sprintf(
			"Some models have no pricing (%d model(s) unpriced). Press 'p' to enter prices.", len(s.report.UnpricedModels))))
	}
	if assumed := s.assumedTierTokens(); assumed > 0 {
		lines = append(lines, st.Warning.Render(AssumedTierNotice(assumed)))
	}
	return lines
}

// Help returns the key hints for this screen.
func (s *RunsScreen) Help() string { return shortHelpText("", runsKeys) }

// Done reports whether the user selected a run.
func (s *RunsScreen) Done() bool { return s.done }

// Back reports whether the user pressed Esc.
func (s *RunsScreen) Back() bool { return s.back }

// SelectedRunID returns the id of the currently highlighted run.
// Only meaningful when Done() is true.
func (s *RunsScreen) SelectedRunID() string {
	if len(s.report.Runs) == 0 {
		return ""
	}
	order := s.order()
	return s.report.Runs[order[clampIndex(s.cursor, len(order))]].Run.ID
}

// Reset clears the done and back flags.
func (s *RunsScreen) Reset() {
	s.done = false
	s.back = false
	s.marks.requested = false
}

// assumedTierTokens sums the assumed-tier cache-write tokens of the named runs
// and the unattributable bucket.
func (s *RunsScreen) assumedTierTokens() int64 {
	total := s.report.AllRuns.AssumedTierCacheWrite
	if s.report.Unattributable != nil {
		total += s.report.Unattributable.Totals.AssumedTierCacheWrite
	}
	return total
}
