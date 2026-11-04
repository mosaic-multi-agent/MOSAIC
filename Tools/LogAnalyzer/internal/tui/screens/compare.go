package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"

	"mosaic-log-analyzer/internal/domain"
)

// compareTitle is the title of the comparison page.
const compareTitle = "Log Analyzer — Compare"

// compareKeys are the short help keys of the comparison.
var compareKeys = []tuicommon.HelpEntry{
	{Key: "↑/↓", Desc: "scroll"}, {Key: KeySort, Desc: "sort"}, {Key: "esc", Desc: "back"}, {Key: "ctrl+c", Desc: "quit"},
}

// compareOverlayGroups are the keys listed by the help overlay of the comparison.
func compareOverlayGroups() []chrome.HelpGroup {
	return []chrome.HelpGroup{
		{Title: "This screen", Entries: []tuicommon.HelpEntry{sortKeyEntry}},
		{Title: "Navigation", Entries: []tuicommon.HelpEntry{
			{Key: "↑/↓ j/k", Desc: "scroll"},
			{Key: "esc", Desc: "back to the runs list"},
			{Key: "q", Desc: "same as esc"},
			{Key: KeyHelp, Desc: "show or hide this help"},
			{Key: "ctrl+c", Desc: "quit"},
		}},
	}
}

// compareChromeRows are the table lines that are not scrollable rows: the table
// header and the totals row.
const compareChromeRows = 2

// CompareScreen renders the comparison of two runs: a table per agent type and a
// totals row, each with run A, run B and the change between them. The figures
// come from the comparison; the screen only renders them.
type CompareScreen struct {
	cmp    domain.RunComparison
	sort   SortCycle
	offset int // first row shown, clamped to the rows that exist
	width  int
	height int
	styles Styles
	back   bool
	pageChrome
}

// NewCompareScreen creates the comparison screen for cmp.
func NewCompareScreen(cmp domain.RunComparison, width, height int, st Styles) *CompareScreen {
	return &CompareScreen{cmp: cmp, sort: newCompareSortCycle(), width: width, height: height, styles: st}
}

// ID implements Screen.
func (s *CompareScreen) ID() ScreenID { return ScreenCompare }

// Init implements Screen.
func (s *CompareScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *CompareScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	k := key.String()
	if s.help.handle(k) {
		return s, nil
	}
	switch k {
	case "esc", "q":
		s.back = true
	case KeySort:
		s.sort = s.sort.Next()
	case "up", "k":
		s.scrollBy(-1)
	case "down", "j":
		s.scrollBy(1)
	}
	return s, nil
}

// scrollBy moves the first shown row, keeping it inside the rows that exist.
func (s *CompareScreen) scrollBy(delta int) {
	s.window(s.offset + delta)
}

// window clamps the first shown row to the rows that fit the current height.
func (s *CompareScreen) window(offset int) Window {
	budget := PageBodyHeight(s.height) - compareChromeRows - compareExtraHeaderLines(s.cmp, s.width, s.styles)
	w := ScrollWindow(offset, -1, len(s.cmp.Rows), budget)
	s.offset = w.Offset
	return w
}

// View implements Screen.
func (s *CompareScreen) View(width, height int) string {
	s.width, s.height = width, height
	w := s.window(s.offset)
	o := s.sort.Active()
	body := compareTableLines(s.cmp, compareOrder(s.cmp.Rows, o), w, o, width, s.styles)
	page := Page{Title: compareTitle, Badges: s.badges(), Keys: compareKeys, Help: true,
		Hint: FormatScrollHint(w, len(s.cmp.Rows))}
	return s.render(page, withLegend(compareOverlayGroups(), compareLegend()...), body, width, height, s.styles)
}

// badges are the header badges: both runs, a run still in progress marked next
// to its label, and the active sort.
func (s *CompareScreen) badges() []PageBadge {
	return []PageBadge{
		runBadge("A", s.cmp.A, s.cmp.ProvisionalA),
		runBadge("B", s.cmp.B, s.cmp.ProvisionalB),
		{Text: s.sort.Label(), Role: tuicommon.RoleSignalMuted},
	}
}

// runBadge names a compared run by its side and label.
func runBadge(side string, run domain.RunRef, provisional bool) PageBadge {
	if provisional {
		return PageBadge{Text: side + " " + run.Label() + " " + ProvisionalLabel, Role: tuicommon.RoleSignalWarning}
	}
	return PageBadge{Text: side + " " + run.Label(), Role: tuicommon.RoleDataActor}
}

// Help implements Screen.
func (s *CompareScreen) Help() string { return shortHelpText("", compareKeys) }

// Back reports whether the user asked to return to the runs list.
func (s *CompareScreen) Back() bool { return s.back }

// Requested implements RunView; the comparison has no view keys.
func (s *CompareScreen) Requested() (ScreenID, bool) { return "", false }

// Reset clears Back.
func (s *CompareScreen) Reset() { s.back = false }
