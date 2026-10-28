package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// dispatchKeys are the short help keys of the Dispatches view.
var dispatchKeys = []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: KeySort, Desc: "sort"}, {Key: "esc", Desc: "back"}}

// RunDetailScreen is the Dispatches view of one run: a table with the orchestrator
// row first and one row per dispatch with its cost, share of the run, status,
// context and cache figures.
//
// Navigation contract:
//   - ↑/↓  move the selection through the rows.
//   - o     cycle the sort order of the dispatch rows.
//   - Esc   -> Back() == true.
type RunDetailScreen struct {
	run    domain.RunReport
	cursor int
	offset int
	sort   SortCycle
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
	runChrome
}

// NewRunDetailScreen creates the run-detail screen for the given run report.
func NewRunDetailScreen(run domain.RunReport, width, height int, styles Styles) *RunDetailScreen {
	return &RunDetailScreen{
		run:    run,
		sort:   newDispatchSortCycle(),
		width:  width,
		height: height,
		styles: styles,
		nav:    viewNav{self: ScreenRunDetail},
	}
}

// ID implements Screen.
func (s *RunDetailScreen) ID() ScreenID { return ScreenRunDetail }

// Init implements Screen.
func (s *RunDetailScreen) Init() tea.Cmd { return nil }

// Update processes key messages for navigation and sorting.
func (s *RunDetailScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	k := key.String()
	if s.help.handle(k) {
		return s, nil
	}
	rows := 1 + len(dispatchEntries(s.run.ByAgentType))
	switch k {
	case "up", "k":
		s.cursor = clampIndex(s.cursor-1, rows)
	case "down", "j":
		s.cursor = clampIndex(s.cursor+1, rows)
	case KeySort:
		s.sort = s.sort.Next()
	case "esc", "q":
		s.back = true
	default:
		s.nav.request(k)
	}
	return s, nil
}

// View renders the Dispatches table, clipped to the given height. The scroll
// offset is settled against the passed-in height on every render so that
// terminal resizes are handled on the next call.
func (s *RunDetailScreen) View(width, height int) string {
	s.width, s.height = width, height
	entries := dispatchEntries(s.run.ByAgentType)
	order := s.sort.Active()
	rows := dispatchTableRows(s.run, true, entries, dispatchOrder(entries, order), s.styles.Theme)

	head := s.summaryLines()
	lines, w := dispatchTableLines(rows, s.cursor, &s.offset, order, width, bodyBudget(height)-len(head)-1, s.styles)

	spec := frameSpec{run: s.run, active: ScreenRunDetail, keys: dispatchKeys, screenKeys: []tuicommon.HelpEntry{sortKeyEntry},
		escDesc: "back to the runs list", hint: FormatScrollHint(w, len(rows)), sort: s.sort.Label()}
	return s.render(spec, append(head, lines...), width, height, s.styles)
}

// summaryLines are the text lines above the table: the run totals, then the
// notices about unpriced models and assumed cache-write tiers.
func (s *RunDetailScreen) summaryLines() []string {
	st := s.styles
	lines := []string{st.Subtitle.Render("Run totals") + st.Body.Render("  "+FormatTotalsLine(s.run.Totals))}
	if len(s.run.UnpricedModels) > 0 {
		models := make([]string, len(s.run.UnpricedModels))
		for i, m := range s.run.UnpricedModels {
			models[i] = m.String()
		}
		lines = append(lines, st.Warning.Render("Unpriced models: "+strings.Join(models, ", ")))
	}
	if assumed := s.run.Totals.AssumedTierCacheWrite; assumed > 0 {
		lines = append(lines, st.Warning.Render(AssumedTierNotice(assumed)))
	}
	return lines
}

// Help returns the key hints for this screen.
func (s *RunDetailScreen) Help() string { return shortHelpText("", dispatchKeys) }

// Back reports whether the user pressed Esc.
func (s *RunDetailScreen) Back() bool { return s.back }

// Reset clears the back flag.
func (s *RunDetailScreen) Reset() {
	s.back = false
	s.nav.reset()
}

// Requested reports a view key pressed for a different per-run view.
func (s *RunDetailScreen) Requested() (ScreenID, bool) { return s.nav.requested() }
