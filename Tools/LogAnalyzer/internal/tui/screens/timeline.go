package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
)

// timeLayout renders a logged timestamp as it was written, without any
// correction for clock skew between streams.
const timeLayout = "15:04:05"

// TimelineScreen is the timeline view (V5): the run's subagent invocations in
// dispatch order with logged times, durations and parallel overlaps, followed
// by the compactions with their actor. It shows timestamps only, never money.
type TimelineScreen struct {
	run    domain.RunReport
	scroll pageScroller
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
}

// NewTimelineScreen creates the timeline screen for one run.
func NewTimelineScreen(run domain.RunReport, width, height int, styles Styles) *TimelineScreen {
	return &TimelineScreen{run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenTimeline}}
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
	switch k := key.String(); k {
	case "esc", "q":
		s.back = true
	case "up", "k", "down", "j":
		s.scroll.handle(k, len(s.body()), s.height)
	default:
		s.nav.request(k)
	}
	return s, nil
}

// View implements Screen.
func (s *TimelineScreen) View(width, height int) string {
	s.width, s.height = width, height
	body := s.body()
	w := s.scroll.window(len(body), height)
	return renderPage(s.styles, width, "Log Analyzer — Timeline", runLabel(s.run),
		body[w.Offset:w.End()], w, len(body), s.Help())
}

// body renders every line of the view; the caller scrolls through it.
func (s *TimelineScreen) body() []string {
	st, width := s.styles, s.width
	tl := s.run.Timeline
	if len(tl.Entries) == 0 && len(tl.Compactions) == 0 {
		return []string{clip(st.Body, width, "No invocations or compactions in this run.")}
	}
	var out []string
	out = append(out, clip(st.Subtitle, width, "Invocations"))
	for _, e := range tl.Entries {
		out = append(out, clip(st.Body, width, "  "+timelineRow(e)))
	}
	if len(tl.Compactions) > 0 {
		out = append(out, "", clip(st.Subtitle, width, "Compactions"))
		for _, c := range tl.Compactions {
			out = append(out, clip(st.Body, width, "  "+compactionRow(c)))
		}
	}
	return out
}

// formatClock renders a logged time, or AbsentMarker when it is unknown.
func formatClock(t time.Time) string {
	if t.IsZero() {
		return AbsentMarker
	}
	return t.Format(timeLayout)
}

// timelineRow renders one invocation: id, type, logged start and end (open when
// there is no end event), duration and the invocations it ran in parallel with.
func timelineRow(e domain.TimelineEntry) string {
	inv := e.Invocation
	typ := inv.Actor.Type
	if typ == "" {
		typ = UnknownTypeLabel
	}
	end := formatClock(inv.EndedAt)
	if inv.State == domain.InvocationOpen {
		end = OpenLabel
	}
	row := fmt.Sprintf("%-10s %-14s %s → %s  %s", inv.Actor.Instance, typ,
		formatClock(inv.StartedAt), end, FormatDuration(e.Duration, e.HasDuration))
	if len(e.Overlaps) > 0 {
		ids := make([]string, len(e.Overlaps))
		for i, id := range e.Overlaps {
			ids[i] = string(id)
		}
		row += "  parallel with " + strings.Join(ids, ", ")
	}
	return row
}

// compactionRow renders one compaction: time, actor, trigger and token counts.
func compactionRow(c domain.Compaction) string {
	return fmt.Sprintf("%s  %s  %s  %s → %s", formatClock(c.At), c.Actor.Label(), c.Trigger,
		FormatTokens(c.TokensBefore), FormatTokens(c.TokensAfter))
}

// Help implements Screen.
func (s *TimelineScreen) Help() string {
	return "↑/↓ scroll  " + viewKeysHelp(ScreenTimeline) + "  esc back  ctrl+c quit"
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
