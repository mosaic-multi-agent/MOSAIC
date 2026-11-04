package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// OverviewScreen is the run overview view (V1): cost share, token split, cache,
// models, outcome of the dispatches and wall time. It has a visual version
// (stacked bars and gauges, the default) and a figures-only version.
type OverviewScreen struct {
	run    domain.RunReport
	scroll pageScroller
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
	runChrome

	figuresOnly bool
	modelSort   SortCycle
}

// NewOverviewScreen creates the overview screen for one run.
func NewOverviewScreen(run domain.RunReport, width, height int, styles Styles) *OverviewScreen {
	return &OverviewScreen{run: run, width: width, height: height, styles: styles,
		nav: viewNav{self: ScreenOverview}, modelSort: newModelSortCycle()}
}

// ID implements Screen.
func (s *OverviewScreen) ID() ScreenID { return ScreenOverview }

// Init implements Screen.
func (s *OverviewScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *OverviewScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
	case "up", "k", "down", "j":
		s.scroll.handle(k, len(s.body()), s.height)
	case KeySort:
		s.modelSort = s.modelSort.Next()
	case KeyToggleView:
		s.figuresOnly = !s.figuresOnly
		s.scroll = pageScroller{}
	default:
		s.nav.request(k)
	}
	return s, nil
}

// View implements Screen.
func (s *OverviewScreen) View(width, height int) string {
	s.width, s.height = width, height
	body := s.body()
	w := s.scroll.window(len(body), height)
	spec := frameSpec{run: s.run, active: ScreenOverview, keys: s.helpKeys(), screenKeys: s.screenKeys(),
		legend: overviewLegend(), escDesc: "back to the runs list", hint: FormatScrollHint(w, len(body)), sort: s.modelSort.Label()}
	return s.render(spec, body[w.Offset:w.End()], width, height, s.styles)
}

// body renders every line of the overview; the caller scrolls through it.
func (s *OverviewScreen) body() []string {
	sections := [][]string{s.summaryLines()}
	if !s.figuresOnly {
		sections = append(sections, s.costShareLines())
	}
	sections = append(sections, s.tokenLines(), s.cacheLines(), s.outcomeLines(), s.wallTimeLines(), s.modelLines())
	var out []string
	for _, sec := range sections {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, sec...)
	}
	return out
}

// helpKeys are the short help keys of the view.
func (s *OverviewScreen) helpKeys() []tuicommon.HelpEntry {
	return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "scroll"}, {Key: KeySort, Desc: "sort"},
		{Key: KeyToggleView, Desc: s.toggleDesc()}, exportKeyEntry, {Key: "esc", Desc: "back"}}
}

// screenKeys are the overlay entries of the keys this view has of its own.
func (s *OverviewScreen) screenKeys() []tuicommon.HelpEntry {
	return []tuicommon.HelpEntry{
		{Key: KeySort, Desc: "cycle the sort order of the by-model table"},
		{Key: KeyToggleView, Desc: "switch between the visual and the figures-only version"},
	}
}

// toggleDesc names the version the toggle key switches to.
func (s *OverviewScreen) toggleDesc() string {
	if s.figuresOnly {
		return "visual"
	}
	return "figures"
}

// Help implements Screen.
func (s *OverviewScreen) Help() string {
	return shortHelpText("", s.helpKeys())
}

// Back reports whether the user asked to leave the per-run views.
func (s *OverviewScreen) Back() bool { return s.back }

// Requested reports a view key pressed for a different per-run view.
func (s *OverviewScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *OverviewScreen) Reset() {
	s.back = false
	s.nav.reset()
}
