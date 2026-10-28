package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"

	"mosaic-log-analyzer/internal/domain"
)

// acrossTitle is the title of the across-runs page.
const acrossTitle = "Log Analyzer — Across Runs"

// acrossKeys are the short help keys of the across-runs view.
var acrossKeys = []tuicommon.HelpEntry{
	{Key: "↑/↓", Desc: "scroll"}, {Key: KeySort, Desc: "sort"}, {Key: "esc", Desc: "back"}, {Key: "ctrl+c", Desc: "quit"},
}

// acrossOverlayGroups are the keys listed by the help overlay of the view.
func acrossOverlayGroups() []chrome.HelpGroup {
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

// AcrossRunsScreen shows the figures across all named runs (V6): the runs side
// by side with their totals, then each agent type summed over the runs with its
// per-run averages. The unattributable bucket is never shown.
type AcrossRunsScreen struct {
	across domain.AcrossRuns
	scroll pageScroller
	sort   SortCycle
	width  int
	height int
	styles Styles
	back   bool
	pageChrome
}

// NewAcrossRunsScreen builds the across-runs screen for the report.
func NewAcrossRunsScreen(report domain.Report, width, height int, styles Styles) *AcrossRunsScreen {
	return &AcrossRunsScreen{across: report.AcrossRuns, width: width, height: height, styles: styles,
		sort: newAcrossSortCycle()}
}

// ID implements Screen.
func (s *AcrossRunsScreen) ID() ScreenID { return ScreenAcrossRuns }

// Init implements Screen.
func (s *AcrossRunsScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *AcrossRunsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
	case "up", "k", "down", "j":
		// The scroller budgets for the frame chrome; the page chrome is one line shorter.
		s.scroll.handle(k, len(s.body()), s.height+FrameChromeLines-PageChromeLines)
	}
	return s, nil
}

// View implements Screen.
func (s *AcrossRunsScreen) View(width, height int) string {
	s.width, s.height = width, height
	body := s.body()
	w := ScrollWindow(s.scroll.offset, -1, len(body), PageBodyHeight(height))
	s.scroll.offset = w.Offset
	badges := []PageBadge{
		{Text: countLabel(len(s.across.Runs), "run"), Role: tuicommon.RoleDataActor},
		{Text: s.sort.Label(), Role: tuicommon.RoleSignalMuted},
	}
	page := Page{Title: acrossTitle, Badges: badges, Keys: acrossKeys, Help: true, Hint: FormatScrollHint(w, len(body))}
	return s.render(page, acrossOverlayGroups(), body[w.Offset:w.End()], width, height, s.styles)
}

// body renders every line of the view; the caller scrolls through it.
func (s *AcrossRunsScreen) body() []string {
	st, width := s.styles, s.width
	if len(s.across.Runs) == 0 {
		return []string{clip(st.Muted, width, "No named runs.")}
	}
	order := s.sort.Active()
	out := []string{clip(st.Subtitle, width, "Runs")}
	out = append(out, acrossRunLines(s.across.Runs, order, width, st)...)
	out = append(out, "", clip(st.Subtitle, width, "Agent types across runs"))
	if len(s.across.ByAgentType) == 0 {
		return append(out, clip(st.Muted, width, "  No subagent invocations."))
	}
	return append(out, acrossTypeLines(s.across.ByAgentType, order, width, st)...)
}

// Help implements Screen.
func (s *AcrossRunsScreen) Help() string { return shortHelpText("", acrossKeys) }

// Back reports whether the user asked to leave the view.
func (s *AcrossRunsScreen) Back() bool { return s.back }

// Requested implements RunView; the across-runs view has no view keys.
func (s *AcrossRunsScreen) Requested() (ScreenID, bool) { return "", false }

// Reset clears Back.
func (s *AcrossRunsScreen) Reset() { s.back = false }
