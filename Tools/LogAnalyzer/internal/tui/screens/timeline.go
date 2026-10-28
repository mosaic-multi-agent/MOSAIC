package screens

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// timeLayout renders a logged timestamp as it was written, without any
// correction for clock skew between streams.
const timeLayout = "15:04:05"

// TimelineScreen is the timeline view (V5). Its default version is a Gantt
// chart: one lane per invocation under a shared time axis, an orchestrator lane
// with the compactions, and a detail strip for the lane under the cursor. Its
// list version is a table of the invocations followed by the compactions. It
// shows timestamps only, never money lanes.
type TimelineScreen struct {
	run    domain.RunReport
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
	runChrome

	list    bool         // list version instead of the Gantt
	grouped bool         // Gantt lanes grouped by agent type
	cursor  int          // selected lane of the Gantt
	offset  int          // first visible lane of the Gantt
	scroll  pageScroller // scroll position of the list version
	sort    SortCycle    // sort of the list version's invocation table
}

// NewTimelineScreen creates the timeline screen for one run.
func NewTimelineScreen(run domain.RunReport, width, height int, styles Styles) *TimelineScreen {
	return &TimelineScreen{run: run, width: width, height: height, styles: styles,
		nav: viewNav{self: ScreenTimeline}, sort: newTimelineSortCycle()}
}

// ID implements Screen.
func (s *TimelineScreen) ID() ScreenID { return ScreenTimeline }

// Init implements Screen.
func (s *TimelineScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *TimelineScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
		s.move(k)
	case KeyToggleView:
		s.list = !s.list
		s.scroll = pageScroller{}
	case KeyGroup:
		if !s.list {
			s.grouped = !s.grouped
			s.cursor, s.offset = 0, 0
		}
	case KeySort:
		if s.list {
			s.sort = s.sort.Next()
		}
	default:
		s.nav.request(k)
	}
	return s, nil
}

// move moves the Gantt cursor by one lane, or scrolls the list version.
func (s *TimelineScreen) move(k string) {
	if s.list {
		s.scroll.handle(k, len(s.listBody()), s.height)
		return
	}
	delta := 1
	if k == "up" || k == "k" {
		delta = -1
	}
	s.cursor = clampIndex(s.cursor+delta, len(s.run.Timeline.Entries))
}

// View implements Screen.
func (s *TimelineScreen) View(width, height int) string {
	s.width, s.height = width, height
	spec := frameSpec{run: s.run, active: ScreenTimeline, keys: s.helpKeys(), screenKeys: s.screenKeys(),
		escDesc: "back to the runs list"}
	var body []string
	if s.list {
		all := s.listBody()
		w := s.scroll.window(len(all), height)
		spec.hint, spec.sort = FormatScrollHint(w, len(all)), s.sort.Label()
		body = all[w.Offset:w.End()]
	} else {
		body, spec.hint = s.ganttBody(width, height)
	}
	return s.render(spec, body, width, height, s.styles)
}

// helpKeys are the short help keys of the current version.
func (s *TimelineScreen) helpKeys() []tuicommon.HelpEntry {
	if s.list {
		return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "scroll"}, {Key: KeySort, Desc: "sort"},
			{Key: KeyToggleView, Desc: "gantt"}, {Key: "esc", Desc: "back"}}
	}
	group := "group"
	if s.grouped {
		group = "order"
	}
	return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: KeyGroup, Desc: group},
		{Key: KeyToggleView, Desc: "list"}, {Key: "esc", Desc: "back"}}
}

// screenKeys are the overlay entries of the keys this view has of its own.
func (s *TimelineScreen) screenKeys() []tuicommon.HelpEntry {
	toggle := tuicommon.HelpEntry{Key: KeyToggleView, Desc: "switch between the Gantt and the list version"}
	if s.list {
		return []tuicommon.HelpEntry{{Key: KeySort, Desc: "cycle the sort order of the invocation table"}, toggle}
	}
	return []tuicommon.HelpEntry{
		{Key: KeyGroup, Desc: "group the lanes by agent type, or back to dispatch order"}, toggle}
}

// formatClock renders a logged time, or AbsentMarker when it is unknown.
func formatClock(t time.Time) string {
	if t.IsZero() {
		return AbsentMarker
	}
	return t.Format(timeLayout)
}

// Help implements Screen.
func (s *TimelineScreen) Help() string {
	return shortHelpText("", s.helpKeys())
}

// Back reports whether the user asked to leave the per-run views.
func (s *TimelineScreen) Back() bool { return s.back }

// Requested reports a view key pressed for a different per-run view.
func (s *TimelineScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *TimelineScreen) Reset() {
	s.back = false
	s.nav.reset()
}
